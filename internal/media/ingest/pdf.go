package ingest

import (
	"context"
	"log/slog"

	"damask/server/internal/transform"
)

type PDFHandler struct {
	trf transform.Transformer
}

func NewPDFHandler(trf transform.Transformer) PDFHandler {
	return PDFHandler{trf: trf}
}

func (h PDFHandler) Supports(mime string) bool {
	return mime == "application/pdf"
}

func (h PDFHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	res, err := h.trf.PDFExtractResolution(ctx, filePath)

	var width, height *int64
	if err == nil {
		width = &res.Width
		height = &res.Height
	} else {
		slog.WarnContext(ctx, "pdf meta extraction failed", "error", err)
	}

	return FileMeta{
		Width:  width,
		Height: height,
	}, nil
}
