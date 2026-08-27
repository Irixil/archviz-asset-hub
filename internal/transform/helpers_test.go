package transform_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"

	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/disintegration/imaging"
)

// mockStorage implements storage.Storage for testing.
type mockStorage struct {
	data map[string][]byte
	err  error
}

func (m *mockStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	if data, exists := m.data[key]; exists {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, errors.New("not found")
}

func (m *mockStorage) Put(_ context.Context, _ string, _ io.Reader) error {
	return errors.New("not implemented")
}

func (m *mockStorage) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockStorage) List(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (m *mockStorage) Stat(_ context.Context, key string) (storage.Info, error) {
	if m.err != nil {
		return storage.Info{}, m.err
	}
	if data, exists := m.data[key]; exists {
		return storage.Info{Size: int64(len(data))}, nil
	}
	return storage.Info{}, fmt.Errorf("stat %s: %w", key, storage.ErrNotFound)
}

type mockTransformer struct{}

func (t *mockTransformer) FFmpegAvailable() bool       { return true }
func (t *mockTransformer) FFprobePath() string         { return "ffprobe" }
func (t *mockTransformer) ImageMagickAvailable() bool  { return true }
func (t *mockTransformer) LibreOfficeAvailable() bool  { return true }
func (t *mockTransformer) CheckExternalDeps() []string { return []string{} }
func (t *mockTransformer) pixel(format imaging.Format) []byte {
	var buf bytes.Buffer
	img := imaging.New(1, 1, color.Black)
	_ = imaging.Encode(&buf, img, format)
	return buf.Bytes()
}
func (t *mockTransformer) GenerateImageOfText(_ context.Context, _ transform.ImageOfTextOptions) ([]byte, error) {
	return t.pixel(imaging.PNG), nil
}

func (t *mockTransformer) PDFSlideshowThumbnail(
	_ context.Context,
	_ io.Reader,
	_ string,
) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) DocumentThumbnail(_ context.Context, _ io.Reader, _ string) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}

func (t *mockTransformer) ImageWatermark(
	_ io.Reader,
	_ io.Reader,
	_ transform.WatermarkParams,
) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) ImageResize(_ io.Reader, _ transform.ResizeParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) ImageConvert(_ io.Reader, _ transform.ConvertParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) ImageCrop(_ io.Reader, _ transform.CropParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) ImageSmartCrop(_ io.Reader, _ transform.SmartCropParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) ImagePreview(_ io.Reader, _ transform.PreviewParams) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) RemoveBackground(_ context.Context, _ []byte, _ string) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}
func (t *mockTransformer) AudioWaveform(_ context.Context, _ io.Reader, _ string) ([]byte, string, error) {
	return t.pixel(imaging.JPEG), transform.MimeImageJPEG, nil
}
func (t *mockTransformer) ExtractAudio(_ context.Context, _, _ string, _ transform.AudioParams) error {
	return nil
}
func (t *mockTransformer) TranscodeAudio(_ context.Context, _, _ string, _ transform.AudioParams) error {
	return nil
}
func (t *mockTransformer) NormalizeAudio(_ context.Context, _, _ string, _ transform.AudioParams) error {
	return nil
}

func (t *mockTransformer) PDFExtractResolution(
	_ context.Context,
	_ string,
) (*transform.VideoResolution, error) {
	return &transform.VideoResolution{Width: 1, Height: 1}, nil
}

func (t *mockTransformer) VideoExtractResolution(
	_ context.Context,
	_ string,
) (*transform.VideoResolution, error) {
	return &transform.VideoResolution{Width: 1, Height: 1}, nil
}

func (t *mockTransformer) VideoExtractThumbnail(
	_ context.Context,
	_ string,
	_ transform.VideoThumbnailParams,
) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}

func (t *mockTransformer) VideoClipThumbnail(
	_ context.Context,
	_ string,
	_ transform.VideoClipParams,
) ([]byte, error) {
	return t.pixel(imaging.JPEG), nil
}

func (t *mockTransformer) VideoTranscode(
	_ context.Context,
	_, _ string,
	_ transform.TranscodeParams,
) error {
	return nil
}

func (t *mockTransformer) VideoWatermark(
	_ context.Context,
	_, _ string,
	_ io.Reader,
	_ transform.VideoWatermarkParams,
) error {
	return nil
}

func (t *mockTransformer) RunCustomFFmpeg(_ context.Context, _, _, _ string, _ map[string]string) (string, error) {
	return "", nil
}

func (t *mockTransformer) DetectOutputMIME(_ context.Context, _ string) string {
	return transform.MimeApplicationOctetStream
}

func (t *mockTransformer) TrimToSeconds(_ context.Context, srcPath string, _ int, _ string) (string, error) {
	return srcPath, nil
}
