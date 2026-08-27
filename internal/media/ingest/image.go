package ingest

import (
	"context"
	"image"
	_ "image/gif"  //nolint:nolintlint // register GIF format
	_ "image/jpeg" //nolint:nolintlint // register JPEG format
	_ "image/png"  //nolint:nolintlint // register PNG format
	"os"
	"strings"
)

type ImageHandler struct{}

func (h ImageHandler) Supports(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

func (h ImageHandler) ExtractMeta(_ context.Context, filePath string) (FileMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return FileMeta{}, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)

	var width, height *int64
	if err == nil {
		w, ht := int64(cfg.Width), int64(cfg.Height)
		width, height = &w, &ht
	}

	return FileMeta{
		Width:  width,
		Height: height,
	}, nil
}
