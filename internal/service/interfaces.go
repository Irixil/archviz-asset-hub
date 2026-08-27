package service

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/repository"
	"damask/server/internal/workflow"
)

// AssetCommentDTO is a comment posted on an asset via a public share.
type AssetCommentDTO struct {
	ID          string
	AssetID     string
	ShareID     string
	AuthorName  string
	AuthorEmail *string
	Body        string
	CreatedAt   time.Time
}

// AssetService handles business logic for asset records.
type AssetService interface {
	Get(ctx context.Context, workspaceID, assetID string) (*AssetDTO, error)
	List(ctx context.Context, params ListAssetsParams) ([]*AssetDTO, error)
	Move(ctx context.Context, workspaceID, assetID string, p MoveAssetParams) (*AssetDTO, error)
	Rename(ctx context.Context, workspaceID, assetID, newStem string) (*AssetDTO, error)
	Delete(ctx context.Context, workspaceID, assetID string) error
	// HardDelete permanently removes the asset, its versions, and all associated storage files.
	HardDelete(ctx context.Context, workspaceID, assetID string) error
	// BulkHardDelete hard-deletes multiple assets atomically.
	BulkHardDelete(ctx context.Context, workspaceID string, assetIDs []string) error
	// BulkSetTag upserts a tag and links it to all the given asset IDs.
	BulkSetTag(ctx context.Context, workspaceID, tagName string, assetIDs []string) error
	// BulkRemoveTag removes a tag from all the given asset IDs (best-effort, skips missing).
	BulkRemoveTag(ctx context.Context, workspaceID, tagName string, assetIDs []string) error
	// BulkMoveProject assigns all given assets to projectID (nil = remove project).
	BulkMoveProject(ctx context.Context, workspaceID string, assetIDs []string, projectID *string) error
	// GetComments returns all share comments posted on an asset.
	GetComments(ctx context.Context, workspaceID, assetID string) ([]AssetCommentDTO, error)
	// CountVersionsByAsset returns the number of non-deleted versions for an asset.
	CountVersionsByAsset(ctx context.Context, assetID string) (int64, error)
	// CountVariantsByCurrentVersion returns the variant count for the asset's current version.
	CountVariantsByCurrentVersion(ctx context.Context, assetID string) (int64, error)
	// IsRebuildingVariants reports whether a rebuild_variants job is pending/processing for the given version.
	IsRebuildingVariants(ctx context.Context, versionID string) (bool, error)
	CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error)
	// RefreshFTS rebuilds the FTS5 index for the asset to include text field values.
	RefreshFTS(ctx context.Context, assetID string) error
	// ListByFields returns assets matching field-value filters (field[key][op]=value style).
	ListByFields(ctx context.Context, params ListAssetsByFieldsParams) ([]*AssetDTO, error)
	// BatchVersionCounts returns a version count per asset ID.
	BatchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error)
	// BatchVariantCounts returns a variant count (current version) per asset ID.
	BatchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error)
	// WriteAssetDownloadedAsync emits asset_downloaded in a background goroutine.
	WriteAssetDownloadedAsync(workspaceID, assetID, userID string)
	RegenerateThumbnail(ctx context.Context, workspaceID string, assetIDs []string) (jobIDs []string, err error)
}

// CreateVariantParams is the input for VariantService.Create.
type CreateVariantParams struct {
	ID              string
	WorkspaceID     string
	AssetID         string // used for audit; may be empty for job-enqueued variants
	AssetVersionID  string
	Type            string
	StorageKey      string
	TransformParams *string
	Size            *int64
}

type PrepareCreateVariantParams struct {
	WorkspaceID           string
	AssetID               string
	Type                  string
	Params                json.RawMessage
	AssetMimeType         string
	ImageRouterConfigured bool
	DefaultImageModel     string
	DefaultBgRemoveModel  string
	Title                 *string
	IsShared              bool
}

type PreparedCreateVariant struct {
	Type     string
	Params   json.RawMessage
	Title    *string
	IsShared bool
}

type PromoteVariantParams struct {
	AssetID     string
	VariantID   string
	WorkspaceID string
	Name        string
}

type PromoteVariantResult struct {
	NewAssetID  string
	NewAssetURL string
}

type RerunVariantParams struct {
	WorkspaceID string
	AssetID     string
	VariantID   string
	NewParams   map[string]any
}

type WatermarkAssetDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	StorageKey   string  `json:"storage_key"`
	MimeType     string  `json:"mime_type"`
	ThumbnailURL *string `json:"thumbnail_url"`
	Scope        string  `json:"scope"`
}

// ParamHistoryEntry is one distinct previous transform_params set for a variant type.
type ParamHistoryEntry struct {
	Params map[string]any
}

// CommitDraftParams is the input for VariantService.CommitDraft.
type CommitDraftParams struct {
	WorkspaceID     string
	AssetID         string
	AssetVersionID  string
	VariantType     string
	StorageKey      string
	TransformParams *string
	ContentType     string
	Title           *string
}

// VariantService handles business logic for asset variant records.
type VariantService interface {
	List(ctx context.Context, p ListVariantsParams) (*ListVariantsResult, error)
	Get(ctx context.Context, workspaceID, id string) (*VariantDTO, error)
	PrepareCreate(ctx context.Context, p PrepareCreateVariantParams) (PreparedCreateVariant, error)
	Create(ctx context.Context, p CreateVariantParams) (*VariantDTO, error)
	// CommitDraft persists a scratch-based draft as a permanent variant row.
	CommitDraft(ctx context.Context, p CommitDraftParams) (*VariantDTO, error)
	UpdateTitle(ctx context.Context, workspaceID, variantID, title string) error
	UpdateSharing(ctx context.Context, p UpdateVariantsSharingParams) error
	ListSharedByAssets(ctx context.Context, assetIDs []string) ([]SharedVariantDTO, error)
	GetSharedForShare(ctx context.Context, variantID, assetID string) (*VariantDTO, error)
	Delete(ctx context.Context, workspaceID, assetID, variantID string) error
	Promote(ctx context.Context, p PromoteVariantParams) (PromoteVariantResult, error)
	SetAsThumbnail(ctx context.Context, workspaceID, assetID, variantID string) error
	Rerun(ctx context.Context, p RerunVariantParams) error
	// GetParamHistory returns distinct previous transform_params for the given workspace and
	// variant type, ordered most-recent first, capped at 10.
	GetParamHistory(ctx context.Context, workspaceID, variantType string) ([]ParamHistoryEntry, error)
	// WriteVariantQueued emits asset_variant_created for job-queued variants.
	WriteVariantQueued(ctx context.Context, workspaceID, assetID, variantType string)
	// WriteVariantDownloadedAsync emits asset_variant_downloaded in a background goroutine.
	WriteVariantDownloadedAsync(workspaceID, assetID, variantID, variantType, shareID, visitorName string)
}

type WatermarkService interface {
	ResolveWatermarkAsset(ctx context.Context, workspaceID, assetID string) (*WatermarkAssetDTO, error)
}

type TextTrackDTO struct {
	ID             string
	WorkspaceID    string
	AssetID        string
	AssetVersionID *string
	Source         string
	Lang           *string
	Content        string
	StorageKey     *string
	ContentType    *string
	Meta           map[string]any
	Status         string
	Error          *string
	CreatedBy      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateTextTrackParams struct {
	WorkspaceID    string
	AssetID        string
	AssetVersionID *string
	Source         string
	Lang           *string
	InitialContent string
	Params         map[string]any
	CreatedBy      string
	// WorkflowContinuation, when set, is forwarded into the async job payload
	// (honored by ai_image_description and ocr) so the job worker can resume
	// a suspended workflow run once the track is ready. Nil for every
	// non-workflow caller (manual UI, API).
	WorkflowContinuation *workflow.NodeContinuation
}

type TextTrackService interface {
	List(ctx context.Context, workspaceID, assetID string) ([]TextTrackDTO, error)
	Get(ctx context.Context, workspaceID, trackID string) (TextTrackDTO, error)
	Create(ctx context.Context, p CreateTextTrackParams) (TextTrackDTO, error)
	Delete(ctx context.Context, workspaceID, trackID string) error
	RunOCR(
		ctx context.Context,
		workspaceID, assetID, trackID, assetVersionID, storageKey, mimeType, lang, outputFormat string,
	) (text string, wordCount int, err error)
	RunExtractPDF(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error
	RunExtractPlain(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error
	RunExtractDocument(ctx context.Context, workspaceID, assetID, trackID, storageKey, mimeType string) error
	CreateAudioTranscript(
		ctx context.Context,
		workspaceID, assetID, assetVersionID, transcript string,
	) (trackID string, err error)
}

// AutoTagSuggestionDTO is a pending AI tag suggestion awaiting review.
type AutoTagSuggestionDTO struct {
	ID        string
	AssetID   string
	TagName   string
	CreatedAt time.Time
}

// AutoTagService suggests tags for assets via a vision-capable AI provider and
// manages the resulting pending suggestions.
type AutoTagService interface {
	// Enqueue enqueues an auto_tag job for the asset. It is a no-op when the
	// asset's MIME type isn't eligible, or when manual is false and the
	// workspace has auto-tagging disabled.
	Enqueue(ctx context.Context, workspaceID, assetID string, manual bool) error
	// EnqueueForWorkflow enqueues an auto_tag job on behalf of a workflow's
	// action.auto_tag node. Unlike Enqueue, it always runs (bypassing the
	// workspace auto-tag-enabled toggle, since this is an explicit workflow
	// step) and lets the node choose "pending" vs "silent" mode per run.
	// When continuation is set, it is carried in the job payload so the job
	// worker can resume the suspended workflow run once tagging finishes.
	EnqueueForWorkflow(
		ctx context.Context,
		workspaceID, assetID, mode string,
		continuation *workflow.NodeContinuation,
	) error
	// IsProviderAvailable reports whether a configured AI provider supports
	// vision tagging for the given MIME type.
	IsProviderAvailable(ctx context.Context, workspaceID, mimeType string) bool
	ListSuggestions(ctx context.Context, workspaceID, assetID string) ([]AutoTagSuggestionDTO, error)
	// AcceptSuggestion applies the suggested tag to its asset and deletes the suggestion.
	// assetID must match the suggestion's asset; mismatches return apperr.ErrNotFound.
	AcceptSuggestion(ctx context.Context, workspaceID, assetID, suggestionID string) (*TagDTO, error)
	// AcceptAll applies every pending suggestion for an asset and returns the count applied.
	AcceptAll(ctx context.Context, workspaceID, assetID string) (int, error)
	// DismissSuggestion deletes a suggestion. assetID must match the suggestion's
	// asset; mismatches return apperr.ErrNotFound.
	DismissSuggestion(ctx context.Context, workspaceID, assetID, suggestionID string) error
	DismissAll(ctx context.Context, workspaceID, assetID string) error
}

// AIProviderModelDTO is a model entry returned for an AI provider.
type AIProviderModelDTO struct {
	ID            string
	Name          string
	ProviderID    string
	PricePerImage float64
	Capabilities  ai.Capability
}

// AIProviderStatusDTO holds the status and available models for a single AI provider.
type AIProviderStatusDTO struct {
	ID           string
	Configured   bool
	KeySource    string
	Capabilities []string
	Models       []AIProviderModelDTO
	// ModelsError is set when the provider is configured but fetching its
	// model list failed (e.g. network timeout), so Models is empty even
	// though the provider itself is usable.
	ModelsError string
}

// WorkspaceService handles business logic for workspace settings, members, and invites.
type WorkspaceService interface {
	Get(ctx context.Context, workspaceID string) (*WorkspaceDTO, error)
	Update(ctx context.Context, workspaceID string, p UpdateWorkspaceParams) (*WorkspaceDTO, error)
	Me(ctx context.Context, workspaceID, userID string) (*WorkspaceMeDTO, error)
	ListForUser(ctx context.Context, userID string) ([]WorkspaceWithRoleDTO, error)
	CountAssets(ctx context.Context, workspaceID string) (int64, error)
	ListAIProviders(ctx context.Context, workspaceID string, capabilities ai.Capability) ([]AIProviderStatusDTO, error)
	GetAIProviderKeyStatus(ctx context.Context, workspaceID, providerName string) (*ai.KeyStatus, error)
	SetAIProviderKey(ctx context.Context, workspaceID, providerName, plainKey string) error
	ClearAIProviderKey(ctx context.Context, workspaceID, providerName string) error
	TestAIProviderKey(ctx context.Context, workspaceID, providerName string) error
	// Members
	GetMember(ctx context.Context, workspaceID, userID string) (*MemberDTO, error)
	ListMembers(ctx context.Context, workspaceID string) ([]MemberDTO, error)
	RemoveMember(ctx context.Context, workspaceID, callerID, targetUserID string) error
	UpdateMemberRole(ctx context.Context, workspaceID, callerID, targetUserID string, role string) error
	// Invites
	CreateInvite(ctx context.Context, workspaceID, callerID string, p CreateInviteParams) (*InviteDTO, error)
	ListInvites(ctx context.Context, workspaceID string) ([]InviteDTO, error)
	DeleteInvite(ctx context.Context, workspaceID, inviteID string) error
	AcceptInvite(ctx context.Context, p AcceptInviteParams) (*AcceptInviteResult, error)
}

// VersionWithCountDTO is a VersionDTO enriched with its derived variant count.
type VersionWithCountDTO struct {
	VersionDTO

	VariantCount int64
}

// VersionService handles business logic for asset version records.
type VersionService interface {
	List(ctx context.Context, assetID string) ([]*VersionDTO, error)
	ListWithVariantCount(ctx context.Context, assetID string) ([]*VersionWithCountDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*VersionDTO, error)
	GetCurrentByAsset(ctx context.Context, assetID string) (*VersionDTO, error)
	// GetFirstByAsset returns the oldest non-deleted version for an asset.
	GetFirstByAsset(ctx context.Context, assetID string) (*VersionDTO, error)
	// GetByHash returns a version matching assetID + contentHash, or ErrNotFound.
	GetByHash(ctx context.Context, assetID, contentHash string) (*VersionDTO, error)
	// NextVersionNum returns the next version number for the asset.
	NextVersionNum(ctx context.Context, assetID string) (int64, error)
	// Create persists a new version row (is_current should be false; call SetCurrent after).
	Create(ctx context.Context, v *VersionDTO) (*VersionDTO, error)
	// UploadNewVersion handles the full asset version upload workflow.
	UploadNewVersion(ctx context.Context, p UploadAssetVersionParams) (*UploadAssetVersionResult, error)
	// SetCurrent atomically promotes versionID to current.
	SetCurrent(ctx context.Context, assetID, versionID string) error
	// SetAssetThumbnail updates assets.thumbnail_key.
	SetAssetThumbnail(ctx context.Context, assetID string, key *string) error
	Delete(ctx context.Context, workspaceID, assetID, versionID string) error
	// WriteVersionUploaded emits the asset_version_uploaded audit event (called from upload handler).
	WriteVersionUploaded(ctx context.Context, workspaceID, assetID string, v *VersionDTO, comment string)
	// WriteVersionRestored emits the asset_version_restored audit event (called from restore handler).
	WriteVersionRestored(ctx context.Context, workspaceID, assetID string, fromVersionNum, toVersionNum int64)
}

// ShareService handles business logic for share link records.
type ShareService interface {
	List(ctx context.Context, workspaceID string) ([]*ShareDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*ShareDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateShareParams) (*ShareDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateShareParams) (*ShareDTO, error)
	Revoke(ctx context.Context, workspaceID, id string) error
}

// EmbedTokenService handles business logic for public asset embed tokens.
type EmbedTokenService interface {
	// GetOrCreate returns the active token for the asset, creating one if none
	// exists. Idempotent — safe to call on every "copy link" click.
	GetOrCreate(ctx context.Context, workspaceID, assetID, userID string) (*EmbedTokenDTO, error)
	// GetActive returns the active token for an asset, or apperr.ErrNotFound.
	GetActive(ctx context.Context, workspaceID, assetID string) (*EmbedTokenDTO, error)
	// Revoke revokes the active token. Returns apperr.ErrNotFound if none active.
	Revoke(ctx context.Context, workspaceID, assetID string) error
	// ResolveCurrentFile is called by the public /e/:token endpoints. No workspace
	// auth — the token id is the only key. Returns apperr.ErrNotFound or ErrGone.
	ResolveCurrentFile(ctx context.Context, tokenID string) (*ResolvedEmbed, error)
}

// WorkflowTriggerPublisher publishes workflow trigger events.
type WorkflowTriggerPublisher interface {
	Dispatch(ctx context.Context, eventType string, data map[string]any) error
}

// WorkflowService handles workflow CRUD and execution orchestration.
type WorkflowService interface {
	List(ctx context.Context, workspaceID string, params ListWorkflowsParams) ([]WorkflowDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*WorkflowDTO, error)
	Create(ctx context.Context, workspaceID, createdBy string, p CreateWorkflowParams) (*WorkflowDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateWorkflowParams) (*WorkflowDTO, error)
	SetEnabled(ctx context.Context, workspaceID, id string, enabled bool) error
	Delete(ctx context.Context, workspaceID, id string) error
	TriggerManual(ctx context.Context, workspaceID, id, assetID string) (string, error)
	TriggerManualBulk(ctx context.Context, workspaceID, workflowID string, assetIDs []string) ([]string, error)
	TriggerWebhook(ctx context.Context, id, token string, body []byte) (string, error)
	GetRun(ctx context.Context, workspaceID, runID string) (*WorkflowRunDTO, error)
	ListRuns(ctx context.Context, workspaceID, workflowID string, limit int, cursor string) ([]WorkflowRunDTO, error)
	ListAllRuns(ctx context.Context, workspaceID string, limit int, cursor string) ([]WorkflowRunDTO, error)
	FindCoveringWorkflow(
		ctx context.Context,
		workspaceID, assetID, assetProjectID, assetFolderID string,
	) (*CoveringWorkflowDTO, error)
	CreateFromVariants(ctx context.Context, workspaceID string, p CreateVariantAutomationParams) (*WorkflowDTO, error)
	GetWebhookToken(ctx context.Context, workspaceID, id string) (string, error)
	RegenerateWebhookToken(ctx context.Context, workspaceID, id string) (string, error)
	NodeSchemas() []WorkflowNodeSchema
	Templates() []WorkflowTemplateDTO
}

// FieldService handles business logic for custom field definitions.
type FieldService interface {
	List(ctx context.Context, workspaceID, scope string) ([]*FieldDefinitionDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*FieldDefinitionDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateFieldDefinitionParams) (*FieldDefinitionDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateFieldDefinitionParams) (*FieldDefinitionDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
	GetStats(ctx context.Context, workspaceID, id string) (FieldStatsDTO, error)
	Reorder(ctx context.Context, workspaceID string, items []ReorderFieldItem) error
	// InheritProjectFields copies inheritable project field values to a newly created asset.
	InheritProjectFields(ctx context.Context, workspaceID, assetID, projectID, userID string) error
	// ListAssetsMissingExif returns the asset IDs that need EXIF extraction jobs enqueued.
	// The caller is responsible for enqueueing the jobs.
	ListAssetsMissingExif(ctx context.Context, workspaceID string) ([]string, error)
	// PurgeExpiredFields hard-deletes field definitions soft-deleted for more than 30 days.
	// Returns the number of definitions deleted.
	PurgeExpiredFields(ctx context.Context) (int, error)
}

// FieldStatsDTO holds usage counts for a field definition.
type FieldStatsDTO struct {
	AssetCount   int64
	ProjectCount int64
}

// ReorderFieldItem is one entry in a reorder request.
type ReorderFieldItem struct {
	ID       string
	Position int64
}

// FieldValueDTO is the service-layer representation of a typed custom field value.
type FieldValueDTO struct {
	FieldID           string
	FieldKey          string
	FieldName         string
	FieldType         string
	FieldSource       string
	FieldOptions      *string
	Value             any
	DefinitionDeleted bool
}

// SetFieldValueInput is one entry in a patch-fields request.
type SetFieldValueInput struct {
	FieldID string
	Value   any
}

// BulkPreviewEntry holds per-field overwrite impact for a set of assets.
type BulkPreviewEntry struct {
	FieldID         string
	FieldName       string
	FieldType       string
	AssetsWithValue int
	DistinctValues  []string
}

// BulkSetValuesResult separates updated and cleared counts from BulkSetValues.
type BulkSetValuesResult struct {
	Updated int64
	Cleared int64
}

// AssetFieldService handles business logic for asset field values.
type AssetFieldService interface {
	GetValues(ctx context.Context, workspaceID, assetID string) ([]*FieldValueDTO, error)
	SetValues(
		ctx context.Context,
		workspaceID, assetID, userID string,
		inputs []SetFieldValueInput,
	) ([]*FieldValueDTO, error)
	// BulkSetValues applies inputs to all assetIDs; returns updated and cleared counts.
	BulkSetValues(
		ctx context.Context,
		workspaceID, userID string,
		assetIDs []string,
		inputs []SetFieldValueInput,
	) (BulkSetValuesResult, error)
	// BulkPreview returns overwrite impact per field for the given asset selection.
	// If fieldIDs is empty, all active (non-deleted) fields for the workspace are used.
	BulkPreview(ctx context.Context, workspaceID string, assetIDs, fieldIDs []string) ([]BulkPreviewEntry, error)
}

// ProjectFieldService handles business logic for project field values.
type ProjectFieldService interface {
	GetValues(ctx context.Context, workspaceID, projectID string) ([]*FieldValueDTO, error)
	SetValues(
		ctx context.Context,
		workspaceID, projectID, userID string,
		inputs []SetFieldValueInput,
	) ([]*FieldValueDTO, error)
}

// CollectionService handles business logic for collection records.
type CollectionService interface {
	List(ctx context.Context, workspaceID string) ([]*CollectionDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*CollectionDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateCollectionParams) (*CollectionDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateCollectionParams) (*CollectionDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
	AddAsset(ctx context.Context, workspaceID, collectionID, assetID string) error
	RemoveAsset(ctx context.Context, workspaceID, collectionID, assetID string) error
	ListForAsset(ctx context.Context, workspaceID, assetID string) ([]*CollectionDTO, error)
	ListAssets(ctx context.Context, workspaceID, collectionID string) ([]*AssetDTO, error)
}

// FolderService handles business logic for folder records.
type FolderService interface {
	Create(ctx context.Context, workspaceID, projectID string, p CreateFolderParams) (*FolderDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*FolderDTO, error)
	List(ctx context.Context, workspaceID, projectID string) ([]*FolderDTO, error)
	// ListTree returns a recursive tree (depth ≤ 2) of folders with asset counts.
	ListTree(ctx context.Context, workspaceID, projectID string) ([]*FolderTreeDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateFolderParams) (*FolderDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// BulkDeleteTagsResult carries the counts returned by TagService.BulkDelete.
type BulkDeleteTagsResult struct {
	Deleted           int
	RemovedFromAssets int64
}

// MergeTagsResult carries the outcome of TagService.Merge.
type MergeTagsResult struct {
	MergedAssets int64
	Target       *TagDTO
}

type SystemTagScope struct {
	FolderID  *string
	ProjectID *string
}

// TagService handles business logic for tag records.
type TagService interface {
	List(ctx context.Context, workspaceID string, includeSystem bool) ([]*TagDTO, error)
	// GetByName returns the tag with the given name in the workspace, or ErrNotFound.
	GetByName(ctx context.Context, workspaceID, name string) (*TagDTO, error)
	Create(ctx context.Context, workspaceID string, p CreateTagParams) (*TagDTO, error)
	Patch(ctx context.Context, workspaceID, currentName string, p PatchTagParams) (*TagDTO, error)
	EnsureSystemTag(ctx context.Context, workspaceID, name string) error
	Delete(ctx context.Context, workspaceID string, names []string) error
	// BulkDelete deletes the named tags atomically and returns counts.
	BulkDelete(ctx context.Context, workspaceID string, names []string) (BulkDeleteTagsResult, error)
	// Merge reassigns all assets from sources to target (creating it if needed), then deletes sources.
	Merge(ctx context.Context, workspaceID string, sources []string, target string) (MergeTagsResult, error)
	ResolveSystemTag(ctx context.Context, workspaceID, tagName string, scope SystemTagScope) (*AssetDTO, error)
	// TouchLastUsed updates last_used_at for the named tag (fire-and-forget).
	TouchLastUsed(ctx context.Context, workspaceID, name string) error
	ListForAsset(ctx context.Context, assetID string) ([]*TagDTO, error)
	// BatchTagsForAssets returns tag names keyed by asset ID.
	BatchTagsForAssets(ctx context.Context, assetIDs []string) (map[string][]string, error)
	AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (*TagDTO, error)
	RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error
	UpsertForAsset(ctx context.Context, workspaceID, assetID, tagName string) error
	// ApplyTag upserts tagName and links it to assetID, performing the same
	// audit + workflow-trigger side effects as AddToAsset, without returning
	// the resulting TagDTO. The error-only, primitive-typed signature lets
	// callers outside the service package (e.g. internal/jobs) depend on it
	// via a local interface without importing service-package types.
	ApplyTag(ctx context.Context, workspaceID, assetID, tagName string) error
}

// ProjectService handles business logic for project records.
type ProjectService interface {
	Create(ctx context.Context, workspaceID string, p CreateProjectParams) (*ProjectDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*ProjectDTO, error)
	List(ctx context.Context, workspaceID string) ([]*ProjectDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateProjectParams) (*ProjectDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// AuditLogService handles read access to the audit event log.
type AuditLogService interface {
	ListAssetEvents(ctx context.Context, p ListAssetEventsParams) (*AuditEventListDTO, error)
	ListProjectEvents(ctx context.Context, p ListProjectEventsParams) (*AuditEventListDTO, error)
	ListWorkspaceActivity(ctx context.Context, p ListWorkspaceActivityParams) (*ActivityListDTO, error)
	ExportActivity(ctx context.Context, p ExportActivityParams) (csv string, err error)
}

// UploadService handles asset ingestion from an [io.Reader].
type UploadService interface {
	Ingest(ctx context.Context, workspaceID string, r io.Reader, meta UploadMeta) (*AssetDTO, error)
}

// UserService handles business logic for user registration and login.
type UserService interface {
	Register(ctx context.Context, p RegisterUserParams) (*RegisterUserResult, error)
	Login(ctx context.Context, p LoginUserParams) (*LoginUserResult, error)
	GetByID(ctx context.Context, userID string) (*UserDTO, error)
	// CreateWorkspace creates a new workspace owned by userID inside a transaction.
	CreateWorkspace(ctx context.Context, userID, name string) (*WorkspaceDTO, error)
	// GetProfile returns the full user profile including auth methods and provider links.
	GetProfile(ctx context.Context, userID string) (*OIDCUserDTO, error)
	GetProfileByEmail(ctx context.Context, email string) (*OIDCUserDTO, error)
	UpdateProfile(ctx context.Context, userID, displayName string) (*OIDCUserDTO, error)
	UploadAvatar(ctx context.Context, userID string, data []byte) (*OIDCUserDTO, error)
	DeleteAvatar(ctx context.Context, userID string) error
	UpdateAvatarKey(ctx context.Context, userID, storageKey string) (*OIDCUserDTO, error)
	ClearAvatar(ctx context.Context, userID string) (*OIDCUserDTO, error)
	ResetPassword(ctx context.Context, userID, passwordHash string) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPasswordHash string) error
	RequestEmailChange(ctx context.Context, userID, email string) error
	CancelEmailChange(ctx context.Context, userID string) error
	ConfirmEmailChange(ctx context.Context, userID, email string) (*OIDCUserDTO, error)
	DeleteAccount(ctx context.Context, userID, password string, hardDelete bool) error
	// UpsertOIDCUser finds or creates a user from OIDC/Google claims; returns (user, workspaceID).
	UpsertOIDCUser(ctx context.Context, p UpsertOIDCUserParams) (*OIDCUserDTO, error)
	// UpsertCanvaUser finds or creates a user from Canva claims; returns (user, workspaceID).
	UpsertCanvaUser(ctx context.Context, p UpsertCanvaUserParams) (*OIDCUserDTO, error)
	// UnlinkProvider removes the given provider (oidc/google/canva) from the user.
	UnlinkProvider(ctx context.Context, userID, provider string) (*OIDCUserDTO, error)
}

// SharePublicService handles business logic for public (unauthenticated) share access.
type SharePublicService interface {
	// GetActive returns the share if it exists, is not revoked, and is not expired.
	// Returns ErrNotFound for missing/revoked shares and ErrGone for expired shares.
	GetActive(ctx context.Context, shareID string) (*ShareDTO, error)
	IncrementViewCount(ctx context.Context, shareID string) error
	ListAssets(ctx context.Context, targetType, targetID string) ([]*PublicAssetDTO, error)
	GetAsset(ctx context.Context, assetID string) (*PublicAssetDTO, error)
	GetAssetFile(ctx context.Context, assetID string) (*PublicAssetFileDTO, error)
	GetAssetThumb(ctx context.Context, assetID string) (*PublicAssetThumbDTO, error)
	IsAssetInTarget(ctx context.Context, targetType, targetID, assetID string) (bool, error)
	CreateComment(ctx context.Context, p CreateShareCommentParams) (*ShareCommentDTO, error)
	ListCommentsByShare(ctx context.Context, shareID string) ([]*ShareCommentDTO, error)
	ListCommentsByShareAndAsset(ctx context.Context, shareID, assetID string) ([]*ShareCommentDTO, error)
	DeleteComment(ctx context.Context, shareID, commentID string) error
	// GetOwnerShare returns a share verified to belong to the workspace (for owner moderation).
	GetOwnerShare(ctx context.Context, workspaceID, shareID string) (*ShareDTO, error)
}

// IntegrationService handles OAuth connection management.
type IntegrationService interface {
	ListConnections(ctx context.Context, workspaceID string) ([]*ConnectionDTO, error)
	DeleteConnection(ctx context.Context, workspaceID, id string) error
	UpsertConnection(ctx context.Context, p UpsertConnectionParams) error
}

// StackService handles business logic for stack export and merge.
type StackService interface {
	ExportZip(ctx context.Context, workspaceID string, p ExportZipParams, w io.Writer) error
	EnqueueMerge(ctx context.Context, workspaceID, userID string, p MergeParams) (jobID string, err error)
}

// IngressService handles business logic for ingress sources, rules, and log entries.
type IngressService interface {
	// Sources
	ListSources(ctx context.Context, workspaceID string) ([]*IngressSourceDTO, error)
	GetSource(ctx context.Context, workspaceID, id string) (*IngressSourceDTO, error)
	CreateSource(
		ctx context.Context,
		workspaceID, userID string,
		p CreateIngressSourceParams,
	) (*IngressSourceDTO, error)
	UpdateSource(ctx context.Context, workspaceID, id string, p UpdateIngressSourceParams) (*IngressSourceDTO, error)
	DeleteSource(ctx context.Context, workspaceID, id string) error
	TestSource(ctx context.Context, workspaceID, id string) error
	TriggerPoll(ctx context.Context, workspaceID, id string) (jobID string, err error)
	// Rules
	ListRules(ctx context.Context, workspaceID, sourceID string) ([]*IngressRuleDTO, error)
	CreateRule(ctx context.Context, workspaceID, sourceID string, p CreateIngressRuleParams) (*IngressRuleDTO, error)
	UpdateRule(
		ctx context.Context,
		workspaceID, sourceID, ruleID string,
		p UpdateIngressRuleParams,
	) (*IngressRuleDTO, error)
	DeleteRule(ctx context.Context, workspaceID, sourceID, ruleID string) error
	ReorderRules(
		ctx context.Context,
		workspaceID, sourceID string,
		entries []ReorderRuleEntry,
	) ([]*IngressRuleDTO, error)
	// Log
	ListLog(ctx context.Context, workspaceID, statusFilter string, limit, offset int64) ([]*IngressLogEntryDTO, error)
	ListSourceLog(ctx context.Context, workspaceID, sourceID string, limit, offset int64) ([]*IngressLogEntryDTO, error)
	DeleteLogEntry(ctx context.Context, workspaceID, entryID string) error
	RetryLogEntry(ctx context.Context, workspaceID, entryID string) (jobID string, err error)
}

// ExportService handles business logic for export configs and runs.
type ExportService interface {
	Create(ctx context.Context, workspaceID, userID string, p CreateExportConfigParams) (*ExportConfigDTO, error)
	Get(ctx context.Context, workspaceID, id string) (*ExportConfigDTO, error)
	List(ctx context.Context, workspaceID string) ([]*ExportConfigDTO, error)
	ListByProject(ctx context.Context, workspaceID, projectID string) ([]*ExportConfigDTO, error)
	Update(ctx context.Context, workspaceID, id string, p UpdateExportConfigParams) (*ExportConfigDTO, error)
	Delete(ctx context.Context, workspaceID, id string) error
	ValidateDestination(ctx context.Context, workspaceID, configID string) error
	ValidateDestinationConfig(ctx context.Context, workspaceID, destType string, destConfig json.RawMessage) error
	TriggerManual(ctx context.Context, workspaceID, userID, configID string) (*ExportRunDTO, error)
	GetRun(ctx context.Context, workspaceID, runID string) (*ExportRunDTO, error)
	ListRuns(ctx context.Context, workspaceID, configID string, limit, offset int) ([]*ExportRunDTO, error)
	ExecuteRun(ctx context.Context, workspaceID, configID, runID string) error
	ListDueConfigs(ctx context.Context) ([]repository.ExportConfig, error)
	CreateRun(ctx context.Context, run repository.ExportRun) (repository.ExportRun, error)
	SetConfigLastRun(ctx context.Context, configID string, p repository.ExportRunResult) error
}
