package api

import (
	"log/slog"
	"strings"

	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
)

// ocrLangNames maps Tesseract language codes to a human-readable English
// name. Only the codes actually installed on the host (see
// transform.ListOCRLanguages) are ever returned to the frontend; this map
// just makes the picker show "French" instead of "fra". Codes with no entry
// here fall back to the raw code.
//
//nolint:gochecknoglobals // intentional package-level list; read-only after init
var ocrLangNames = map[string]string{
	"eng":     langNameEnglish,
	"fra":     "French",
	"spa":     "Spanish",
	"cat":     "Catalan",
	"deu":     "German",
	"ita":     "Italian",
	"por":     "Portuguese",
	"nld":     "Dutch",
	"pol":     "Polish",
	"rus":     "Russian",
	"ukr":     "Ukrainian",
	"ces":     "Czech",
	"slk":     "Slovak",
	"slv":     "Slovenian",
	"hrv":     "Croatian",
	"srp":     "Serbian",
	"bul":     "Bulgarian",
	"ron":     "Romanian",
	"hun":     "Hungarian",
	"fin":     "Finnish",
	"swe":     "Swedish",
	"nor":     "Norwegian",
	"dan":     "Danish",
	"isl":     "Icelandic",
	"ell":     "Greek",
	"tur":     "Turkish",
	"heb":     "Hebrew",
	"ara":     "Arabic",
	"fas":     "Persian",
	"urd":     "Urdu",
	"hin":     "Hindi",
	"ben":     "Bengali",
	"tam":     "Tamil",
	"tel":     "Telugu",
	"mar":     "Marathi",
	"guj":     "Gujarati",
	"kan":     "Kannada",
	"mal":     "Malayalam",
	"pan":     "Punjabi",
	"nep":     "Nepali",
	"sin":     "Sinhala",
	"tha":     "Thai",
	"lao":     "Lao",
	"mya":     "Burmese",
	"khm":     "Khmer",
	"vie":     "Vietnamese",
	"ind":     "Indonesian",
	"msa":     "Malay",
	"tgl":     "Filipino",
	"jpn":     "Japanese",
	"kor":     "Korean",
	"chi_sim": "Chinese (Simplified)",
	"chi_tra": "Chinese (Traditional)",
	"amh":     "Amharic",
	"swa":     "Swahili",
	"afr":     "Afrikaans",
	"sqi":     "Albanian",
	"hye":     "Armenian",
	"aze":     "Azerbaijani",
	"bel":     "Belarusian",
	"bos":     "Bosnian",
	"est":     "Estonian",
	"eus":     "Basque",
	"glg":     "Galician",
	"kat":     "Georgian",
	"kaz":     "Kazakh",
	"lav":     "Latvian",
	"lit":     "Lithuanian",
	"mkd":     "Macedonian",
	"mlt":     "Maltese",
	"mon":     "Mongolian",
	"uzb":     "Uzbek",
	"cym":     "Welsh",
	"gle":     "Irish",
	"epo":     "Esperanto",
	"lat":     "Latin",
}

func ocrLangName(code string) string {
	if name, ok := ocrLangNames[code]; ok {
		return name
	}
	return strings.ToUpper(code)
}

// OCRLanguageResponse describes one Tesseract language available on this host.
type OCRLanguageResponse struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// OCRLanguagesResponse lists Tesseract languages available on this host.
type OCRLanguagesResponse struct {
	Available bool                  `json:"available"`
	Languages []OCRLanguageResponse `json:"languages"`
}

// @Summary List available OCR languages
// @Success 200 {object} OCRLanguagesResponse
// @Router /api/v1/workflows/ocr-languages [get].
func (s *Server) handleListOCRLanguages(c fiber.Ctx) error {
	codes, err := transform.ListOCRLanguages(c.Context())
	if err != nil {
		slog.WarnContext(c.Context(), "list OCR languages failed", "error", err)
	}
	out := make([]OCRLanguageResponse, len(codes))
	for i, code := range codes {
		out[i] = OCRLanguageResponse{Code: code, Name: ocrLangName(code)}
	}
	return c.JSON(fiber.Map{
		"available": err == nil,
		"languages": out,
	})
}
