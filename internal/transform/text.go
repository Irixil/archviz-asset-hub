package transform

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type ImageOfTextOptions struct {
	TextContent string
	FgColorHex  string
	BgColorHex  string
	FontSize    float64
	Dpi         float64
	Width       int
	Height      int
	FontFile    io.ReadCloser // optional, if empty use default font
}

func (t *transformer) GenerateImageOfText(ctx context.Context, opts ImageOfTextOptions) ([]byte, error) {
	_ = ctx

	openTypeFont, err := loadTextFont(opts.FontFile)
	if err != nil {
		return nil, err
	}

	applyImageOfTextDefaults(&opts)

	fgColor := parseHexColorRGBA(opts.FgColorHex, color.RGBA{0, 0, 0, 255})
	bgColor := parseHexColorRGBA(opts.BgColorHex, color.RGBA{255, 255, 255, 255})

	face, err := opentype.NewFace(openTypeFont, &opentype.FaceOptions{
		Size:    opts.FontSize,
		DPI:     opts.Dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}

	metrics := face.Metrics()
	lineHeight := (int(metrics.Height) + 63) / 64 //nolint:mnd // round to nearest integer
	ascent := (metrics.Ascent + 63) &^ 63         //nolint:mnd // round to nearest integer

	margin := 10
	maxWidth := opts.Width - 2*margin   //nolint:mnd // fixed margin on each side
	maxHeight := opts.Height - 2*margin //nolint:mnd // fixed margin on top and bottom

	dst := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	d := &font.Drawer{Dst: dst, Src: &image.Uniform{fgColor}, Face: face}

	wrappedLines := wrapTextLines(d, opts.TextContent, maxWidth)

	y := fixed.I(margin) + ascent
	for _, line := range wrappedLines {
		if (y.Round() + lineHeight) > (margin + maxHeight) {
			break
		}
		d.Dot = fixed.Point26_6{X: fixed.I(margin), Y: y}
		d.DrawString(line)
		y += fixed.I(lineHeight)
	}

	buf := new(bytes.Buffer)
	if err = png.Encode(buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func loadTextFont(fontFile io.ReadCloser) (*opentype.Font, error) {
	if fontFile == nil {
		return opentype.Parse(goregular.TTF)
	}
	data, err := io.ReadAll(fontFile)
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font file: %w", err)
	}
	return f, nil
}

func applyImageOfTextDefaults(opts *ImageOfTextOptions) {
	if opts.TextContent == "" {
		opts.TextContent = " "
	}
	if opts.FgColorHex == "" {
		opts.FgColorHex = "#000000"
	}
	if opts.BgColorHex == "" {
		opts.BgColorHex = "#FFFFFF"
	}
	if opts.Dpi <= 0 {
		opts.Dpi = 72
	}
	if opts.Width <= 0 {
		opts.Width = 400
	}
	if opts.Height <= 0 {
		opts.Height = 400
	}
	if opts.FontSize <= 0 {
		maxLen := 1
		for line := range strings.SplitSeq(opts.TextContent, "\n") {
			if len(line) > maxLen {
				maxLen = len(line)
			}
		}
		//nolint:mnd // heuristic font size based on width and text length
		opts.FontSize = max(8, min(32, float64(opts.Width)/float64(maxLen)*1.5))
	}
}

func parseHexColorRGBA(hex string, fallback color.RGBA) color.RGBA {
	c := fallback
	_, _ = fmt.Sscanf(hex, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	return c
}

func wrapTextLines(d *font.Drawer, text string, maxWidth int) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		words := strings.Fields(line)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		current := words[0]
		for _, word := range words[1:] {
			test := current + " " + word
			if d.MeasureString(test).Round() <= maxWidth {
				current = test
			} else {
				out = append(out, current)
				current = word
			}
		}
		out = append(out, current)
	}
	return out
}
