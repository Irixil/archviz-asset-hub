// Package gdrive implements an ingress Source backed by Google Drive.
package gdrive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"damask/server/internal/ingress"
	"damask/server/internal/oauth"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const maxGDrivePollSize = 100

// Config is the decrypted JSON config stored in ingress_sources.config.
type Config struct {
	ConnectionID      string `json:"connection_id"`
	WorkspaceID       string `json:"workspace_id"`
	FolderID          string `json:"folder_id"`
	FolderName        string `json:"folder_name"`
	IncludeSubfolders bool   `json:"include_subfolders"`
	ExportGoogleDocs  bool   `json:"export_google_docs"`
	AfterImport       string `json:"after_import"` // "leave" | "move_to_trash"
}

// globalRefresher is set at server startup via SetRefresher.
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
	ingress.Register("gdrive", New)
}

// New builds a GDriveSource from decrypted config JSON.
func New(configJSON []byte) (ingress.Source, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("gdrive: parse config: %w", err)
	}
	if cfg.WorkspaceID == "" {
		return nil, errors.New("gdrive: workspace_id is required")
	}
	if cfg.ConnectionID == "" {
		return nil, errors.New("gdrive: connection_id is required")
	}
	if cfg.FolderID == "" {
		return nil, errors.New("gdrive: folder_id is required")
	}
	return &Source{cfg: cfg}, nil
}

// Source polls a Google Drive folder for new files.
type Source struct {
	cfg Config
}

func (s *Source) Type() string { return "gdrive" }

func (s *Source) driveService(ctx context.Context) (*drive.Service, error) {
	refresherMu.RLock()
	r := globalRefresher
	refresherMu.RUnlock()
	if r == nil {
		return nil, errors.New("gdrive: token refresher not initialised")
	}
	token, err := r.EnsureFreshToken(ctx, s.cfg.WorkspaceID, s.cfg.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("gdrive: refresh token: %w", err)
	}
	svc, err := drive.NewService(ctx, option.WithTokenSource(
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
	))
	if err != nil {
		return nil, fmt.Errorf("gdrive: new drive service: %w", err)
	}
	return svc, nil
}

func (s *Source) Validate(ctx context.Context) error {
	svc, err := s.driveService(ctx)
	if err != nil {
		return err
	}
	_, err = svc.Files.Get(s.cfg.FolderID).Fields("id,name").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("gdrive: validate folder %s: %w", s.cfg.FolderID, err)
	}
	return nil
}

func (s *Source) Poll(ctx context.Context) ([]ingress.IngestItem, error) {
	svc, err := s.driveService(ctx)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("'%s' in parents and trashed = false", s.cfg.FolderID)
	var items []ingress.IngestItem
	pageToken := ""
	for {
		call := svc.Files.List().
			Q(query).
			Fields("nextPageToken, files(id,name,mimeType,size,modifiedTime,md5Checksum)").
			PageSize(maxGDrivePollSize).
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		var result *drive.FileList
		result, err = call.Do()
		if err != nil {
			return nil, fmt.Errorf("gdrive: list files: %w", err)
		}
		for _, f := range result.Files {
			// Skip Google Workspace files if export is disabled.
			if isGoogleWorkspaceType(f.MimeType) && !s.cfg.ExportGoogleDocs {
				continue
			}
			remoteID := f.Id + "#" + f.ModifiedTime
			if f.Md5Checksum != "" {
				remoteID = f.Id + "#" + f.Md5Checksum
			}
			item := ingress.IngestItem{
				RemoteID: remoteID,
				Filename: exportFilename(f.Name, f.MimeType),
				Size:     f.Size,
				Meta: map[string]string{
					"file_id":   f.Id,
					"mime_type": f.MimeType,
				},
			}
			items = append(items, item)
		}
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	return items, nil
}

func (s *Source) Fetch(ctx context.Context, item ingress.IngestItem) (io.ReadCloser, error) {
	svc, err := s.driveService(ctx)
	if err != nil {
		return nil, err
	}

	fileID := item.Meta["file_id"]
	mimeType := item.Meta["mime_type"]

	if isGoogleWorkspaceType(mimeType) {
		exportMIME := exportMIMEType(mimeType)
		exportResp, exportErr := svc.Files.Export(fileID, exportMIME).Context(ctx).Download()
		if exportErr != nil {
			return nil, fmt.Errorf("gdrive: export %s as %s: %w", fileID, exportMIME, exportErr)
		}
		return exportResp.Body, nil
	}

	resp, err := svc.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		slog.ErrorContext(ctx, "gdrive: cannot download item", "item", item, "error", err)
		return nil, fmt.Errorf("gdrive: download [%s]: %w", fileID, err)
	}
	return resp.Body, nil
}

const googleWorkspacePDFMIME = "application/pdf"

var googleWorkspaceMIMEs = map[string]string{
	"application/vnd.google-apps.document":     googleWorkspacePDFMIME,
	"application/vnd.google-apps.spreadsheet":  googleWorkspacePDFMIME,
	"application/vnd.google-apps.presentation": googleWorkspacePDFMIME,
	"application/vnd.google-apps.drawing":      "image/png",
}

func isGoogleWorkspaceType(mimeType string) bool {
	_, ok := googleWorkspaceMIMEs[mimeType]
	return ok
}

func exportMIMEType(mimeType string) string {
	if m, ok := googleWorkspaceMIMEs[mimeType]; ok {
		return m
	}
	return googleWorkspacePDFMIME
}

func exportFilename(name, mimeType string) string {
	switch mimeType {
	case "application/vnd.google-apps.document",
		"application/vnd.google-apps.spreadsheet",
		"application/vnd.google-apps.presentation":
		return name + ".pdf"
	case "application/vnd.google-apps.drawing":
		return name + ".png"
	}
	return name
}
