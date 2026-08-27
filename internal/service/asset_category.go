package service

import (
	"path/filepath"
	"strings"
)

// PrimaryAssetCategory returns the architecture library's format-level tag.
// Content-level tags such as "沙发" or "石材" still require human review.
func PrimaryAssetCategory(filename, mimeType string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")

	switch ext {
	case "3dm", "3ds", "abc", "blend", "c4d", "cgeo", "dae", "fbx", "glb", "gltf", "max", "obj", "rvt", "skp", "vrmesh":
		return "3D模型"
	case "psb", "psd":
		return "PSD素材"
	case "hdr", "exr":
		return "HDRI环境"
	case "dgn", "dwg", "dxf":
		return "CAD图纸"
	case "ies":
		return "IES灯光"
	case "7z", "rar", "zip":
		return "素材包"
	case "avif", "bmp", "gif", "heic", "heif", "jpeg", "jpg", "png", "svg", "tif", "tiff", "webp":
		return "图片素材"
	}

	if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return "图片素材"
	}
	return ""
}
