package service

import (
	"encoding/json"
	"fmt"
	"io"
	netmail "net/mail"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/assetio"
	"damask/server/internal/workflow"
)

const (
	maxWorkflowNameLength  = 200
	scheduleTypeManual     = "manual"
	scheduleTypeAfterQuiet = "after_quiet"
)

// mapDTOs converts a slice of T to a slice of D using conv.
func mapDTOs[T, D any](rows []T, conv func(T) D) []D {
	out := make([]D, len(rows))
	for i, r := range rows {
		out[i] = conv(r)
	}
	return out
}

// ListAssetsParams holds filters for listing assets via AssetService.List.
type ListAssetsParams struct {
	WorkspaceID string
	// Filters
	ProjectID    *string
	FolderID     *string // non-nil = filter by folder_id; use FolderIsRoot for root
	FolderIsRoot bool    // true = folder_id IS NULL (requires ProjectID)
	CollectionID *string
	TagNames     []string
	SearchQuery  string
	MimePrefix   *string
	// SimilarToIDs, when non-nil, restricts results to these asset IDs.
	SimilarToIDs []string
	// Sort: "created_at" (default), "size", "id", "taken_at"
	SortField string
	SortDesc  bool
	// Cursor (opaque; parsed by handler from cursor query param)
	CursorField string
	CursorValue string
	CursorID    string
	Limit       int64
}

// FieldFilter is a typed field[key][op]=value filter for asset listing.
type FieldFilter struct {
	Key      string
	Operator string // eq | lt | lte | gt | gte | contains | starts_with
	Value    string
}

// ListAssetsByFieldsParams holds parameters for field-filter-based asset listing.
type ListAssetsByFieldsParams struct {
	WorkspaceID  string
	FieldFilters []FieldFilter
	CursorAt     *string // raw cursor value (created_at string)
	CursorID     *string
	Limit        int64
}

// MoveAssetParams holds the destination for AssetService.Move.
// Nil fields mean "keep existing value". An empty-string pointer clears the field.
type MoveAssetParams struct {
	FolderID  *string
	ProjectID *string
}

// AssetDTO is the output of AssetService methods.
type AssetDTO struct {
	ID                   string
	WorkspaceID          string
	ProjectID            *string
	FolderID             *string
	DerivedFromAssetID   *string
	OriginalFilename     string
	StorageKey           string
	MimeType             string
	Size                 int64
	Width                *int64
	Height               *int64
	ThumbnailKey         *string
	ThumbnailContentType string
	Metadata             *string
	CurrentVersionID     *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	// DuplicateOf is set when a content-hash duplicate was found elsewhere in
	// the workspace and duplicate_detection_mode is "warn".
	DuplicateOf *assetio.DuplicateMatch
}

type UploadAssetVersionParams struct {
	WorkspaceID string
	AssetID     string
	Filename    string
	ContentType string
	Comment     string
	UserID      string
	Reader      io.Reader
}

type UploadAssetVersionResult struct {
	Asset   *AssetDTO
	Version *VersionDTO
}

type CreateWorkflowParams struct {
	Name                 string
	Description          string
	Graph                string
	TriggerConfig        string
	NotifyOnFailureEmail string
}

func (p CreateWorkflowParams) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	if len(p.Name) > maxWorkflowNameLength {
		return fmt.Errorf("name must not exceed %d characters: %w", maxWorkflowNameLength, apperr.ErrInvalidInput)
	}
	var graph workflow.Graph
	if err := json.Unmarshal([]byte(p.Graph), &graph); err != nil {
		return fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
	}
	if err := graph.Validate(); err != nil {
		return fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
	}
	if err := validateWorkflowFailureEmail(p.NotifyOnFailureEmail); err != nil {
		return err
	}
	return nil
}

type UpdateWorkflowParams struct {
	Name                 *string
	Description          *string
	Graph                *string
	NotifyOnFailureEmail *string
}

func (p UpdateWorkflowParams) Validate() error {
	if p.Name != nil && strings.TrimSpace(*p.Name) == "" {
		return fmt.Errorf("name must not be empty: %w", apperr.ErrInvalidInput)
	}
	if p.Graph != nil {
		var graph workflow.Graph
		if err := json.Unmarshal([]byte(*p.Graph), &graph); err != nil {
			return fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
		}
		if err := graph.Validate(); err != nil {
			return fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
		}
	}
	if p.NotifyOnFailureEmail != nil {
		if err := validateWorkflowFailureEmail(*p.NotifyOnFailureEmail); err != nil {
			return err
		}
	}
	return nil
}

type WorkflowDTO struct {
	ID                   string
	WorkspaceID          string
	Name                 string
	Description          string
	Enabled              bool
	TriggerType          string
	Graph                string
	NotifyOnFailureEmail string
	LastRunAt            *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ListWorkflowsParams struct {
	TriggerType *string
	EnabledOnly bool
}

type AutomationScope string

const (
	AutomationScopeWorkspace AutomationScope = "workspace"
	AutomationScopeProject   AutomationScope = "project"
	AutomationScopeFolder    AutomationScope = "folder"
	AutomationScopeAsset     AutomationScope = "asset"
)

type CreateVariantAutomationParams struct {
	AssetID   string
	CreatedBy string
	Scope     AutomationScope
}

func (p CreateVariantAutomationParams) Validate() error {
	if strings.TrimSpace(p.AssetID) == "" {
		return fmt.Errorf("asset_id is required: %w", apperr.ErrInvalidInput)
	}
	switch p.Scope {
	case AutomationScopeWorkspace, AutomationScopeProject, AutomationScopeFolder, AutomationScopeAsset:
		return nil
	default:
		return fmt.Errorf("scope must be asset, workspace, project, or folder: %w", apperr.ErrInvalidInput)
	}
}

type CoveringWorkflowDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WorkflowURL string `json:"workflow_url"`
	Scope       string `json:"scope"`
}

type ListVariantsParams struct {
	WorkspaceID    string
	AssetID        string
	AssetProjectID string
	AssetFolderID  string
}

type ListVariantsResult struct {
	Variants         []*VariantDTO
	CoveringWorkflow *CoveringWorkflowDTO
}

type WorkflowRunDTO struct {
	ID          string
	WorkflowID  string
	Status      string
	TriggerData map[string]any
	Error       *string
	StartedAt   *time.Time
	CompletedAt *time.Time
	Steps       []WorkflowRunStepDTO
	CreatedAt   time.Time
}

type WorkflowRunStepDTO struct {
	NodeID      string
	NodeType    string
	Status      string
	Attempt     int
	InputCtx    map[string]any
	OutputCtx   map[string]any
	Error       *string
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type WorkflowNodePort struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type WorkflowNodeSchema struct {
	Type         string             `json:"type"`
	Label        string             `json:"label"`
	Category     string             `json:"category"`
	Description  string             `json:"description"`
	Inputs       []WorkflowNodePort `json:"inputs"`
	Outputs      []WorkflowNodePort `json:"outputs"`
	ConfigSchema json.RawMessage    `json:"config_schema"`
}

type WorkflowTemplateDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TriggerType string `json:"trigger_type"`
	Graph       string `json:"graph"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
}

func validateWorkflowFailureEmail(raw string) error {
	email := strings.TrimSpace(raw)
	if email == "" {
		return nil
	}
	if _, err := netmail.ParseAddress(email); err != nil {
		return fmt.Errorf("notify_on_failure_email is invalid: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// ---- Export DTOs ----

// ExportConfigDTO is the outbound representation of an export config.
// dest_config is never included — credentials stay server-side.
type ExportConfigDTO struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	ProjectID       string     `json:"project_id"`
	Label           string     `json:"label"`
	DestType        string     `json:"dest_type"`
	Versions        string     `json:"versions"`
	IncludeVariants bool       `json:"include_variants"`
	ScheduleType    string     `json:"schedule_type"`
	QuietMinutes    *int       `json:"quiet_minutes,omitempty"`
	Enabled         bool       `json:"enabled"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	LastRunStatus   *string    `json:"last_run_status,omitempty"`
	LastError       *string    `json:"last_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ExportRunDTO is the outbound representation of a single export run.
type ExportRunDTO struct {
	ID             string     `json:"id"`
	ExportConfigID string     `json:"export_config_id"`
	TriggeredBy    *string    `json:"triggered_by,omitempty"`
	Status         string     `json:"status"`
	AssetsTotal    int        `json:"assets_total"`
	AssetsExported int        `json:"assets_exported"`
	AssetsSkipped  int        `json:"assets_skipped"`
	BytesWritten   int64      `json:"bytes_written"`
	Error          *string    `json:"error,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CreateExportConfigParams is the input for ExportService.Create.
type CreateExportConfigParams struct {
	ProjectID       string          `json:"project_id"`
	Label           string          `json:"label"`
	DestType        string          `json:"dest_type"`
	DestConfig      json.RawMessage `json:"dest_config"`
	Versions        string          `json:"versions"`
	IncludeVariants bool            `json:"include_variants"`
	ScheduleType    string          `json:"schedule_type"`
	QuietMinutes    *int            `json:"quiet_minutes"`
}

func (p CreateExportConfigParams) Validate() error {
	if len(p.Label) == 0 || len(p.Label) > 128 {
		return fmt.Errorf("label must be 1-128 characters: %w", apperr.ErrInvalidInput)
	}
	if p.DestType != "sftp" && p.DestType != "gdrive" {
		return fmt.Errorf("dest_type must be 'sftp' or 'gdrive': %w", apperr.ErrInvalidInput)
	}
	if p.Versions != "current" && p.Versions != "all" {
		return fmt.Errorf("versions must be 'current' or 'all': %w", apperr.ErrInvalidInput)
	}
	if p.ScheduleType != scheduleTypeManual && p.ScheduleType != scheduleTypeAfterQuiet {
		return fmt.Errorf("schedule_type must be 'manual' or 'after_quiet': %w", apperr.ErrInvalidInput)
	}
	if p.ScheduleType == scheduleTypeAfterQuiet {
		if p.QuietMinutes == nil {
			return fmt.Errorf("quiet_minutes required when schedule_type is 'after_quiet': %w", apperr.ErrInvalidInput)
		}
		if *p.QuietMinutes < 1 || *p.QuietMinutes > 10080 {
			return fmt.Errorf("quiet_minutes must be 1-10080: %w", apperr.ErrInvalidInput)
		}
	}
	if len(p.DestConfig) == 0 {
		return fmt.Errorf("dest_config is required: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// UpdateExportConfigParams is the input for ExportService.Update.
type UpdateExportConfigParams struct {
	Label           string          `json:"label"`
	DestType        string          `json:"dest_type"`
	DestConfig      json.RawMessage `json:"dest_config"`
	Versions        string          `json:"versions"`
	IncludeVariants bool            `json:"include_variants"`
	ScheduleType    string          `json:"schedule_type"`
	QuietMinutes    *int            `json:"quiet_minutes"`
	Enabled         bool            `json:"enabled"`
}

func (p UpdateExportConfigParams) Validate() error {
	return CreateExportConfigParams{
		Label:        p.Label,
		DestType:     p.DestType,
		DestConfig:   p.DestConfig,
		Versions:     p.Versions,
		ScheduleType: p.ScheduleType,
		QuietMinutes: p.QuietMinutes,
	}.Validate()
}
