package transform

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"testing"

	"github.com/disintegration/imaging"
)

func TestIsTIFFContainer(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"little-endian TIFF", []byte{'I', 'I', 0x2a, 0x00, 0, 0}, true},
		{"big-endian TIFF", []byte{'M', 'M', 0x00, 0x2a, 0, 0}, true},
		{"JPEG", []byte{0xff, 0xd8, 0xff, 0xe0}, false},
		{"PNG", []byte{0x89, 'P', 'N', 'G'}, false},
		{"too short", []byte{'I', 'I'}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTIFFContainer(c.data); got != c.want {
				t.Errorf("isTIFFContainer(%v) = %v, want %v", c.data, got, c.want)
			}
		})
	}
}

func TestApplyOrientation(t *testing.T) {
	src := imaging.New(4, 2, color.NRGBA{255, 0, 0, 255}) // 4 wide, 2 tall

	cases := []struct {
		name         string
		orientation  int
		wantW, wantH int
	}{
		{"unspecified", 0, 4, 2},
		{"normal", 1, 4, 2},
		{"flip horizontal", 2, 4, 2},
		{"rotate 180", 3, 4, 2},
		{"flip vertical", 4, 4, 2},
		{"transpose", 5, 2, 4},
		{"rotate 270", 6, 2, 4},
		{"transverse", 7, 2, 4},
		{"rotate 90", 8, 2, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := applyOrientation(src, c.orientation)
			b := out.Bounds()
			if b.Dx() != c.wantW || b.Dy() != c.wantH {
				t.Errorf("orientation %d: got %dx%d, want %dx%d", c.orientation, b.Dx(), b.Dy(), c.wantW, c.wantH)
			}
		})
	}
}

// buildTestTIFF constructs a minimal uncompressed grayscale TIFF (little-endian)
// with the given EXIF Orientation tag baked into IFD0, so both x/image/tiff and
// goexif can decode it without needing real camera RAW test fixtures.
func buildTestTIFF(t *testing.T, width, height int, orientation uint16) []byte {
	t.Helper()

	pix := make([]byte, width*height)
	for i := range pix {
		pix[i] = 0x80
	}

	const ifdOffset = 8
	type entry struct {
		tag, typ uint16
		count    uint32
		value    uint32
	}

	entries := []entry{
		{256, 3, 1, uint32(width)},       // ImageWidth (SHORT)
		{257, 3, 1, uint32(height)},      // ImageLength (SHORT)
		{258, 3, 1, 8},                   // BitsPerSample (SHORT)
		{259, 3, 1, 1},                   // Compression: none
		{262, 3, 1, 1},                   // PhotometricInterpretation: BlackIsZero
		{273, 4, 1, 0},                   // StripOffsets (LONG) - patched below
		{274, 3, 1, uint32(orientation)}, // Orientation (SHORT)
		{277, 3, 1, 1},                   // SamplesPerPixel
		{278, 3, 1, uint32(height)},      // RowsPerStrip
		{279, 4, 1, uint32(len(pix))},    // StripByteCounts (LONG)
	}

	ifdSize := 2 + len(entries)*12 + 4
	imgDataOffset := ifdOffset + uint32(ifdSize)
	for i := range entries {
		if entries[i].tag == 273 {
			entries[i].value = imgDataOffset
		}
	}

	var buf bytes.Buffer
	buf.Write([]byte{'I', 'I', 0x2a, 0x00})
	binary.Write(&buf, binary.LittleEndian, uint32(ifdOffset))

	binary.Write(&buf, binary.LittleEndian, uint16(len(entries)))
	for _, e := range entries {
		binary.Write(&buf, binary.LittleEndian, e.tag)
		binary.Write(&buf, binary.LittleEndian, e.typ)
		binary.Write(&buf, binary.LittleEndian, e.count)
		binary.Write(&buf, binary.LittleEndian, e.value)
	}
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // next IFD offset

	buf.Write(pix)

	if buf.Len() != int(imgDataOffset)+len(pix) {
		t.Fatalf("unexpected TIFF layout: buf len %d, want %d", buf.Len(), int(imgDataOffset)+len(pix))
	}

	return buf.Bytes()
}

func TestDecodeOrientedAppliesTIFFOrientation(t *testing.T) {
	data := buildTestTIFF(t, 4, 2, 6) // rotate270 -> dimensions swap

	img, err := decodeOriented(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decodeOriented: %v", err)
	}

	b := img.Bounds()
	if b.Dx() != 2 || b.Dy() != 4 {
		t.Errorf("got %dx%d, want 2x4 (orientation-corrected)", b.Dx(), b.Dy())
	}
}

func TestDecodeOrientedNoOrientationTag(t *testing.T) {
	data := buildTestTIFF(t, 4, 2, 1) // normal orientation, no correction expected

	img, err := decodeOriented(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decodeOriented: %v", err)
	}

	b := img.Bounds()
	if b.Dx() != 4 || b.Dy() != 2 {
		t.Errorf("got %dx%d, want 4x2 (unchanged)", b.Dx(), b.Dy())
	}
}
