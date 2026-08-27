// Package transform handles image and video processing pipelines.
package transform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	"github.com/disintegration/imaging"
	"github.com/muesli/smartcrop"
	"github.com/muesli/smartcrop/nfnt"
)

const (
	maxPreviewSize = 800
	avatarSize     = 256
)

// ProcessAvatar decodes image data, fits it into a square canvas, and returns WebP-encoded bytes.
func ProcessAvatar(data []byte) ([]byte, error) {
	img, err := decodeOriented(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode avatar: %w", err)
	}

	fitted := imaging.Fit(img, avatarSize, avatarSize, imaging.Lanczos)
	canvas := imaging.New(avatarSize, avatarSize, color.NRGBA{0, 0, 0, 0})
	avatar := imaging.PasteCenter(canvas, fitted)

	var out bytes.Buffer
	if err = nativewebp.Encode(&out, avatar, &nativewebp.Options{
		CompressionLevel: nativewebp.BestCompression,
	}); err != nil {
		return nil, fmt.Errorf("encode avatar: %w", err)
	}
	return out.Bytes(), nil
}

// WatermarkParams defines parameters for image watermark transforms.
type WatermarkParams struct {
	WatermarkAssetID string  `json:"watermark_asset_id"`
	Opacity          float64 `json:"opacity"`
	Format           string  `json:"format"`  // jpeg | png | tiff | webp
	Quality          int     `json:"quality"` // 1–100 (JPEG only; ignored for WebP)
}

func (p *WatermarkParams) normalize() {
	if p.Opacity <= 0 || p.Opacity > 1 {
		p.Opacity = 0.5
	}
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}
	if p.Format == "" {
		p.Format = FormatJPEG
	}
}

// Normalize applies default values to watermark params.
func (p *WatermarkParams) Normalize() {
	p.normalize()
}

func renderWatermarkOverlay(wmReader io.Reader, bounds image.Rectangle, opacity float64) (*image.NRGBA, error) {
	wmImg, err := decodeOriented(wmReader)
	if err != nil {
		return nil, fmt.Errorf("decode watermark image: %w", err)
	}

	wmOpacity := applyWatermarkOpacity(wmImg, opacity)
	wmBounds := wmOpacity.Bounds()
	if wmBounds.Dx() == 0 || wmBounds.Dy() == 0 {
		return nil, errors.New("watermark image has invalid dimensions")
	}

	dst := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += wmBounds.Dy() {
		for x := bounds.Min.X; x < bounds.Max.X; x += wmBounds.Dx() {
			tileRect := image.Rect(x, y, x+wmBounds.Dx(), y+wmBounds.Dy())
			draw.Draw(dst, tileRect, wmOpacity, wmBounds.Min, draw.Over)
		}
	}
	return dst, nil
}

// ResizeParams defines parameters for image resize/fit transforms.
type ResizeParams struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Fit     string `json:"fit"`     // cover | contain | fill
	Quality int    `json:"quality"` // 1–100, default 85
	Format  string `json:"format"`  // jpeg | png | tiff | webp
}

// ConvertParams defines parameters for image format conversion.
type ConvertParams struct {
	Format  string `json:"format"`  // jpeg | png | tiff | webp
	Quality int    `json:"quality"` // 1–100 (JPEG only; ignored for WebP)
}

// CropParams defines parameters for an image crop operation.
type CropParams struct {
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Quality int    `json:"quality"` // 1–100 (JPEG only; ignored for WebP)
	Format  string `json:"format"`  // jpeg | png | tiff | webp
}

// SmartCropParams defines parameters for smart-crop transforms.
type SmartCropParams struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Quality int    `json:"quality"` // 1–100, default 85
	Format  string `json:"format"`  // jpeg | png | tiff | webp
}

// PreviewParams defines parameters for the low-res in-memory preview.
type PreviewParams struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Fit     string `json:"fit"`
	Quality int    `json:"quality"` // 1–100 (JPEG only; ignored for WebP)
	Format  string `json:"format"`  // jpeg | png | tiff | webp
}

// ApplyWatermark decodes, composites, and returns the final NRGBA image.
func ApplyWatermark(
	_ context.Context,
	srcReader io.Reader,
	wmReader io.Reader,
	params WatermarkParams,
) (*image.NRGBA, error) {
	params.normalize()

	srcImg, err := decodeOriented(srcReader)
	if err != nil {
		return nil, fmt.Errorf("decode source image: %w", err)
	}

	dst := imaging.Clone(srcImg)
	overlay, err := renderWatermarkOverlay(wmReader, dst.Bounds(), params.Opacity)
	if err != nil {
		return nil, err
	}
	draw.Draw(dst, dst.Bounds(), overlay, overlay.Bounds().Min, draw.Over)
	return dst, nil
}

// ImageWatermark reads an image, applies a watermark, and returns encoded bytes.
func (t *transformer) ImageWatermark(src io.Reader, wm io.Reader, p WatermarkParams) ([]byte, string, error) {
	if strings.TrimSpace(p.WatermarkAssetID) == "" {
		return nil, "", errors.New("watermark asset id is required")
	}

	result, err := ApplyWatermark(context.Background(), src, wm, p)
	if err != nil {
		return nil, "", err
	}

	return encodeImage(result, p.Format, p.Quality)
}

func applyWatermarkOpacity(img image.Image, opacity float64) *image.NRGBA {
	src := imaging.Clone(img)
	dst := image.NewNRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			c, ok := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if !ok {
				continue
			}
			c.A = uint8(math.Round(float64(c.A) * opacity))
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst
}

// ImageResize reads an image, resizes it according to params, and returns encoded bytes.
func (t *transformer) ImageResize(src io.Reader, p ResizeParams) ([]byte, string, error) {
	if p.Width <= 0 && p.Height <= 0 {
		return nil, "", errors.New("width or height must be > 0")
	}
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}

	img, err := decodeOriented(src)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	var result image.Image
	switch strings.ToLower(p.Fit) {
	case "fill":
		w, h := ensureDimensions(img, p.Width, p.Height)
		result = imaging.Fill(img, w, h, imaging.Center, imaging.Lanczos)
	case "cover":
		w, h := ensureDimensions(img, p.Width, p.Height)
		result = imaging.Fill(img, w, h, imaging.Center, imaging.Lanczos)
	default: // contain
		result = imaging.Fit(img, p.Width, p.Height, imaging.Lanczos)
	}

	return encodeImage(result, p.Format, p.Quality)
}

// ImageConvert reads an image and re-encodes it in the target format.
func (t *transformer) ImageConvert(src io.Reader, p ConvertParams) ([]byte, string, error) {
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}
	img, err := decodeOriented(src)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	return encodeImage(img, p.Format, p.Quality)
}

// ImageCrop reads an image, crops the specified rectangle, and returns encoded bytes.
func (t *transformer) ImageCrop(src io.Reader, p CropParams) ([]byte, string, error) {
	if p.Width <= 0 || p.Height <= 0 {
		return nil, "", errors.New("width and height must be > 0")
	}
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}
	img, err := decodeOriented(src)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	cropped := imaging.Crop(img, image.Rect(p.X, p.Y, p.X+p.Width, p.Y+p.Height))
	return encodeImage(cropped, p.Format, p.Quality)
}

// ImageSmartCrop finds the most visually interesting region of src at the given size.
func (t *transformer) ImageSmartCrop(src io.Reader, p SmartCropParams) ([]byte, string, error) {
	if p.Width <= 0 || p.Height <= 0 {
		return nil, "", errors.New("width and height must be > 0")
	}
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 85
	}

	img, err := decodeOriented(src)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	analyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
	topCrop, err := analyzer.FindBestCrop(img, p.Width, p.Height)
	if err != nil {
		return nil, "", fmt.Errorf("find best crop: %w", err)
	}

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	sub, ok := img.(subImager)
	if !ok {
		return nil, "", errors.New("decoded image does not support subimage")
	}
	cropped := sub.SubImage(topCrop)
	result := imaging.Resize(cropped, p.Width, p.Height, imaging.Lanczos)

	return encodeImage(result, p.Format, p.Quality)
}

// ImagePreview generates a small in-memory preview (max 800px) for the UI.
func (t *transformer) ImagePreview(src io.Reader, p PreviewParams) ([]byte, string, error) {
	if p.Quality <= 0 || p.Quality > 100 {
		p.Quality = 80
	}
	// Cap preview at 800px.
	if p.Width > maxPreviewSize {
		p.Width = maxPreviewSize
	}
	if p.Height > maxPreviewSize {
		p.Height = maxPreviewSize
	}
	if p.Width <= 0 && p.Height <= 0 {
		p.Width = maxPreviewSize
	}

	img, err := decodeOriented(src)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	var result image.Image
	switch strings.ToLower(p.Fit) {
	case "fill", "cover":
		w, h := ensureDimensions(img, p.Width, p.Height)
		result = imaging.Fill(img, w, h, imaging.Center, imaging.Lanczos)
	default:
		w, h := ensureDimensions(img, p.Width, p.Height)
		result = imaging.Fit(img, w, h, imaging.Lanczos)
	}

	if p.Format == "" {
		p.Format = FormatJPEG
	}
	return encodeImage(result, p.Format, p.Quality)
}

// encodeImage encodes an image to the given format and returns the bytes + content-type.
func encodeImage(img image.Image, format string, quality int) ([]byte, string, error) {
	var buf bytes.Buffer
	var contentType string
	switch strings.ToLower(format) {
	case FormatPNG:
		if err := imaging.Encode(&buf, img, imaging.PNG); err != nil {
			return nil, "", fmt.Errorf("encode png: %w", err)
		}
		contentType = MimeImagePNG
	case "tiff":
		if err := imaging.Encode(&buf, img, imaging.TIFF); err != nil {
			return nil, "", fmt.Errorf("encode tiff: %w", err)
		}
		contentType = "image/tiff"
	case FormatWebP:
		if err := nativewebp.Encode(&buf, img, &nativewebp.Options{
			CompressionLevel: nativewebp.BestCompression,
		}); err != nil {
			return nil, "", fmt.Errorf("encode webp: %w", err)
		}
		contentType = MimeImageWebP
	default: // jpeg
		if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
		contentType = MimeImageJPEG
	}
	return buf.Bytes(), contentType, nil
}

// ensureDimensions returns both dimensions; if one is 0, it uses the other.
func ensureDimensions(img image.Image, w, h int) (int, int) {
	if w <= 0 {
		w = img.Bounds().Dx()
	}
	if h <= 0 {
		h = img.Bounds().Dy()
	}
	return w, h
}
