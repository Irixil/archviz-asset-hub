package audit

// Asset event types.
const (
	EventAssetCreated                 = "asset_created"
	EventAssetThumbnailRegen          = "asset_thumbnail_regen"
	EventAssetRenamed                 = "asset_renamed"
	EventAssetMoved                   = "asset_moved"
	EventAssetTagged                  = "asset_tagged"
	EventAssetUntagged                = "asset_untagged"
	EventAssetFieldSet                = "asset_field_set"
	EventAssetFieldCleared            = "asset_field_cleared"
	EventAssetVersionUploaded         = "asset_version_uploaded"
	EventAssetVersionRestored         = "asset_version_restored"
	EventAssetVersionDeleted          = "asset_version_deleted"
	EventAssetShared                  = "asset_shared"
	EventAssetShareRevoked            = "asset_share_revoked"
	EventAssetDeleted                 = "asset_deleted"
	EventAssetDownloaded              = "asset_downloaded"
	EventAssetVariantCreated          = "asset_variant_created"
	EventAssetVariantDownloaded       = "asset_variant_downloaded"
	EventAssetVariantDeleted          = "asset_variant_deleted"
	EventAssetVariantPromoted         = "asset_variant_promoted"
	EventAssetThumbnailSetFromVariant = "asset_thumbnail_set_from_variant"
	EventAssetVariantRerun            = "asset_variant_rerun"
)

// Project event types.
const (
	EventProjectCreated      = "project_created"
	EventProjectRenamed      = "project_renamed"
	EventProjectFieldSet     = "project_field_set"
	EventProjectFieldCleared = "project_field_cleared"
	EventProjectDeleted      = "project_deleted"
)

// Actor types.
const (
	ActorTypeUser   = "user"
	ActorTypeSystem = "system"
)

// --- Asset payload structs ---

type AssetCreatedPayload struct {
	V        int    `json:"v"`
	Filename string `json:"filename"`
	Source   string `json:"source"` // "upload" | "ingress" | "derived"
	SourceID string `json:"source_id,omitempty"`
}

type AssetRenamedPayload struct {
	V      int    `json:"v"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type AssetMovedPayload struct {
	V               int     `json:"v"`
	BeforeProjectID *string `json:"before_project_id"`
	AfterProjectID  *string `json:"after_project_id"`
	BeforeFolderID  *string `json:"before_folder_id"`
	AfterFolderID   *string `json:"after_folder_id"`
}

type AssetTaggedPayload struct {
	V   int    `json:"v"`
	Tag string `json:"tag"`
}

type AssetUntaggedPayload struct {
	V   int    `json:"v"`
	Tag string `json:"tag"`
}

type AssetFieldSetPayload struct {
	V         int    `json:"v"`
	FieldKey  string `json:"field_key"`
	FieldName string `json:"field_name"`
	Before    any    `json:"before"`
	After     any    `json:"after"`
}

type AssetFieldClearedPayload struct {
	V         int    `json:"v"`
	FieldKey  string `json:"field_key"`
	FieldName string `json:"field_name"`
	Before    any    `json:"before"`
}

type AssetVersionUploadedPayload struct {
	V          int    `json:"v"`
	VersionNum int64  `json:"version_num"`
	Size       int64  `json:"size"`
	Comment    string `json:"comment,omitempty"`
}

type AssetVersionRestoredPayload struct {
	V              int   `json:"v"`
	FromVersionNum int64 `json:"from_version_num"`
	ToVersionNum   int64 `json:"to_version_num"`
}

type AssetVersionDeletedPayload struct {
	V          int   `json:"v"`
	VersionNum int64 `json:"version_num"`
}

type AssetSharedPayload struct {
	V          int     `json:"v"`
	ShareID    string  `json:"share_id"`
	TargetType string  `json:"target_type"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
}

type AssetShareRevokedPayload struct {
	V       int    `json:"v"`
	ShareID string `json:"share_id"`
}

type AssetDeletedPayload struct {
	V int `json:"v"`
}

type AssetDownloadedPayload struct {
	V       int     `json:"v"`
	Via     string  `json:"via"` // "direct" | "share"
	ShareID *string `json:"share_id,omitempty"`
}

type AssetVariantCreatedPayload struct {
	V    int    `json:"v"`
	Type string `json:"type"`
}

type AssetVariantDownloadedPayload struct {
	V           int     `json:"v"`
	VariantID   string  `json:"variant_id"`
	Type        string  `json:"type"`
	ShareID     *string `json:"share_id,omitempty"`
	VisitorName *string `json:"visitor_name,omitempty"`
}

type AssetVariantDeletedPayload struct {
	V         int    `json:"v"`
	VariantID string `json:"variant_id"`
	Type      string `json:"type"`
}

type AssetVariantPromotedPayload struct {
	V          int    `json:"v"`
	VariantID  string `json:"variant_id"`
	NewAssetID string `json:"new_asset_id"`
}

type AssetThumbnailSetFromVariantPayload struct {
	V         int    `json:"v"`
	VariantID string `json:"variant_id"`
}

type AssetVariantRerunPayload struct {
	V         int            `json:"v"`
	VariantID string         `json:"variant_id"`
	Params    map[string]any `json:"params"`
}

// --- Project payload structs ---

type ProjectCreatedPayload struct {
	V    int    `json:"v"`
	Name string `json:"name"`
}

type ProjectRenamedPayload struct {
	V      int    `json:"v"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type ProjectFieldSetPayload struct {
	V         int    `json:"v"`
	FieldKey  string `json:"field_key"`
	FieldName string `json:"field_name"`
	Before    any    `json:"before"`
	After     any    `json:"after"`
}

type ProjectFieldClearedPayload struct {
	V         int    `json:"v"`
	FieldKey  string `json:"field_key"`
	FieldName string `json:"field_name"`
	Before    any    `json:"before"`
}

type ProjectDeletedPayload struct {
	V int `json:"v"`
}
