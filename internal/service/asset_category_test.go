package service_test

import (
	"testing"

	"damask/server/internal/service"
)

func TestPrimaryAssetCategory(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"scene.max":       "3D模型",
		"render.PSD":      "PSD素材",
		"environment.hdr": "HDRI环境",
		"plan.dwg":        "CAD图纸",
		"light.ies":       "IES灯光",
		"materials.zip":   "素材包",
		"preview.JPG":     "图片素材",
		"notes.txt":       "",
	}
	for filename, want := range tests {
		if got := service.PrimaryAssetCategory(filename, "application/octet-stream"); got != want {
			t.Errorf("PrimaryAssetCategory(%q) = %q, want %q", filename, got, want)
		}
	}
}
