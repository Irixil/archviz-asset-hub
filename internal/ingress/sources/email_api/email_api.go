// Package email_api implements a push-based IngestorSource backed by the
// built-in mail server. Poll() is a no-op; items arrive via SMTP and are
// inserted directly into ingress_log by the mail server hook.
package email_api //nolint:staticcheck // name is a db token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"

	"damask/server/internal/ingress"
)

const emailTokenLength = 32

func init() {
	ingress.Register("email_api", New)
	ingress.RegisterOnCreate("email_api", onCreateConfig)
}

func onCreateConfig(config map[string]any) (map[string]any, error) {
	token, err := ingress.GenerateToken(emailTokenLength)
	if err != nil {
		return nil, fmt.Errorf("email_api: generate ingest_token: %w", err)
	}
	out := make(map[string]any, len(config)+1)
	maps.Copy(out, config)
	out["ingest_token"] = token // always overwrite — must never be user-controlled
	return out, nil
}

// Config is the JSON configuration for an email_api source.
// The ingest_token is generated server-side and is read-only for the user.
type Config struct {
	IngestToken string `json:"ingest_token"`
}

// EmailAPISource is a push-based source — the mail server hook delivers items.
type EmailAPISource struct {
	cfg Config
}

// New builds an EmailAPISource from decrypted config JSON.
func New(configJSON []byte) (ingress.Source, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("email_api: parse config: %w", err)
	}
	return &EmailAPISource{cfg: cfg}, nil
}

func (s *EmailAPISource) Type() string { return "email_api" }

// Validate checks that the ingest token is set.
func (s *EmailAPISource) Validate(_ context.Context) error {
	if s.cfg.IngestToken == "" {
		return errors.New("email_api: ingest_token is empty")
	}
	return nil
}

// Poll is a no-op for push-based sources.
// Items arrive via the SMTP hook in the mail server.
func (s *EmailAPISource) Poll(_ context.Context) ([]ingress.IngestItem, error) {
	return nil, nil
}

// Fetch is not used for push-based sources (attachments are written to temp
// files by the SMTP hook before the fetch job is enqueued).
func (s *EmailAPISource) Fetch(_ context.Context, item ingress.IngestItem) (io.ReadCloser, error) {
	if path, ok := item.Meta["tmp_path"]; ok {
		// The SMTP hook stored the attachment at this temp path
		return nil, fmt.Errorf("email_api: fetch not supported; use tmp_path=%s from meta", path)
	}
	return nil, errors.New("email_api: fetch not supported for push source")
}
