package transform

import (
	"context"
	"io"
	"os/exec"

	"damask/server/internal/config"
)

type Transformer interface {
	FFmpegAvailable() bool
	FFprobePath() string
	ImageMagickAvailable() bool
	LibreOfficeAvailable() bool
	CheckExternalDeps() []string

	GenerateImageOfText(ctx context.Context, opts ImageOfTextOptions) ([]byte, error)
	PDFSlideshowThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error)
	DocumentThumbnail(ctx context.Context, src io.Reader, mimeType string) ([]byte, error)

	ImageWatermark(src io.Reader, wm io.Reader, p WatermarkParams) ([]byte, string, error)
	ImageResize(src io.Reader, p ResizeParams) ([]byte, string, error)
	ImageConvert(src io.Reader, p ConvertParams) ([]byte, string, error)
	ImageCrop(src io.Reader, p CropParams) ([]byte, string, error)
	ImageSmartCrop(src io.Reader, p SmartCropParams) ([]byte, string, error)
	ImagePreview(src io.Reader, p PreviewParams) ([]byte, string, error)
	RemoveBackground(ctx context.Context, imageData []byte, apiKey string) ([]byte, error)

	AudioWaveform(ctx context.Context, src io.Reader, mimeType string) ([]byte, string, error)
	ExtractAudio(ctx context.Context, srcPath, dstPath string, p AudioParams) error
	TranscodeAudio(ctx context.Context, srcPath, dstPath string, p AudioParams) error
	NormalizeAudio(ctx context.Context, srcPath, dstPath string, p AudioParams) error

	PDFExtractResolution(ctx context.Context, srcPath string) (*VideoResolution, error)

	VideoExtractResolution(ctx context.Context, srcPath string) (*VideoResolution, error)
	VideoExtractThumbnail(ctx context.Context, srcPath string, p VideoThumbnailParams) ([]byte, error)
	VideoClipThumbnail(ctx context.Context, srcPath string, p VideoClipParams) ([]byte, error)
	VideoTranscode(ctx context.Context, srcPath, dstPath string, p TranscodeParams) error
	VideoWatermark(ctx context.Context, srcPath, dstPath string, wm io.Reader, p VideoWatermarkParams) error

	RunCustomFFmpeg(ctx context.Context, cmd, srcPath, outDir string, refs map[string]string) (string, error)
	DetectOutputMIME(ctx context.Context, outputPath string) string
	TrimToSeconds(ctx context.Context, srcPath string, n int, outDir string) (string, error)
}

func NewTransformer(cfg ...config.FFmpegConfig) Transformer {
	ffmpegCfg := config.FFmpegConfig{}
	if len(cfg) > 0 {
		ffmpegCfg = cfg[0]
	}
	return &transformer{
		ffmpeg: newFFmpegRuntime(ffmpegCfg),
	}
}

type transformer struct {
	ffmpeg ffmpegRuntime
}

// FFmpegAvailable reports whether the configured ffmpeg binary can be resolved.
func (t *transformer) FFmpegAvailable() bool {
	return t.ffmpeg.available()
}

func (t *transformer) FFprobePath() string {
	return t.ffmpeg.ffprobePath
}

// ImageMagickAvailable reports whether the ImageMagick `convert` binary is in PATH.
func (t *transformer) ImageMagickAvailable() bool {
	_, err := exec.LookPath("convert")
	return err == nil
}

// LibreOfficeAvailable reports whether the `soffice` binary is in PATH.
func (t *transformer) LibreOfficeAvailable() bool {
	_, err := exec.LookPath("soffice")
	return err == nil
}

// CheckExternalDeps returns the names of required external binaries that are missing.
// ffmpeg is required for video thumbnails and PDF slideshows.
// convert (ImageMagick) is required for image and PDF thumbnails.
// soffice (LibreOffice) is required for office document thumbnails.
func (t *transformer) CheckExternalDeps() []string {
	var missing []string
	if !t.ffmpeg.available() {
		missing = append(missing, "ffmpeg")
	}
	for _, bin := range []string{"convert", "soffice", "pdftotext"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	return missing
}
