package transform

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const sniffFirstBytes = 512

const (
	FormatJPEG = "jpeg"
	FormatMP4  = "mp4"
	FormatPNG  = "png"
	FormatWebM = "webm"
	FormatWebP = "webp"
)

const (
	MimeImageGIF               = "image/gif"
	MimeImageJPEG              = "image/jpeg"
	MimeImagePNG               = "image/png"
	MimeImageWebP              = "image/webp"
	MimeVideoMP4               = "video/mp4"
	MimeVideoWebM              = "video/webm"
	MimeTextPlain              = "text/plain"
	MimeTextHTML               = "text/html"
	MimeApplicationOctetStream = "application/octet-stream"
)

// DetectMimeType sniffs the MIME type of the file at filePath.
// When content sniffing returns a generic type (zip, octet-stream, plain text),
// it falls back to extension-based lookup to correctly identify OOXML/ODF formats
// that are zip-based and would otherwise be misidentified.
func DetectMimeType(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sniff := make([]byte, sniffFirstBytes)
	n, _ := f.Read(sniff)
	mimeType := stripMimeParams(http.DetectContentType(sniff[:n]))

	if isGenericMime(mimeType) {
		if ext := filepath.Ext(filePath); ext != "" {
			if byExt := stripMimeParams(mime.TypeByExtension(ext)); byExt != "" {
				return byExt, nil
			}
		}
	}

	return mimeType, nil
}

func stripMimeParams(ct string) string {
	if before, _, ok := strings.Cut(ct, ";"); ok {
		return strings.TrimSpace(before)
	}
	return ct
}

func isGenericMime(ct string) bool {
	switch ct {
	case "application/zip", "application/octet-stream", "text/plain":
		return true
	}
	return false
}

// IsImageMime reports whether mime is an image/* type.
func IsImageMime(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// IsVideoMime reports whether mime is a video/* type.
func IsVideoMime(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

// IsAudioMime reports whether mime is an audio/* type.
func IsAudioMime(mime string) bool {
	return strings.HasPrefix(mime, "audio/")
}

// IsPdfMime reports whether mime is application/pdf.
func IsPdfMime(mime string) bool {
	return mime == "application/pdf"
}

// IsAutoTaggable reports whether mime is eligible for AI auto-tagging.
func IsAutoTaggable(mime string) bool {
	return IsImageMime(mime) || IsVideoMime(mime) || IsPdfMime(mime) || IsAudioMime(mime)
}

// IsTextMime reports whether mime is text/*.
func IsTextMime(mime string) bool {
	return strings.HasPrefix(mime, "text/") || mime == "application/x-subrip"
}

// IsFontMime reports whether mime is a font/* type.
func IsFontMime(mime string) bool {
	return strings.HasPrefix(mime, "font/")
}

// IsDocumentMime reports whether mime is an office document type handled by LibreOffice.
func IsDocumentMime(mime string) bool {
	switch mime {
	case
		"application/vnd.oasis.opendocument.presentation",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.oasis.opendocument.text",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/rtf",
		"text/html",
		"application/vnd.oasis.opendocument.spreadsheet",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"text/csv":
		return true
	}
	return false
}

// MimeToExt returns a file extension (with leading dot) for the given MIME type.
// Returns ".bin" if no extension is known.
func MimeToExt(ct string) string {
	ms, err := mime.ExtensionsByType(ct)
	if err == nil && len(ms) > 0 {
		return ms[0]
	}
	return ".bin"
}

// FormatExtension maps a format name to a file extension.
func FormatExtension(format string) string {
	switch strings.ToLower(format) {
	case FormatWebM:
		return ".webm"
	case FormatMP4:
		return ".mp4"
	case FormatPNG:
		return ".png"
	case FormatWebP:
		return ".webp"
	case "tiff":
		return ".tiff"
	default:
		return ".jpg"
	}
}

func FormatVideoMimeType(format string) string {
	if format == FormatWebM {
		return MimeVideoWebM
	}
	return MimeVideoMP4
}
