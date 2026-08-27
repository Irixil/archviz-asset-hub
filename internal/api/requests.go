package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"

	"damask/server/internal/auth"
)

const (
	minPasswordLength           = 8
	maxDisplayNameLength        = 100
	maxVariantNameLength        = 255
	validationRequired          = "required"
	validationPasswordMinLength = "must be at least 8 characters"
	validationCannotBeEmpty     = "cannot be empty"

	apiTargetAsset   = "asset"
	apiTargetProject = "project"
	fieldTypeSelect  = "select"

	appEnvDevelopment      = "development"
	apiCreatedAtField      = "created_at"
	apiErrorKey            = "error"
	apiFilenameKey         = "filename"
	apiJobIDKey            = "job_id"
	apiSameSiteLax         = "Lax"
	apiStatusKey           = "status"
	apiStatusPending       = "pending"
	apiTokenKey            = "token"
	apiTargetNotFound      = "target not found"
	apiWorkspaceIDKey      = "workspace_id"
	apiEnqueuedKey         = "enqueued"
	apiResultsKey          = "results"
	apiFolderRoot          = "root"
	sortFieldSize          = "size"
	sortFieldTakenAt       = "taken_at"
	contentTypeImageJPEG   = "image/jpeg"
	contentTypeOctetStream = "application/octet-stream"
)

// -- auth.go ------------------------------------------------------------------

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *RegisterRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	if r.Email == "" {
		p["email"] = validationRequired
	}
	if r.Password == "" {
		p["password"] = validationRequired
	} else if len(r.Password) < minPasswordLength {
		p["password"] = validationPasswordMinLength
	}
	return p
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Email == "" {
		p["email"] = validationRequired
	}
	if r.Password == "" {
		p["password"] = validationRequired
	}
	return p
}

// -- workspace.go -------------------------------------------------------------

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

func (r *CreateWorkspaceRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	return p
}

type CreateInviteRequest struct {
	Email string    `json:"email"`
	Role  auth.Role `json:"role"` // "editor" or "viewer"
}

func (r *CreateInviteRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Email == "" {
		p["email"] = validationRequired
	}
	if r.Role == "" {
		r.Role = auth.Editor
	}
	if r.Role != auth.Editor && r.Role != auth.Viewer {
		p["role"] = "must be editor or viewer"
	}
	return p
}

type UpdateMemberRoleRequest struct {
	Role auth.Role `json:"role"`
}

func (r *UpdateMemberRoleRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Role != auth.Owner && r.Role != auth.Editor && r.Role != auth.Viewer {
		p["role"] = "must be owner, editor, or viewer"
	}
	return p
}

type AcceptInviteRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (r *AcceptInviteRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Token == "" {
		p["token"] = validationRequired
	}
	if r.Name == "" {
		p["name"] = validationRequired
	}
	if r.Password == "" {
		p["password"] = validationRequired
	} else if len(r.Password) < minPasswordLength {
		p["password"] = validationPasswordMinLength
	}
	return p
}

type SwitchWorkspaceRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

func (r *SwitchWorkspaceRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.WorkspaceID == "" {
		p["workspace_id"] = validationRequired
	}
	return p
}

// -- users/auth account management -------------------------------------------

type UpdateMeRequest struct {
	DisplayName string `json:"display_name"`
}

func (r *UpdateMeRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.DisplayName = strings.TrimSpace(r.DisplayName)
	if r.DisplayName == "" {
		p["display_name"] = validationRequired
	} else if len(r.DisplayName) > maxDisplayNameLength {
		p["display_name"] = "must be 100 characters or fewer"
	}
	return p
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

func (r *ForgotPasswordRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" {
		p["email"] = validationRequired
	} else if _, err := mail.ParseAddress(r.Email); err != nil {
		p["email"] = "invalid"
	}
	return p
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (r *ResetPasswordRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if strings.TrimSpace(r.Token) == "" {
		p["token"] = validationRequired
	}
	if len(r.Password) < minPasswordLength {
		p["password"] = validationPasswordMinLength
	}
	return p
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (r *ChangePasswordRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.NewPassword) < minPasswordLength {
		p["new_password"] = validationPasswordMinLength
	}
	return p
}

type RequestEmailChangeRequest struct {
	Email string `json:"email"`
}

func (r *RequestEmailChangeRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" {
		p["email"] = validationRequired
	} else if _, err := mail.ParseAddress(r.Email); err != nil {
		p["email"] = "invalid"
	}
	return p
}

type DeleteMeRequest struct {
	Password string `json:"password"`
}

type ShareAccessRequest struct {
	VisitorName string `json:"visitor_name"`
	Password    string `json:"password"`
}

func (r *ShareAccessRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.VisitorName = strings.TrimSpace(r.VisitorName)
	if r.VisitorName == "" {
		p["visitor_name"] = validationRequired
	}
	if len(r.VisitorName) > maxDisplayNameLength {
		p["visitor_name"] = "max 100 characters"
	}
	return p
}

type UpdateWorkspaceSettingsRequest struct {
	VersionRetentionCount  int64   `json:"version_retention_count"`
	ExifKeep               bool    `json:"exif_keep"`
	ExifKeepGPS            bool    `json:"exif_keep_gps"`
	LockedTaxonomy         *bool   `json:"locked_taxonomy"`
	AutoTagEnabled         *bool   `json:"auto_tag_enabled"`
	AutoTagMode            *string `json:"auto_tag_mode"`
	DuplicateDetectionMode *string `json:"duplicate_detection_mode"`
}

func (r *UpdateWorkspaceSettingsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.VersionRetentionCount < 0 {
		p["version_retention_count"] = "must be 0 (keep all) or a positive integer"
	}
	if r.AutoTagMode != nil && *r.AutoTagMode != "pending" && *r.AutoTagMode != "silent" {
		p["auto_tag_mode"] = "must be 'pending' or 'silent'"
	}
	if r.DuplicateDetectionMode != nil {
		switch *r.DuplicateDetectionMode {
		case "off", "warn", "block":
		default:
			p["duplicate_detection_mode"] = "must be 'off', 'warn', or 'block'"
		}
	}
	return p
}

// -- projects.go --------------------------------------------------------------

type CreateProjectRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

func (r *CreateProjectRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	return p
}

type UpdateProjectRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Color        *string `json:"color"`
	CoverAssetID *string `json:"cover_asset_id"`
}

func (r *UpdateProjectRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		if trimmed == "" {
			p["name"] = validationCannotBeEmpty
		} else {
			r.Name = &trimmed
		}
	}
	return p
}

// -- folders.go ---------------------------------------------------------------

type CreateFolderRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
	Position int64   `json:"position"`
}

func (r *CreateFolderRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	return p
}

type UpdateFolderRequest struct {
	Name     *string `json:"name"`
	Position *int64  `json:"position"`
}

func (r *UpdateFolderRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		if trimmed == "" {
			p["name"] = validationCannotBeEmpty
		} else {
			r.Name = &trimmed
		}
	}
	return p
}

// -- assets.go ----------------------------------------------------------------

type BulkTagRequest struct {
	AssetIDs []string `json:"asset_ids"`
	TagName  string   `json:"tag_name"`
	Mode     string   `json:"mode"` // "add" (default) | "remove"
}

func (r *BulkTagRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.TagName = strings.TrimSpace(strings.ToLower(r.TagName))
	if r.TagName == "" {
		p["tag_name"] = validationRequired
	}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = "required, must contain at least one id"
	}
	if r.Mode == "" {
		r.Mode = "add"
	} else if r.Mode != "add" && r.Mode != "remove" {
		p["mode"] = `must be "add" or "remove"`
	}
	return p
}

type BulkProjectRequest struct {
	AssetIDs  []string `json:"asset_ids"`
	ProjectID *string  `json:"project_id"` // null = unassign
}

func (r *BulkProjectRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = validationRequired
	}
	return p
}

type UpdateAssetFolderRequest struct {
	FolderID *string `json:"folder_id"`
}

func (r *UpdateAssetFolderRequest) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

type RenameAssetRequest struct {
	Name string `json:"name"`
}

func (r *RenameAssetRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	return p
}

type BulkDeleteRequest struct {
	AssetIDs []string `json:"asset_ids"`
}

func (r *BulkDeleteRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = validationRequired
	}
	return p
}

// -- tags.go ------------------------------------------------------------------

type AddTagRequest struct {
	Name string `json:"name"`
}

func (r *AddTagRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(strings.ToLower(r.Name))
	if r.Name == "" {
		p["name"] = validationRequired
	}
	return p
}

type createTagRequest struct {
	Name      string  `json:"name"`
	Color     *string `json:"color"`
	GroupName *string `json:"group_name"`
}

func (r *createTagRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(strings.ToLower(r.Name))
	if r.Name == "" {
		p["name"] = validationRequired
	}
	if r.Color != nil {
		*r.Color = strings.ToLower(strings.TrimSpace(*r.Color))
		if !hexColorRegex.MatchString(*r.Color) {
			p["color"] = "must be a valid hex color (e.g. #22c55e)"
		}
	}
	return p
}

type patchTagRequest struct {
	Name      *string `json:"name"`
	Color     *string `json:"color"`
	GroupName *string `json:"group_name"`
}

func (r *patchTagRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Name != nil {
		*r.Name = strings.TrimSpace(strings.ToLower(*r.Name))
		if *r.Name == "" {
			p["name"] = "must not be empty"
		}
	}
	if r.Color != nil {
		*r.Color = strings.ToLower(strings.TrimSpace(*r.Color))
		if !hexColorRegex.MatchString(*r.Color) {
			p["color"] = "must be a valid hex color (e.g. #22c55e)"
		}
	}
	return p
}

type bulkDeleteTagsRequest struct {
	Names []string `json:"names"`
}

func (r *bulkDeleteTagsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Names) == 0 {
		p["names"] = validationRequired
	}
	for i, n := range r.Names {
		r.Names[i] = strings.TrimSpace(strings.ToLower(n))
	}
	return p
}

type mergeTagsRequest struct {
	Sources []string `json:"sources"`
	Target  string   `json:"target"`
}

func (r *mergeTagsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Target = strings.TrimSpace(strings.ToLower(r.Target))
	if r.Target == "" {
		p["target"] = validationRequired
	}
	if len(r.Sources) == 0 {
		p["sources"] = validationRequired
	}
	for i, s := range r.Sources {
		r.Sources[i] = strings.TrimSpace(strings.ToLower(s))
	}
	return p
}

// -- shares.go ----------------------------------------------------------------

type CreateShareRequest struct {
	Label         string  `json:"label,omitempty"`
	TargetType    string  `json:"target_type"`
	TargetID      string  `json:"target_id"`
	Password      *string `json:"password,omitempty"`
	ExpiresInDays *int    `json:"expires_in_days,omitempty"`
	AllowComments bool    `json:"allow_comments,omitempty"`
	AllowDownload *bool   `json:"allow_download,omitempty"`
}

func (r *CreateShareRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	validTargetTypes := map[string]bool{"collection": true, apiTargetAsset: true, apiTargetProject: true}
	if !validTargetTypes[r.TargetType] {
		p["target_type"] = "must be one of: collection, asset, project"
	}
	if r.TargetID == "" {
		p["target_id"] = validationRequired
	}
	return p
}

type UpdateShareRequest struct {
	Label         *string `json:"label,omitempty"`
	Password      *string `json:"password,omitempty"`
	ClearPassword *bool   `json:"clear_password,omitempty"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
	ClearExpiry   *bool   `json:"clear_expiry,omitempty"`
	AllowComments *bool   `json:"allow_comments,omitempty"`
	AllowDownload *bool   `json:"allow_download,omitempty"`
}

func (r *UpdateShareRequest) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

// -- shares_public.go ---------------------------------------------------------

type CreateCommentRequest struct {
	AssetID     string  `json:"asset_id"`
	AuthorName  string  `json:"author_name"`
	AuthorEmail *string `json:"author_email"`
	Body        string  `json:"body"`
}

func (r *CreateCommentRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.AuthorName = strings.TrimSpace(r.AuthorName)
	r.Body = strings.TrimSpace(r.Body)
	if r.AuthorName == "" {
		p["author_name"] = validationRequired
	}
	if r.Body == "" {
		p["body"] = validationRequired
	}
	if r.AssetID == "" {
		p["asset_id"] = validationRequired
	}
	return p
}

// -- variants.go --------------------------------------------------------------

type CreateVariantRequest struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params" swaggertype:"object"`
}

func (r *CreateVariantRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Type == "" {
		p["type"] = validationRequired
	}
	if len(r.Params) > 0 && !json.Valid(r.Params) {
		p["params"] = "invalid json"
	}
	return p
}

type PromoteVariantRequest struct {
	Name string `json:"name"`
}

func (r *PromoteVariantRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	if len(r.Name) > maxVariantNameLength {
		p["name"] = "max 255 chars"
	}
	return p
}

type RerunVariantRequest struct {
	Params map[string]any `json:"params"`
}

func (r *RerunVariantRequest) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

type UpdateVariantsSharingRequest struct {
	Updates map[string]bool `json:"updates"`
}

func (r *UpdateVariantsSharingRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Updates) == 0 {
		p["updates"] = validationRequired
	}
	return p
}

type PatchVariantRequest struct {
	Title string `json:"title"`
}

func (r *PatchVariantRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(strings.TrimSpace(r.Title)) > maxVariantNameLength {
		p["title"] = fmt.Sprintf("max %d chars", maxVariantNameLength)
	}
	return p
}

type CreateTextTrackRequest struct {
	Source string         `json:"source"`
	Lang   *string        `json:"lang"`
	Params map[string]any `json:"params"`
}

func (r *CreateTextTrackRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Source = strings.TrimSpace(r.Source)
	if r.Source == "" {
		p["source"] = validationRequired
	}
	return p
}

// -- ingress.go ---------------------------------------------------------------

type IngressRuleReq struct {
	Position int64  `json:"position"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

func (r *IngressRuleReq) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Field == "" {
		p["field"] = validationRequired
	}
	if r.Operator == "" {
		p["operator"] = validationRequired
	}
	if r.Value == "" {
		p["value"] = validationRequired
	}
	if r.Action == "" {
		p["action"] = validationRequired
	}
	return p
}

type CreateIngressSourceReq struct {
	Type            string           `json:"type"`
	Label           string           `json:"label"`
	Config          map[string]any   `json:"config,omitempty"`
	DestFolderID    *string          `json:"dest_folder_id,omitempty"`
	DestProjectID   *string          `json:"dest_project_id,omitempty"`
	Enabled         *bool            `json:"enabled,omitempty"`
	PollIntervalMin int64            `json:"poll_interval_min,omitempty"`
	Rules           []IngressRuleReq `json:"rules,omitempty"`
}

func (r *CreateIngressSourceReq) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Type == "" {
		p["type"] = validationRequired
	}
	if r.Label == "" {
		p["label"] = validationRequired
	}
	return p
}

type UpdateIngressSourceReq struct {
	Label           string           `json:"label,omitempty"`
	Config          map[string]any   `json:"config,omitempty"`
	DestFolderID    *json.RawMessage `json:"dest_folder_id,omitempty"    swaggertype:"string"`
	DestProjectID   *json.RawMessage `json:"dest_project_id,omitempty"   swaggertype:"string"`
	Enabled         *bool            `json:"enabled,omitempty"`
	PollIntervalMin int64            `json:"poll_interval_min,omitempty"`
}

func (r *UpdateIngressSourceReq) Valid(_ context.Context) map[string]string {
	return map[string]string{}
}

type ReorderRuleEntry struct {
	ID       string `json:"id"`
	Position int64  `json:"position"`
}

// -- custom_fields_definitions.go (reorder) -----------------------------------

type ReorderFieldEntry struct {
	ID       string `json:"id"`
	Position int64  `json:"position"`
}

type ReorderFieldDefinitionsRequest struct {
	Items []ReorderFieldEntry
}

// UnmarshalJSON lets the client send a bare JSON array while still satisfying
// the Validator interface required by decodeAndValidate.
func (r *ReorderFieldDefinitionsRequest) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &r.Items)
}

func (r *ReorderFieldDefinitionsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Items) == 0 {
		p["items"] = "at least one item required"
	}
	return p
}

// -- custom_fields_definitions.go ---------------------------------------------

type CreateFieldDefinitionRequest struct {
	Scope              string  `json:"scope"`
	Name               string  `json:"name"`
	Key                string  `json:"key"`
	FieldType          string  `json:"field_type"`
	Options            *string `json:"options"`
	Required           bool    `json:"validationRequired"`
	Position           int64   `json:"position"`
	InheritFromProject bool    `json:"inherit_from_project"`
}

func (r *CreateFieldDefinitionRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	if r.Scope != apiTargetAsset && r.Scope != apiTargetProject {
		p["scope"] = "must be 'asset' or 'project'"
	}
	validTypes := map[string]bool{
		"text":          true,
		"number":        true,
		"date":          true,
		"boolean":       true,
		fieldTypeSelect: true,
		"url":           true,
	}
	if !validTypes[r.FieldType] {
		p["field_type"] = "must be one of: text, number, date, boolean, select, url"
	}
	if !keyRegexp.MatchString(r.Key) {
		p["key"] = "must match /^[a-z0-9_]+$/"
	}
	if r.FieldType == fieldTypeSelect {
		if r.Options == nil || *r.Options == "" {
			p["options"] = "required for select fields"
		} else {
			var opts []string
			if err := json.Unmarshal([]byte(*r.Options), &opts); err != nil || len(opts) == 0 {
				p["options"] = "must be a non-empty JSON array of strings"
			}
		}
	}
	return p
}

type UpdateFieldDefinitionRequest struct {
	Name               *string `json:"name"`
	Key                *string `json:"key"`
	FieldType          *string `json:"field_type"`
	Options            *string `json:"options"`
	Required           *bool   `json:"validationRequired"`
	Position           *int64  `json:"position"`
	InheritFromProject *bool   `json:"inherit_from_project"`
}

func (r *UpdateFieldDefinitionRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		if trimmed == "" {
			p["name"] = validationCannotBeEmpty
		} else {
			r.Name = &trimmed
		}
	}
	return p
}

// -- custom_fields_values.go --------------------------------------------------

type PatchAssetFieldsRequest struct {
	Values []FieldValueInput `json:"values"`
}

func (r *PatchAssetFieldsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Values) == 0 {
		p["values"] = validationRequired
	}
	return p
}

type BulkPatchAssetFieldsRequest struct {
	AssetIDs []string          `json:"asset_ids"`
	Values   []FieldValueInput `json:"values"`
}

func (r *BulkPatchAssetFieldsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = validationRequired
	}
	if len(r.Values) == 0 {
		p["values"] = validationRequired
	}
	return p
}

// -- stack_export.go ----------------------------------------------------------

type stackExportRequest struct {
	AssetIDs    []string `json:"asset_ids"`
	Filename    string   `json:"filename"`
	VariantMode string   `json:"variant_mode"` // "" | "none" | "shared" | "all"
}

func (r *stackExportRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Filename = strings.TrimSpace(r.Filename)
	if r.Filename == "" {
		r.Filename = "stack-export"
	}
	if len(r.AssetIDs) == 0 {
		p["asset_ids"] = validationRequired
	}
	switch r.VariantMode {
	case "", "none", "shared", "all":
	default:
		p["variant_mode"] = "must be one of: none, shared, all"
	}
	return p
}

// -- stack_merge.go -----------------------------------------------------------

type stackMergeRequest struct {
	AssetIDs   []string `json:"asset_ids"`
	OutputType string   `json:"output_type"`
	Filename   string   `json:"filename"`
	GifFrameMs int      `json:"gif_frame_ms"`
}

func (r *stackMergeRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	//nolint:mnd // minimum of 2 assets required for a merge.
	if len(r.AssetIDs) < 2 {
		p["asset_ids"] = "at least 2 assets required"
	}
	if r.OutputType != "gif" && r.OutputType != "pdf" {
		p["output_type"] = "must be gif or pdf"
	}
	r.Filename = strings.TrimSpace(r.Filename)
	if r.Filename == "" {
		r.Filename = "stack-merge"
	}
	if r.GifFrameMs <= 0 {
		r.GifFrameMs = 500
	}
	return p
}

// -- collections.go -----------------------------------------------------------

type createCollectionRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AssetIDs    []string `json:"asset_ids"`
}

func (r *createCollectionRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	return p
}

type updateCollectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r *updateCollectionRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		p["name"] = validationRequired
	}
	return p
}

// -- custom_fields_project_values.go ------------------------------------------

type PatchProjectFieldsRequest struct {
	Values []FieldValueInput `json:"values"`
}

func (r *PatchProjectFieldsRequest) Valid(_ context.Context) map[string]string {
	p := map[string]string{}
	if len(r.Values) == 0 {
		p["values"] = validationRequired
	}
	return p
}
