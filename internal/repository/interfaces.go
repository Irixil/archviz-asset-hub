package repository

import (
	"context"
	"time"
)

// AssetRepository handles persistence for Asset records.
type AssetRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Asset, error)
	List(ctx context.Context, params ListAssetsParams) ([]Asset, error)
	Create(ctx context.Context, params CreateAssetParams) (Asset, error)
	Update(ctx context.Context, params UpdateAssetParams) (Asset, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
	// IsProjectCover reports whether the asset is set as the cover of any project.
	IsProjectCover(ctx context.Context, workspaceID, assetID string) (bool, error)
	// IsWorkspaceIcon reports whether the asset is set as the workspace icon.
	IsWorkspaceIcon(ctx context.Context, workspaceID, assetID string) (bool, error)
	// CountByIDs returns how many of the given IDs belong to the workspace.
	CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error)
	// RefreshFTS rebuilds the FTS5 index entry for the asset, including its text field values.
	RefreshFTS(ctx context.Context, assetID string) error
	// ListByFields returns assets matching the given field filters (JOIN-per-filter pattern).
	ListByFields(ctx context.Context, params ListAssetsByFieldsParams) ([]Asset, error)
	// CollectStorageKeys returns all storage keys (asset + versions + variants) for an asset.
	CollectStorageKeys(ctx context.Context, workspaceID, assetID string) (AssetStorageKeys, error)
	// HardDelete permanently deletes the asset DB row (assumes storage cleanup is done by caller).
	HardDelete(ctx context.Context, workspaceID, assetID string) error
	// CountVersionsByAsset returns the number of non-deleted versions for an asset.
	CountVersionsByAsset(ctx context.Context, assetID string) (int64, error)
	// CountVariantsByCurrentVersion returns the number of variants on the asset's current version.
	CountVariantsByCurrentVersion(ctx context.Context, assetID string) (int64, error)
	// IsRebuildingVariants reports whether a rebuild_variants job is pending/processing for the given version.
	IsRebuildingVariants(ctx context.Context, versionID string) (bool, error)
	// ListComments returns all share comments posted on the asset.
	ListComments(ctx context.Context, assetID string) ([]AssetComment, error)
	// BatchVersionCounts returns version counts keyed by asset ID.
	BatchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error)
	// BatchVariantCounts returns variant counts (current version) keyed by asset ID.
	BatchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error)
	// SetProject assigns or clears the project for an asset (projectID = nil clears it).
	SetProject(ctx context.Context, workspaceID, assetID string, projectID *string) error
	// FindWatermarkInFolder returns the oldest asset in folderID whose filename contains
	// "watermark" (case-insensitive), or ErrNotFound.
	FindWatermarkInFolder(ctx context.Context, workspaceID, folderID string) (Asset, error)
	// FindWatermarkInProject returns the oldest such asset anywhere in projectID's folders.
	FindWatermarkInProject(ctx context.Context, workspaceID, projectID string) (Asset, error)
	// FindWatermarkInWorkspace returns the oldest such asset anywhere in the workspace.
	FindWatermarkInWorkspace(ctx context.Context, workspaceID string) (Asset, error)
}

// ProjectRepository handles persistence for Project records.
type ProjectRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Project, error)
	// List returns all projects with their asset counts.
	List(ctx context.Context, workspaceID string) ([]ProjectWithCount, error)
	Create(ctx context.Context, p Project) (Project, error)
	Update(ctx context.Context, p Project) (Project, error)
	// Delete removes the project row. The caller is responsible for calling
	// NullifyAssets inside the same transaction before calling Delete.
	Delete(ctx context.Context, workspaceID, id string) error
	// NullifyAssets sets project_id = NULL on all assets in the project.
	// Must be called inside a transaction before Delete.
	NullifyAssets(ctx context.Context, workspaceID, projectID string) error
}

// FolderRepository handles persistence for Folder records.
type FolderRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Folder, error)
	ListByProject(ctx context.Context, workspaceID, projectID string) ([]Folder, error)
	Create(ctx context.Context, f Folder) (Folder, error)
	Update(ctx context.Context, f Folder) (Folder, error)
	Delete(ctx context.Context, workspaceID, id string) error
	// GetChildren returns direct children of the given folder.
	GetChildren(ctx context.Context, workspaceID, parentID string) ([]Folder, error)
	// NullifyAssets sets folder_id = NULL on all assets in the folder.
	NullifyAssets(ctx context.Context, workspaceID, folderID string) error
	// ListTree returns a recursive tree of folders for a project, each with asset count. Depth ≤ 2.
	ListTree(ctx context.Context, workspaceID, projectID string) ([]FolderTree, error)
}

// TagRepository handles persistence for Tag records.
type TagRepository interface {
	GetByName(ctx context.Context, workspaceID, name string) (Tag, error)
	List(ctx context.Context, workspaceID string, includeSystem bool) ([]Tag, error)
	Upsert(ctx context.Context, workspaceID, name string) (Tag, error)
	EnsureSystemTag(ctx context.Context, workspaceID, name string) error
	UpdateMetadata(ctx context.Context, workspaceID, name string, color, groupName *string) error
	Rename(ctx context.Context, workspaceID, oldName, newName string) error
	Delete(ctx context.Context, workspaceID string, names []string) error
	ListForAsset(ctx context.Context, assetID string) ([]Tag, error)
	AddToAsset(ctx context.Context, assetID, tagID string) error
	RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error
	// BatchTagsForAssets returns tag names keyed by asset ID for the given set of asset IDs.
	BatchTagsForAssets(ctx context.Context, assetIDs []string) (map[string][]string, error)
	// CountAssets returns the number of asset_tags rows for a given tag ID.
	CountAssets(ctx context.Context, tagID string) (int64, error)
	// ReassignAssets moves all asset_tags rows from fromTagID to toTagID (idempotent).
	ReassignAssets(ctx context.Context, fromTagID, toTagID string) error
	// TouchLastUsed updates last_used_at to now for the given tag (fire-and-forget).
	TouchLastUsed(ctx context.Context, workspaceID, name string) error
	FindAssetBySystemTagInFolder(ctx context.Context, workspaceID, tagName, folderID string) (Asset, error)
	FindAssetBySystemTagInProject(ctx context.Context, workspaceID, tagName, projectID string) (Asset, error)
	FindAssetBySystemTagInWorkspace(ctx context.Context, workspaceID, tagName string) (Asset, error)
	// RunInTx executes fn inside a single database transaction.
	RunInTx(ctx context.Context, fn func(tx TagRepository) error) error
}

// CollectionRepository handles persistence for Collection records.
type CollectionRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Collection, error)
	List(ctx context.Context, workspaceID string) ([]Collection, error)
	Create(ctx context.Context, c Collection) (Collection, error)
	Update(ctx context.Context, c Collection) (Collection, error)
	Delete(ctx context.Context, workspaceID, id string) error
	AddAsset(ctx context.Context, collectionID, assetID string) error
	RemoveAsset(ctx context.Context, collectionID, assetID string) error
	// ListForAsset returns all collections (with asset counts) that contain the asset.
	ListForAsset(ctx context.Context, workspaceID, assetID string) ([]Collection, error)
	// CountAssets returns the number of assets in the collection.
	CountAssets(ctx context.Context, collectionID string) (int64, error)
	// ListAssetIDs returns the asset IDs in the collection.
	ListAssetIDs(ctx context.Context, collectionID string) ([]string, error)
}

// ShareRepository handles persistence for Share records.
type ShareRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Share, error)
	// GetPublic returns a share by ID without workspace scoping (for public endpoints).
	GetPublic(ctx context.Context, id string) (Share, error)
	// GetByIDAndWorkspace returns a share only when it belongs to the workspace.
	GetByIDAndWorkspace(ctx context.Context, workspaceID, id string) (Share, error)
	List(ctx context.Context, workspaceID string) ([]Share, error)
	Create(ctx context.Context, s Share) (Share, error)
	Update(ctx context.Context, s Share) (Share, error)
	Revoke(ctx context.Context, workspaceID, id string) error
	IncrementViewCount(ctx context.Context, id string) error
	// Asset listing for public share endpoints (no workspace auth required).
	ListAssetsByTarget(ctx context.Context, targetType, targetID string) ([]PublicAsset, error)
	GetPublicAsset(ctx context.Context, assetID string) (PublicAsset, error)
	GetPublicAssetFile(ctx context.Context, assetID string) (PublicAssetFile, error)
	GetPublicAssetThumb(ctx context.Context, assetID string) (*string, time.Time, error)
	// IsAssetInTarget verifies the asset belongs to the share target.
	IsAssetInTarget(ctx context.Context, targetType, targetID, assetID string) (bool, error)
	// Comment methods.
	CreateComment(ctx context.Context, c ShareComment) (ShareComment, error)
	ListCommentsByShare(ctx context.Context, shareID string) ([]ShareComment, error)
	ListCommentsByShareAndAsset(ctx context.Context, shareID, assetID string) ([]ShareComment, error)
	DeleteComment(ctx context.Context, shareID, commentID string) error
}

// VersionRepository handles persistence for AssetVersion records.
type VersionRepository interface {
	GetByID(ctx context.Context, id string) (AssetVersion, error)
	// GetByIDForWorkspace returns the version only if it belongs to the workspace.
	GetByIDForWorkspace(ctx context.Context, workspaceID, id string) (AssetVersion, error)
	// GetCurrentByAsset returns the current (active) version for an asset.
	GetCurrentByAsset(ctx context.Context, assetID string) (AssetVersion, error)
	// GetFirstByAsset returns the oldest non-deleted version for an asset.
	GetFirstByAsset(ctx context.Context, assetID string) (AssetVersion, error)
	ListByAsset(ctx context.Context, assetID string) ([]AssetVersion, error)
	Create(ctx context.Context, v AssetVersion) (AssetVersion, error)
	// SoftDelete marks the version as deleted without removing the storage file.
	SoftDelete(ctx context.Context, id string) error
	// Delete hard-deletes the version row (used by retention job).
	Delete(ctx context.Context, id string) error
	CountByAsset(ctx context.Context, assetID string) (int64, error)
	// IsReferencedAsCover returns true when the version is set as a project cover or workspace icon.
	IsReferencedAsCover(ctx context.Context, versionID string) (bool, error)
	// GetByHash returns a version matching assetID + contentHash, or ErrNotFound.
	GetByHash(ctx context.Context, assetID, contentHash string) (AssetVersion, error)
	// NextVersionNum returns MAX(version_num)+1 for the asset (1 if no versions exist).
	NextVersionNum(ctx context.Context, assetID string) (int64, error)
	// SetCurrent atomically promotes versionID to current for assetID.
	SetCurrent(ctx context.Context, assetID, versionID string) error
	// RunInTx runs fn within a single DB transaction, giving fn a repository bound to that tx.
	RunInTx(ctx context.Context, fn func(tx VersionRepository) error) error
	// SetAssetThumbnail updates assets.thumbnail_key for the given asset.
	SetAssetThumbnail(ctx context.Context, assetID string, key *string) error
	// ListWithVariantCount returns versions with per-version variant counts.
	ListWithVariantCount(ctx context.Context, assetID string) ([]AssetVersionWithCount, error)
	// FindDuplicateVersions returns every version in the workspace sharing
	// contentHash, excluding excludeAssetID, INCLUDING soft-deleted versions,
	// ranked most-actionable-first (live/non-deleted, then most recent).
	FindDuplicateVersions(
		ctx context.Context,
		workspaceID, contentHash, excludeAssetID string,
	) ([]DuplicateVersionMatch, error)
}

// FieldRepository handles persistence for FieldDefinition records.
type FieldRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (FieldDefinition, error)
	List(ctx context.Context, workspaceID, scope string) ([]FieldDefinition, error)
	Create(ctx context.Context, f FieldDefinition) (FieldDefinition, error)
	Update(ctx context.Context, f FieldDefinition) (FieldDefinition, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
	CountByWorkspaceAndScope(ctx context.Context, workspaceID, scope string) (int64, error)
	CountAssetValues(ctx context.Context, fieldID string) (int64, error)
	CountProjectValues(ctx context.Context, fieldID string) (int64, error)
	UpdatePosition(ctx context.Context, workspaceID, id string, position int64) error
	// InheritProjectFields copies inheritable project field values to a newly created asset.
	// It is a no-op when there are no inheritable definitions or the project has no values set.
	InheritProjectFields(ctx context.Context, workspaceID, assetID, projectID, userID string) error
	// GetByKey returns the field definition with the given key in the workspace, or ErrNotFound.
	GetByKey(ctx context.Context, workspaceID, key string) (FieldDefinition, error)
	// ListImageAssetIDs returns IDs of all image assets in the workspace.
	ListImageAssetIDs(ctx context.Context, workspaceID string) ([]string, error)
	// ListMissingExifField returns asset IDs that are image assets but lack a value for fieldID.
	ListMissingExifField(ctx context.Context, workspaceID, fieldID string, limit int64) ([]string, error)
	// EnsureSystemField idempotently creates a system-sourced (non-user-editable) asset
	// field definition. A no-op if a definition with the same key already exists.
	EnsureSystemField(ctx context.Context, params EnsureSystemFieldParams) error
	// ListBySource returns non-deleted field definitions for a given system source
	// (e.g. "exif"), ordered by position.
	ListBySource(ctx context.Context, workspaceID, source string) ([]FieldDefinition, error)
	// PurgeExpired hard-deletes field definitions soft-deleted for more than 30 days,
	// including their asset and project field values. Returns the number of definitions deleted.
	PurgeExpired(ctx context.Context) (int, error)
}

// EnsureSystemFieldParams is the input for FieldRepository.EnsureSystemField.
type EnsureSystemFieldParams struct {
	ID          string
	WorkspaceID string
	Source      string
	Name        string
	Key         string
	FieldType   string
	Position    int64
}

// AssetFieldRepository handles persistence for asset field values.
type AssetFieldRepository interface {
	GetValues(ctx context.Context, assetID string) ([]FieldValue, error)
	DeleteValue(ctx context.Context, assetID, fieldID string) error
	UpsertValue(ctx context.Context, assetID string, p SetFieldValueParams) error
	RunInTx(ctx context.Context, fn func(tx AssetFieldRepository) error) error
}

// ProjectFieldRepository handles persistence for project field values.
type ProjectFieldRepository interface {
	GetValues(ctx context.Context, projectID string) ([]FieldValue, error)
	DeleteValue(ctx context.Context, projectID, fieldID string) error
	UpsertValue(ctx context.Context, projectID string, p SetFieldValueParams) error
}

// VariantRepository handles persistence for asset variant records.
type VariantRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Variant, error)
	ListByAsset(ctx context.Context, workspaceID, assetID string) ([]Variant, error)
	Create(ctx context.Context, v Variant) (Variant, error)
	Delete(ctx context.Context, workspaceID, id string) error
	UpdateTitle(ctx context.Context, workspaceID, variantID string, title *string) error
	UpdateSharedBatch(ctx context.Context, workspaceID string, ids []string, isShared bool) error
	ListSharedByAssetIDs(ctx context.Context, assetIDs []string) ([]VariantWithAssetID, error)
	GetSharedByVariantAndAsset(ctx context.Context, variantID, assetID string) (Variant, error)
	// ListVariantParamHistory returns up to `limit` non-empty transform_params JSON strings
	// for the given workspace + variant type, most recently created first. Rows are NOT
	// deduplicated here — raw transform_params may have unsorted JSON keys for some variant
	// types, so distinctness must be computed by the caller after canonicalizing each entry.
	ListVariantParamHistory(ctx context.Context, workspaceID, variantType string, limit int) ([]string, error)
}

// WorkspaceRepository handles persistence for Workspace records.
type WorkspaceRepository interface {
	GetByID(ctx context.Context, id string) (Workspace, error)
	Create(ctx context.Context, w Workspace) (Workspace, error)
	Update(ctx context.Context, w Workspace) (Workspace, error)
	UpdateLockedTaxonomy(ctx context.Context, workspaceID string, locked bool) (Workspace, error)
	UpdateAutoTagSettings(ctx context.Context, workspaceID string, enabled bool, mode string) (Workspace, error)
	UpdateDuplicateDetectionMode(ctx context.Context, workspaceID string, mode string) (Workspace, error)
	GetAIProviderKey(ctx context.Context, workspaceID, providerName string) (string, error)
	SetAIProviderKey(ctx context.Context, workspaceID, providerName, encKey string) error
	ClearAIProviderKey(ctx context.Context, workspaceID string, providerName string) error
	CountAssets(ctx context.Context, workspaceID string) (int64, error)
	// Member methods
	GetMember(ctx context.Context, workspaceID, userID string) (Member, error)
	ListMembers(ctx context.Context, workspaceID string) ([]Member, error)
	CountMembers(ctx context.Context, workspaceID string) (int64, error)
	CreateMember(ctx context.Context, m Member) error
	DeleteMember(ctx context.Context, workspaceID, userID string) error
	UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) error
	// Invite methods
	CreateInvite(ctx context.Context, inv Invite) (Invite, error)
	ListPendingInvites(ctx context.Context, workspaceID string) ([]Invite, error)
	GetInviteByToken(ctx context.Context, token string) (Invite, error)
	DeleteInvite(ctx context.Context, workspaceID, inviteID string) error
	AcceptInvite(ctx context.Context, inviteID string) error
	// Workspace list for user
	ListByUserID(ctx context.Context, userID string) ([]WorkspaceWithRole, error)
	// RunInTx executes fn inside a single database transaction.
	// The WorkspaceRepository passed to fn is scoped to that transaction.
	RunInTx(ctx context.Context, fn func(tx WorkspaceRepository) error) error
	// RunRegistrationTx executes fn with tx-scoped UserRepository and WorkspaceRepository
	// sharing the same underlying database transaction. Used only by UserService.Register.
	RunRegistrationTx(
		ctx context.Context,
		fn func(ctx context.Context, txUsers UserRepository, txWorkspaces WorkspaceRepository) error,
	) error
}

// UserRepository handles persistence for User records.
type UserRepository interface {
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	Create(ctx context.Context, u User) (User, error)
	Update(ctx context.Context, u User) (User, error)
	UpdateProfile(ctx context.Context, id, displayName string) (User, error)
	UpdateAvatarKey(ctx context.Context, id, storageKey string) (User, error)
	ClearAvatarKey(ctx context.Context, id string) (User, error)
	SetPassword(ctx context.Context, id, passwordHash string) error
	SetAuthMethods(ctx context.Context, id, authMethods string) (User, error)
	SetPendingEmail(ctx context.Context, id, email string) error
	ClearPendingEmail(ctx context.Context, id string) error
	ConfirmEmailChange(ctx context.Context, id, pendingEmail string) (User, error)
	SoftDelete(ctx context.Context, id string) error
	AnonymizeDeletedUser(ctx context.Context, id string) error
	HardDelete(ctx context.Context, id string) error
	// GetByGoogleID returns the user linked to this Google sub ID.
	GetByGoogleID(ctx context.Context, googleUserID string) (User, error)
	// GetByCanvaID returns the user linked to this Canva user ID.
	GetByCanvaID(ctx context.Context, canvaUserID string) (User, error)
	// GetByOIDC returns the user linked to this OIDC issuer+sub pair.
	GetByOIDC(ctx context.Context, issuer, sub string) (User, error)
	// CreateWithGoogle creates a new user with a Google identity in one query.
	CreateWithGoogle(ctx context.Context, u User) (User, error)
	// CreateWithOIDC creates a new user with an OIDC identity in one query.
	CreateWithOIDC(ctx context.Context, u User) (User, error)
	// CreateWithCanva creates a new user with a Canva identity in one query.
	CreateWithCanva(ctx context.Context, u User) (User, error)
	// LinkGoogle sets google_user_id + avatar_url + auth_methods on an existing user.
	LinkGoogle(ctx context.Context, u User) (User, error)
	// LinkOIDC sets oidc_issuer + oidc_sub + avatar_url + auth_methods on an existing user.
	LinkOIDC(ctx context.Context, u User) (User, error)
	// LinkCanva sets canva_user_id + avatar_url + auth_methods on an existing user.
	LinkCanva(ctx context.Context, u User) (User, error)
	// UnlinkGoogle clears google_user_id and updates auth_methods.
	UnlinkGoogle(ctx context.Context, u User) (User, error)
	// UnlinkOIDC clears oidc_sub/oidc_issuer and updates auth_methods.
	UnlinkOIDC(ctx context.Context, u User) (User, error)
	// UnlinkCanva clears canva_user_id and updates auth_methods.
	UnlinkCanva(ctx context.Context, u User) (User, error)
	// ListWorkspaceIDs returns the workspace IDs the user belongs to (ordered by join date).
	ListWorkspaceIDs(ctx context.Context, userID string) ([]string, error)
	// RunInTx executes fn inside a single database transaction.
	RunInTx(ctx context.Context, fn func(tx UserRepository) error) error
}

// OAuthConnectionRepository handles persistence for oauth_connections rows.
type OAuthConnectionRepository interface {
	List(ctx context.Context, workspaceID string) ([]OAuthConnection, error)
	GetByID(ctx context.Context, workspaceID, id string) (OAuthConnection, error)
	GetByProviderUserID(ctx context.Context, workspaceID, provider, providerUserID string) (OAuthConnection, error)
	Create(ctx context.Context, c OAuthConnection) error
	UpdateTokens(ctx context.Context, id, accessToken string, refreshToken *string, expiresAt *string) error
	UpdateTokensAndScopes(
		ctx context.Context,
		id, accessToken string,
		refreshToken *string,
		expiresAt *string,
		scopes string,
	) error
	Delete(ctx context.Context, workspaceID, id string) error
}

// WorkflowRepository handles persistence for workflow definitions.
type WorkflowRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (Workflow, error)
	List(ctx context.Context, workspaceID string) ([]Workflow, error)
	ListByTrigger(ctx context.Context, triggerType string) ([]Workflow, error)
	// ListEnabledByTrigger returns all enabled workflows in a workspace with the
	// given trigger_type, ordered by name ASC.
	ListEnabledByTrigger(ctx context.Context, workspaceID, triggerType string) ([]Workflow, error)
	Create(ctx context.Context, p CreateWorkflowParams) (Workflow, error)
	Update(ctx context.Context, p UpdateWorkflowParams) (Workflow, error)
	FindCoveringWorkflow(
		ctx context.Context,
		workspaceID, assetID, assetProjectID, assetFolderID string,
	) (*CoveringWorkflow, error)
	SetEnabled(ctx context.Context, workspaceID, id string, enabled bool) error
	Delete(ctx context.Context, workspaceID, id string) error
	TouchLastRunAt(ctx context.Context, id string) error
	RunInTx(ctx context.Context, fn func(WorkflowRepository) error) error
}

// WorkflowRunRepository handles persistence for workflow runs and steps.
type WorkflowRunRepository interface {
	GetByID(ctx context.Context, id string) (WorkflowRun, error)
	List(ctx context.Context, workspaceID, workflowID string, limit int, cursor string) ([]WorkflowRun, error)
	Create(ctx context.Context, p CreateWorkflowRunParams) (WorkflowRun, error)
	SetStatus(ctx context.Context, id, status string) error
	SetFinal(ctx context.Context, p SetWorkflowRunFinalParams) error
	ListSteps(ctx context.Context, runID string) ([]WorkflowRunStep, error)
	CreateStep(ctx context.Context, p CreateWorkflowRunStepParams) (WorkflowRunStep, error)
	SetStepStatus(ctx context.Context, id, status string) error
	SetStepFailed(ctx context.Context, id, errMsg string) error
	SetStepCompleted(ctx context.Context, id, outputCtx string) error
	IncrementStepAttempt(ctx context.Context, id string) error
}

// WorkflowWebhookRepository handles persistence for inbound webhook secrets.
type WorkflowWebhookRepository interface {
	GetTokenHash(ctx context.Context, workflowID string) (string, error)
	Upsert(ctx context.Context, workflowID, tokenHash string) error
	Delete(ctx context.Context, workflowID string) error
}

// ExportConfigRepository handles persistence for export_configs rows.
type ExportConfigRepository interface {
	Create(ctx context.Context, p ExportConfig) (ExportConfig, error)
	Get(ctx context.Context, workspaceID, id string) (ExportConfig, error)
	List(ctx context.Context, workspaceID string) ([]ExportConfig, error)
	ListByProject(ctx context.Context, workspaceID, projectID string) ([]ExportConfig, error)
	Update(ctx context.Context, p ExportConfig) (ExportConfig, error)
	Delete(ctx context.Context, workspaceID, id string) error
	SetLastRun(ctx context.Context, id string, p ExportRunResult) error
	ListDue(ctx context.Context) ([]ExportConfig, error)
}

// EmbedTokenRepository handles persistence for public asset embed tokens.
type EmbedTokenRepository interface {
	// Create inserts a new token row. Returns apperr.ErrConflict if an active
	// token already exists for this asset (unique index violation).
	Create(ctx context.Context, params CreateEmbedTokenParams) (EmbedToken, error)
	// GetByID loads a token by its 16-char id regardless of revocation status.
	// Returns apperr.ErrNotFound if id does not exist.
	GetByID(ctx context.Context, id string) (EmbedToken, error)
	// GetActiveByAssetID returns the single non-revoked token for an asset.
	// Returns apperr.ErrNotFound if none exists.
	GetActiveByAssetID(ctx context.Context, workspaceID, assetID string) (EmbedToken, error)
	// Revoke sets revoked_at = now() on the given token id, scoped to workspace.
	// Returns apperr.ErrNotFound if the token does not exist or is already revoked.
	Revoke(ctx context.Context, workspaceID, id string) error
}

// EmbedToken is the domain representation of a public asset embed token.
type EmbedToken struct {
	ID          string
	WorkspaceID string
	AssetID     string
	CreatedBy   string
	Label       string
	CreatedAt   time.Time
	RevokedAt   *time.Time
}

// CreateEmbedTokenParams is the input for EmbedTokenRepository.Create.
type CreateEmbedTokenParams struct {
	ID          string
	WorkspaceID string
	AssetID     string
	CreatedBy   string
	Label       string
}

// ExportRunRepository handles persistence for export_runs rows.
type ExportRunRepository interface {
	Create(ctx context.Context, p ExportRun) (ExportRun, error)
	Get(ctx context.Context, workspaceID, id string) (ExportRun, error)
	List(ctx context.Context, configID string, limit, offset int) ([]ExportRun, error)
	Start(ctx context.Context, id string) error
	UpdateProgress(ctx context.Context, id string, p ExportProgress) error
	Finish(ctx context.Context, id string, p ExportFinish) error
}

// IngressRepository handles persistence for ingress sources, rules, and log entries.
// Every method is workspace-scoped: rule and log-entry methods enforce this via a join
// through their parent source rather than trusting the caller to have already checked it.
type IngressRepository interface {
	ListSources(ctx context.Context, workspaceID string) ([]IngressSource, error)
	GetSource(ctx context.Context, workspaceID, id string) (IngressSource, error)
	CreateSource(ctx context.Context, params CreateIngressSourceParams) (IngressSource, error)
	UpdateSource(ctx context.Context, params UpdateIngressSourceParams) (IngressSource, error)
	DeleteSource(ctx context.Context, workspaceID, id string) error

	ListRules(ctx context.Context, workspaceID, sourceID string) ([]IngressRule, error)
	GetRule(ctx context.Context, workspaceID, id string) (IngressRule, error)
	CreateRule(ctx context.Context, params CreateIngressRuleParams) (IngressRule, error)
	UpdateRule(ctx context.Context, workspaceID string, params UpdateIngressRuleParams) (IngressRule, error)
	DeleteRule(ctx context.Context, workspaceID, id string) error

	ListSourceLog(ctx context.Context, workspaceID, sourceID string, limit, offset int64) ([]IngressLogEntry, error)
	ListWorkspaceLog(ctx context.Context, workspaceID, status string, limit, offset int64) ([]IngressLogEntry, error)
	GetLogEntry(ctx context.Context, workspaceID, id string) (IngressLogEntry, error)
	UpdateLogEntry(ctx context.Context, workspaceID string, params UpdateIngressLogEntryParams) error
	DeleteLogEntry(ctx context.Context, workspaceID, id string) error
}

// IngressSource is the domain representation of an ingress_sources row.
// Config remains the encrypted ciphertext blob; encryption/decryption stays in internal/ingress.
type IngressSource struct {
	ID              string
	WorkspaceID     string
	CreatedBy       string
	Type            string
	Label           string
	Config          string
	PublicToken     string
	DestFolderID    *string
	DestProjectID   *string
	Enabled         bool
	PollIntervalMin int64
	LastPolledAt    *time.Time
	LastError       *string
	ErrorCount      int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CreateIngressSourceParams is the input for IngressRepository.CreateSource.
type CreateIngressSourceParams struct {
	ID              string
	WorkspaceID     string
	CreatedBy       string
	Type            string
	Label           string
	Config          string
	PublicToken     string
	DestFolderID    *string
	DestProjectID   *string
	Enabled         bool
	PollIntervalMin int64
}

// UpdateIngressSourceParams is the input for IngressRepository.UpdateSource.
type UpdateIngressSourceParams struct {
	ID              string
	WorkspaceID     string
	Label           string
	Config          string
	DestFolderID    *string
	DestProjectID   *string
	Enabled         bool
	PollIntervalMin int64
}

// IngressRule is the domain representation of an ingress_rules row.
type IngressRule struct {
	ID       string
	SourceID string
	Position int64
	Field    string
	Operator string
	Value    string
	Action   string
}

// CreateIngressRuleParams is the input for IngressRepository.CreateRule.
type CreateIngressRuleParams struct {
	ID          string
	WorkspaceID string
	SourceID    string
	Position    int64
	Field       string
	Operator    string
	Value       string
	Action      string
}

// UpdateIngressRuleParams is the input for IngressRepository.UpdateRule.
type UpdateIngressRuleParams struct {
	ID       string
	Position int64
	Field    string
	Operator string
	Value    string
	Action   string
}

// IngressLogEntry is the domain representation of an ingress_log row.
type IngressLogEntry struct {
	ID         string
	SourceID   string
	RemoteID   string
	Filename   string
	AssetID    *string
	Status     string
	Error      *string
	ImportedAt time.Time
}

// UpdateIngressLogEntryParams is the input for IngressRepository.UpdateLogEntry.
type UpdateIngressLogEntryParams struct {
	ID      string
	Status  string
	AssetID *string
	Error   *string
}

// StorageStatsRepository handles the read-only aggregate queries backing workspace
// storage usage/limit reporting. Named "StorageStats" (not "Storage") to avoid
// colliding with internal/storage.Storage, the asset-bytes storage abstraction.
type StorageStatsRepository interface {
	GetByProjectAndType(ctx context.Context, workspaceID string) ([]StorageProjectTypeRow, error)
	GetFolderCountsByProject(ctx context.Context, workspaceID string) ([]StorageFolderCountRow, error)
	GetStorageLimitBytes(ctx context.Context, workspaceID string) (*int64, error)
	GetByFolder(ctx context.Context, workspaceID, projectID string) ([]StorageFolderRow, error)
}

// StorageProjectTypeRow is one (project, asset_type) storage aggregate row.
type StorageProjectTypeRow struct {
	ProjectID     *string
	ProjectName   string
	AssetType     string
	VersionsBytes int64
	VariantsBytes int64
}

// StorageFolderCountRow is the folder count for one project.
type StorageFolderCountRow struct {
	ProjectID   *string
	FolderCount int64
}

// StorageFolderRow is one folder's storage aggregate row.
type StorageFolderRow struct {
	FolderID      *string
	FolderName    string
	VersionsBytes int64
	VariantsBytes int64
}

// AutoTagSuggestionRepository handles persistence for pending AI tag suggestions.
type AutoTagSuggestionRepository interface {
	List(ctx context.Context, workspaceID, assetID string) ([]AutoTagSuggestion, error)
	Get(ctx context.Context, workspaceID, id string) (AutoTagSuggestion, error)
	Create(ctx context.Context, params CreateAutoTagSuggestionParams) (AutoTagSuggestion, error)
	Delete(ctx context.Context, workspaceID, id string) error
	DeleteByAsset(ctx context.Context, workspaceID, assetID string) error
}

// AutoTagSuggestion is the domain representation of an auto_tag_suggestions row.
type AutoTagSuggestion struct {
	ID             string
	WorkspaceID    string
	AssetID        string
	AssetVersionID *string
	TagName        string
	CreatedAt      time.Time
}

// CreateAutoTagSuggestionParams is the input for AutoTagSuggestionRepository.Create.
type CreateAutoTagSuggestionParams struct {
	ID             string
	WorkspaceID    string
	AssetID        string
	AssetVersionID *string
	TagName        string
}

// TextTrackRepository handles persistence for asset text tracks (OCR, transcripts,
// extracted document text, etc.) and their full-text search shadow rows.
type TextTrackRepository interface {
	List(ctx context.Context, workspaceID, assetID string) ([]TextTrack, error)
	Get(ctx context.Context, workspaceID, id string) (TextTrack, error)
	Create(ctx context.Context, params CreateTextTrackParams) (TextTrack, error)
	SetReady(ctx context.Context, workspaceID, id string, params SetTextTrackReadyParams) error
	SetFailed(ctx context.Context, workspaceID, id, errMsg string) error
	Delete(ctx context.Context, workspaceID, id string) error
	// InsertFTS indexes a track's content for full-text search. trackID must already
	// belong to workspaceID (callers verify via Get/Create before indexing).
	InsertFTS(ctx context.Context, params InsertTextFTSParams) error
	// DeleteFTS removes the FTS shadow row for a track (no-op if absent).
	DeleteFTS(ctx context.Context, trackID string) error
}

// TextTrack is the domain representation of an asset_text_tracks row.
type TextTrack struct {
	ID             string
	WorkspaceID    string
	AssetID        string
	AssetVersionID *string
	Source         string
	Lang           *string
	Content        string
	StorageKey     *string
	ContentType    *string
	Meta           *string
	Status         string
	Error          *string
	CreatedBy      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateTextTrackParams is the input for TextTrackRepository.Create.
type CreateTextTrackParams struct {
	ID             string
	WorkspaceID    string
	AssetID        string
	AssetVersionID *string
	Source         string
	Lang           *string
	Content        string
	Meta           *string
	Status         string
	CreatedBy      *string
}

// SetTextTrackReadyParams is the input for TextTrackRepository.SetReady.
type SetTextTrackReadyParams struct {
	Content     string
	StorageKey  *string
	ContentType *string
	Meta        *string
}

// InsertTextFTSParams is the input for TextTrackRepository.InsertFTS.
type InsertTextFTSParams struct {
	TrackID     string
	AssetID     string
	WorkspaceID string
	Source      string
	Lang        string
	Content     string
}

// AuditLogRepository handles read access to asset_events/project_events for the
// audit trail and workspace activity feed. Writes go through audit.Writer, which
// stays a separate fire-and-forget path per docs/backend-architecture.md.
type AuditLogRepository interface {
	// ListAssetEvents returns events for a single asset, newest first.
	ListAssetEvents(ctx context.Context, params ListAuditEventsParams) ([]AuditEvent, error)
	// ListProjectEvents returns events for a single project, newest first.
	ListProjectEvents(ctx context.Context, params ListAuditEventsParams) ([]AuditEvent, error)
	// ListWorkspaceAssetEvents returns asset events across the workspace, newest first.
	ListWorkspaceAssetEvents(ctx context.Context, params ListAuditEventsParams) ([]AuditEvent, error)
	// ListWorkspaceProjectEvents returns project events across the workspace, newest first.
	ListWorkspaceProjectEvents(ctx context.Context, params ListAuditEventsParams) ([]AuditEvent, error)
}

// AuditEvent is the domain representation of a single asset_events/project_events row.
type AuditEvent struct {
	ID        string
	EntityID  string // asset_id or project_id, depending on the method that returned it
	EventType string
	Payload   string
	CreatedAt string // stored/returned as text; compared lexically for cursor pagination
	UserID    *string
	UserName  *string
	ActorType string
}

// ListAuditEventsParams is the shared input for AuditLogRepository's List* methods.
// Not every field applies to every method: EntityID is required by ListAssetEvents/
// ListProjectEvents and ignored by the workspace-wide variants; UserID only applies
// to the workspace-wide variants.
type ListAuditEventsParams struct {
	WorkspaceID string
	EntityID    string
	Cursor      string // "" = no cursor
	UserID      string // "" = no filter
	EventType   string // "" = no filter
	Limit       int64
}
