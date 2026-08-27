// Package canva implements an ingress Source backed by the Canva API.
package canva

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"damask/server/internal/ingress"
	"damask/server/internal/oauth"
)

const (
	canvaAPIBase  = "https://api.canva.com/rest/v1"
	exportPollMax = 45 // max poll attempts at 2s each = 90 seconds
	fetchThrottle = 2 * time.Second
)

// Config is the decrypted JSON config stored in ingress_sources.config.
type Config struct {
	ConnectionID string `json:"connection_id"`
	WorkspaceID  string `json:"workspace_id"`
	ExportFormat string `json:"export_format"` // "pdf" | "png" | "jpg"
	Ownership    string `json:"ownership"`     // "owned" | "shared" | "any"
	NameFilter   string `json:"name_filter"`
}

var (
	refresherMu     sync.RWMutex
	globalRefresher *oauth.TokenRefresher
)

// SetRefresher injects the TokenRefresher. Call once at server startup.
func SetRefresher(r *oauth.TokenRefresher) {
	refresherMu.Lock()
	defer refresherMu.Unlock()
	globalRefresher = r
}

func init() {
	ingress.Register("canva", New)
}

// New builds a CanvaSource from decrypted config JSON.
func New(configJSON []byte) (ingress.Source, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("canva: parse config: %w", err)
	}
	if cfg.ConnectionID == "" {
		return nil, errors.New("canva: connection_id is required")
	}
	if cfg.ExportFormat == "" {
		cfg.ExportFormat = "pdf"
	}
	if cfg.Ownership == "" {
		cfg.Ownership = "owned"
	}
	return &Source{cfg: cfg}, nil
}

// Source polls Canva for new or updated designs and exports them.
type Source struct {
	cfg Config
}

func (s *Source) Type() string { return "canva" }

func (s *Source) accessToken(ctx context.Context) (string, error) {
	refresherMu.RLock()
	r := globalRefresher
	refresherMu.RUnlock()
	if r == nil {
		return "", errors.New("canva: token refresher not initialised")
	}
	return r.EnsureFreshToken(ctx, s.cfg.WorkspaceID, s.cfg.ConnectionID)
}

func (s *Source) Validate(ctx context.Context) error {
	token, err := s.accessToken(ctx)
	if err != nil {
		return err
	}
	// Verify token validity with a lightweight call.
	_, err = s.doGet(ctx, token, canvaAPIBase+"/users/me")
	return err
}

func (s *Source) Poll(ctx context.Context) ([]ingress.IngestItem, error) {
	token, err := s.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	ownership := s.cfg.Ownership
	if ownership == "any" {
		ownership = ""
	}

	var items []ingress.IngestItem
	pageToken := ""
	for {
		url := canvaAPIBase + "/designs?sort_by=modified_descending"
		if ownership != "" {
			url += "&ownership=" + ownership
		}
		if pageToken != "" {
			url += "&continuation=" + pageToken
		}

		body, fetchErr := s.doGet(ctx, token, url)
		if fetchErr != nil {
			return nil, fmt.Errorf("canva: list designs: %w", fetchErr)
		}

		var page struct {
			Items []struct {
				ID        string `json:"id"`
				Title     string `json:"title"`
				UpdatedAt int64  `json:"updated_at"`
				Thumbnail struct {
					URL string `json:"url"`
				} `json:"thumbnail"`
			} `json:"items"`
			Continuation string `json:"continuation"`
		}
		if err = json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("canva: parse designs response: %w", err)
		}

		for _, d := range page.Items {
			if s.cfg.NameFilter != "" {
				if !strings.Contains(strings.ToLower(d.Title), strings.ToLower(s.cfg.NameFilter)) {
					continue
				}
			}
			remoteID := d.ID + "#" + strconv.FormatInt(d.UpdatedAt, 10)
			items = append(items, ingress.IngestItem{
				RemoteID: remoteID,
				Filename: d.Title + "." + s.cfg.ExportFormat,
				ModTime:  time.Unix(d.UpdatedAt, 0),
				Meta: map[string]string{
					"design_id":     d.ID,
					"title":         d.Title,
					"thumbnail_url": d.Thumbnail.URL,
				},
			})
		}

		if page.Continuation == "" {
			break
		}
		pageToken = page.Continuation
	}
	return items, nil
}

func (s *Source) Fetch(ctx context.Context, item ingress.IngestItem) (io.ReadCloser, error) {
	// Separate timeout for export creation + polling only — does not apply to the download stream.
	pollCtx, pollCancel := context.WithTimeout(ctx, 90*time.Second)
	defer pollCancel()

	token, err := s.accessToken(pollCtx)
	if err != nil {
		return nil, err
	}

	designID := item.Meta["design_id"]

	// Step 1: create export job.
	exportBody, _ := json.Marshal(map[string]any{
		"design_id": designID,
		"format":    map[string]string{"type": s.cfg.ExportFormat},
	})
	resp, err := s.doPost(pollCtx, token, canvaAPIBase+"/exports", exportBody)
	if err != nil {
		return nil, fmt.Errorf("canva: create export: %w", err)
	}

	var exportResp struct {
		Job struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"job"`
	}
	if err = json.Unmarshal(resp, &exportResp); err != nil {
		return nil, fmt.Errorf("canva: parse export response: %w", err)
	}
	jobID := exportResp.Job.ID

	// Step 2: poll until done.
	var downloadURL string
	for range exportPollMax {
		var body []byte
		body, err = s.doGet(pollCtx, token, canvaAPIBase+"/exports/"+jobID)
		if err != nil {
			return nil, fmt.Errorf("canva: poll export: %w", err)
		}

		var status struct {
			Job struct {
				Status string   `json:"status"`
				Urls   []string `json:"urls"`
			} `json:"job"`
		}
		if err = json.Unmarshal(body, &status); err != nil {
			return nil, fmt.Errorf("canva: parse export status: %w", err)
		}

		switch status.Job.Status {
		case "success":
			if len(status.Job.Urls) == 0 {
				return nil, errors.New("canva: export succeeded but no download URLs")
			}
			downloadURL = status.Job.Urls[0]
		case "failed":
			return nil, fmt.Errorf("canva: export job failed for design %s", designID)
		}
		if downloadURL != "" {
			break
		}
		time.Sleep(fetchThrottle)
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("canva: export timed out for design %s", designID)
	}

	// Step 3: download with a fresh context so the response body isn't cancelled when
	// pollCtx's defer fires upon returning from this function.
	dlCtx, dlCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	req, _ := http.NewRequestWithContext(dlCtx, http.MethodGet, downloadURL, nil)
	dlResp, err := http.DefaultClient.Do(req) //nolint:bodyclose // caller closes the returned ReadCloser
	if err != nil {
		dlCancel()
		return nil, fmt.Errorf("canva: download export: %w", err)
	}
	return &cancelOnClose{ReadCloser: dlResp.Body, cancel: dlCancel}, nil
}

// cancelOnClose wraps an [io.ReadCloser] and calls a cancel func on Close,
// ensuring the download context is released when the caller is done streaming.
type cancelOnClose struct {
	io.ReadCloser

	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	c.cancel()
	return c.ReadCloser.Close()
}

func (s *Source) doGet(ctx context.Context, token, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// slog.InfoContext(ctx, "get canva", "url", url, "token", token)
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("canva API %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *Source) doPost(ctx context.Context, token, url string, body []byte) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// slog.InfoContext(ctx, "post canva", "url", url, "token", token, "body", string(body))
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("canva API %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
