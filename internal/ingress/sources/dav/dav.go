// Package dav implements an IngestorSource backed by a WebDAV server
// (Nextcloud, ownCloud, plain DAV).
package dav

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"damask/server/internal/ingress"

	"github.com/studio-b12/gowebdav"
)

func init() {
	ingress.Register("dav", New)
}

// Config is the decrypted JSON configuration for a WebDAV source.
type Config struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Path     string `json:"path"`
}

// Source watches a WebDAV collection for new files.
type Source struct {
	cfg Config
}

// New builds a DAVSource from decrypted config JSON.
func New(configJSON []byte) (ingress.Source, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("dav: parse config: %w", err)
	}
	if cfg.Path == "" {
		cfg.Path = "/"
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Type() string { return "dav" }

func (s *Source) Validate(_ context.Context) error {
	c := s.client()
	if err := c.Connect(); err != nil {
		return fmt.Errorf("dav: connect %s: %w", s.cfg.URL, err)
	}
	if _, err := c.Stat(s.cfg.Path); err != nil {
		return fmt.Errorf("dav: stat %s: %w", s.cfg.Path, err)
	}
	return nil
}

func (s *Source) Poll(_ context.Context) ([]ingress.IngestItem, error) {
	c := s.client()

	entries, err := c.ReadDir(s.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("dav: readdir %s: %w", s.cfg.Path, err)
	}

	var items []ingress.IngestItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := s.cfg.Path + "/" + entry.Name()
		items = append(items, ingress.IngestItem{
			RemoteID: path,
			Filename: entry.Name(),
			ModTime:  entry.ModTime(),
			Size:     entry.Size(),
		})
	}
	return items, nil
}

func (s *Source) Fetch(_ context.Context, item ingress.IngestItem) (io.ReadCloser, error) {
	c := s.client()
	rc, err := c.ReadStream(item.RemoteID)
	if err != nil {
		return nil, fmt.Errorf("dav: read stream %s: %w", item.RemoteID, err)
	}
	return rc, nil
}

func (s *Source) client() *gowebdav.Client {
	return gowebdav.NewClient(s.cfg.URL, s.cfg.Username, s.cfg.Password)
}
