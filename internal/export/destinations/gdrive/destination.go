// Package gdrive implements an ExportDestination backed by Google Drive.
package gdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"damask/server/internal/export"
	"damask/server/internal/oauth"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const resumableUploadThreshold = 5 * 1024 * 1024 // 5 MB

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
	export.Register("gdrive", func(cfg []byte) (export.Destination, error) {
		return New(cfg)
	})
}

// Config is the decrypted JSON config for a GDrive export destination.
type Config struct {
	ConnectionID string `json:"connection_id"`
	WorkspaceID  string `json:"workspace_id"`
	FolderID     string `json:"folder_id"`
	FolderName   string `json:"folder_name"`
}

// Destination writes exports to a Google Drive folder.
type Destination struct {
	cfg Config
}

// New builds a GDrive Destination from decrypted config JSON.
func New(configJSON []byte) (*Destination, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("gdrive dest: parse config: %w", err)
	}
	if cfg.ConnectionID == "" {
		return nil, errors.New("gdrive dest: connection_id is required")
	}
	if cfg.FolderID == "" {
		return nil, errors.New("gdrive dest: folder_id is required")
	}
	return &Destination{cfg: cfg}, nil
}

func (d *Destination) Type() string { return "gdrive" }

func (d *Destination) Write(ctx context.Context, remotePath string, r io.Reader, size int64, _ string) error {
	svc, err := d.driveService(ctx)
	if err != nil {
		return err
	}
	name := path.Base(remotePath)
	f := &drive.File{
		Name:    name,
		Parents: []string{d.cfg.FolderID},
	}
	call := svc.Files.Create(f).SupportsAllDrives(true)
	if size > resumableUploadThreshold {
		call = call.SupportsTeamDrives(true)
	}
	if _, e := call.Media(r).Do(); e != nil {
		return fmt.Errorf("gdrive dest: upload %s: %w", name, e)
	}
	return nil
}

func (d *Destination) ReadManifest(ctx context.Context, remotePath string) ([]byte, error) {
	svc, err := d.driveService(ctx)
	if err != nil {
		return nil, err
	}
	name := path.Base(remotePath)
	fileID, err := d.findFile(svc, name)
	if err != nil {
		return nil, err
	}
	if fileID == "" {
		return nil, nil
	}
	resp, err := svc.Files.Get(fileID).Download()
	if err != nil {
		return nil, fmt.Errorf("gdrive dest: download manifest: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (d *Destination) WriteManifest(ctx context.Context, remotePath string, data []byte) error {
	svc, err := d.driveService(ctx)
	if err != nil {
		return err
	}
	name := path.Base(remotePath)
	fileID, err := d.findFile(svc, name)
	if err != nil {
		return err
	}
	if fileID != "" {
		_, err = svc.Files.Update(fileID, &drive.File{}).Media(bytes.NewReader(data)).Do()
		if err != nil {
			return fmt.Errorf("gdrive dest: update manifest: %w", err)
		}
		return nil
	}
	f := &drive.File{
		Name:    name,
		Parents: []string{d.cfg.FolderID},
	}
	if _, e := svc.Files.Create(f).Media(bytes.NewReader(data)).Do(); e != nil {
		return fmt.Errorf("gdrive dest: create manifest: %w", e)
	}
	return nil
}

func (d *Destination) Validate(ctx context.Context) error {
	svc, err := d.driveService(ctx)
	if err != nil {
		return err
	}
	f, err := svc.Files.Get(d.cfg.FolderID).
		Fields("id,name,capabilities").
		SupportsAllDrives(true).
		Do()
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && gErr.Code == 404 {
			return fmt.Errorf("gdrive dest: folder %s not found", d.cfg.FolderID)
		}
		return fmt.Errorf("gdrive dest: get folder: %w", err)
	}
	if f.Capabilities != nil && !f.Capabilities.CanAddChildren {
		return fmt.Errorf("gdrive dest: no write permission on folder %s", d.cfg.FolderID)
	}
	return nil
}

func (d *Destination) driveService(ctx context.Context) (*drive.Service, error) {
	refresherMu.RLock()
	r := globalRefresher
	refresherMu.RUnlock()
	if r == nil {
		return nil, errors.New("gdrive dest: token refresher not initialised")
	}
	token, err := r.EnsureFreshToken(ctx, d.cfg.WorkspaceID, d.cfg.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("gdrive dest: refresh token: %w", err)
	}
	svc, err := drive.NewService(ctx, option.WithTokenSource(
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
	))
	if err != nil {
		return nil, fmt.Errorf("gdrive dest: new drive service: %w", err)
	}
	return svc, nil
}

func (d *Destination) findFile(svc *drive.Service, name string) (string, error) {
	escaped := strings.ReplaceAll(name, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	q := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", escaped, d.cfg.FolderID)
	list, err := svc.Files.List().Q(q).Fields("files(id)").SupportsAllDrives(true).IncludeItemsFromAllDrives(true).Do()
	if err != nil {
		return "", fmt.Errorf("gdrive dest: list files: %w", err)
	}
	if len(list.Files) == 0 {
		return "", nil
	}
	return list.Files[0].Id, nil
}
