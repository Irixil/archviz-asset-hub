package transform

import (
	"bytes"
	"image"
	"io"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

// EXIF Orientation tag values (TIFF tag 0x0112), per the EXIF spec.
const (
	orientationNormal     = 1
	orientationFlipH      = 2
	orientationRotate180  = 3
	orientationFlipV      = 4
	orientationTranspose  = 5
	orientationRotate270  = 6
	orientationTransverse = 7
	orientationRotate90   = 8
	tiffMagicLen          = 4
)

// decodeOriented decodes an image and applies EXIF orientation correction.
// imaging.AutoOrientation only understands JPEG containers; for TIFF-based
// containers (DNG/CR2/NEF/ARW raw files, plain TIFF) it silently no-ops, so
// those are corrected here via goexif, which parses TIFF-based EXIF directly.
func decodeOriented(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}

	if isTIFFContainer(data) {
		if o := tiffOrientation(data); o > orientationNormal {
			img = applyOrientation(img, o)
		}
	}

	return img, nil
}

func isTIFFContainer(data []byte) bool {
	if len(data) < tiffMagicLen {
		return false
	}
	return (data[0] == 'I' && data[1] == 'I' && data[2] == 0x2a && data[3] == 0x00) ||
		(data[0] == 'M' && data[1] == 'M' && data[2] == 0x00 && data[3] == 0x2a)
}

// tiffOrientation reads the EXIF Orientation tag (1-8), returning 0 if absent/unparseable.
func tiffOrientation(data []byte) (o int) {
	defer func() {
		if recover() != nil {
			o = 0
		}
	}()
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 0
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 0
	}
	v, err := tag.Int(0)
	if err != nil || v < orientationNormal || v > orientationRotate90 {
		return 0
	}
	return v
}

// applyOrientation mirrors imaging's own (unexported) fixOrientation switch,
// so TIFF-container files get the identical geometric correction JPEGs
// already get natively.
func applyOrientation(img image.Image, o int) image.Image {
	switch o {
	case orientationFlipH:
		return imaging.FlipH(img)
	case orientationRotate180:
		return imaging.Rotate180(img)
	case orientationFlipV:
		return imaging.FlipV(img)
	case orientationTranspose:
		return imaging.Transpose(img)
	case orientationRotate270:
		return imaging.Rotate270(img)
	case orientationTransverse:
		return imaging.Transverse(img)
	case orientationRotate90:
		return imaging.Rotate90(img)
	default:
		return img
	}
}
