package ingest

import (
	"context"
	"log/slog"

	"damask/server/internal/transform"
)

// FileMeta holds the media metadata extracted from a file.
type FileMeta struct {
	MimeType    string
	Size        int64
	Width       *int64
	Height      *int64
	DurationSec *float64
}

// MediaHandler extracts metadata from a specific media type.
type MediaHandler interface {
	Supports(mime string) bool
	ExtractMeta(ctx context.Context, filePath string) (FileMeta, error)
}

// Registry holds an ordered list of media handlers and dispatches metadata
// extraction to the first handler that claims support for a given MIME type.
type Registry struct {
	handlers []MediaHandler
}

// NewRegistry builds a Registry wired to the given Transformer.
// External dependency availability is checked; missing deps are logged.
func NewRegistry(trf transform.Transformer) *Registry {
	if missing := trf.CheckExternalDeps(); len(missing) > 0 {
		slog.WarnContext(
			context.Background(),
			"external dependencies missing — some thumbnail types will be skipped",
			"missing",
			missing,
		)
	}
	return &Registry{
		handlers: []MediaHandler{
			ImageHandler{},
			NewVideoHandler(trf),
			NewPDFHandler(trf),
			NewDefaultHandler([]string{
				"application/x-subrip",
				"application/msword",
				"application/vnd",
				"text/plain",
				"text/html",
				"text/csv",
				"audio/",
				"font/",
			}),
		},
	}
}

// ExtractMeta returns metadata for the file at filePath using the handler
// registered for mimeType. Returns a zero FileMeta if no handler matches.
func (r *Registry) ExtractMeta(ctx context.Context, filePath, mimeType string) (FileMeta, error) {
	for _, h := range r.handlers {
		if h.Supports(mimeType) {
			return h.ExtractMeta(ctx, filePath)
		}
	}
	return FileMeta{}, nil
}

// Supports reports whether any registered handler claims the given MIME type.
func (r *Registry) Supports(mimeType string) bool {
	for _, h := range r.handlers {
		if h.Supports(mimeType) {
			return true
		}
	}
	return false
}
