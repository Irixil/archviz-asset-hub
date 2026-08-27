package ingest

import (
	"context"
	"log/slog"
	"strings"

	"damask/server/internal/transform"
)

type VideoHandler struct {
	trf transform.Transformer
}

func NewVideoHandler(trf transform.Transformer) VideoHandler {
	return VideoHandler{trf: trf}
}

func (h VideoHandler) Supports(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

func (h VideoHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	res, err := h.trf.VideoExtractResolution(ctx, filePath)

	var width, height *int64
	if err == nil {
		width = &res.Width
		height = &res.Height
	} else {
		slog.WarnContext(ctx, "video meta extraction failed", "error", err)
	}

	return FileMeta{
		Width:  width,
		Height: height,
	}, nil
}
