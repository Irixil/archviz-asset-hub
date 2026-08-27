package export

import "time"

// Manifest is the JSON structure written to the ZIP and as a remote sidecar.
type Manifest struct {
	DamaskExportVersion string          `json:"damask_export_version"`
	ExportedAt          time.Time       `json:"exported_at"`
	ExportConfigID      string          `json:"export_config_id"`
	ExportRunID         string          `json:"export_run_id"`
	Project             ManifestProject `json:"project"`
	Assets              []ManifestAsset `json:"assets"`
}

// ManifestProject holds project-level metadata in the manifest.
type ManifestProject struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Description  *string            `json:"description,omitempty"`
	Color        *string            `json:"color,omitempty"`
	CustomFields []ManifestFieldVal `json:"custom_fields,omitempty"`
}

// ManifestAsset holds per-asset data in the manifest.
type ManifestAsset struct {
	ID               string             `json:"id"`
	OriginalFilename string             `json:"original_filename"`
	Folder           *string            `json:"folder,omitempty"`
	Tags             []string           `json:"tags,omitempty"`
	CustomFields     []ManifestFieldVal `json:"custom_fields,omitempty"`
	Versions         []ManifestVersion  `json:"versions"`
}

// ManifestVersion holds per-version data in the manifest.
type ManifestVersion struct {
	VersionNum  int64             `json:"version_num"`
	IsCurrent   bool              `json:"is_current"`
	ContentHash string            `json:"content_hash"`
	Size        int64             `json:"size"`
	MimeType    string            `json:"mime_type"`
	Comment     *string           `json:"comment,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Path        string            `json:"path"`
	Variants    []ManifestVariant `json:"variants"`
}

// ManifestVariant holds per-variant data in the manifest.
type ManifestVariant struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title,omitempty"`
	Path        string `json:"path"`
	ContentHash string `json:"content_hash,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// ManifestFieldVal holds a custom field value in the manifest.
type ManifestFieldVal struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}
