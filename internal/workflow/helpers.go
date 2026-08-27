package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/events"
	"damask/server/internal/jobspec"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"

	"github.com/google/uuid"
)

var tracer = telemetry.Tracer("damask/internal/workflow")

type Deps struct {
	Workflows   repository.WorkflowRepository
	Runs        repository.WorkflowRunRepository
	Queue       queue.JobQueue
	Storage     storage.Storage
	Mailer      mail.Mailer
	Hub         events.EventHub
	Audit       audit.Writer
	Assets      AssetManager
	Variants    VariantManager
	Versions    VersionManager
	Shares      ShareManager
	Tags        TagManager
	AssetFields AssetFieldManager
	Workspace   WorkspaceManager
	TextTracks  TextTrackManager
	AutoTag     AutoTagManager
	Config      *config.Config
}

type Asset struct {
	ID               string
	WorkspaceID      string
	MimeType         string
	CurrentVersionID *string
	FolderID         *string
	ProjectID        *string
}

type AssetMoveParams struct {
	FolderID  *string
	ProjectID *string
}

type AssetManager interface {
	Get(ctx context.Context, workspaceID, assetID string) (*Asset, error)
	Move(ctx context.Context, workspaceID, assetID string, p AssetMoveParams) (*Asset, error)
}

type VariantPrepareRequest struct {
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

type VariantPrepareResult struct {
	Type     string
	Params   json.RawMessage
	Title    *string
	IsShared bool
}

type VariantJobPayload struct {
	AssetID      string            `json:"asset_id"`
	WorkspaceID  string            `json:"workspace_id"`
	VersionID    string            `json:"version_id"`
	VersionNum   int64             `json:"version_num"`
	VariantID    string            `json:"variant_id,omitempty"`
	StorageKey   string            `json:"storage_key"`
	MimeType     string            `json:"mime_type"`
	Type         string            `json:"type"`
	Params       json.RawMessage   `json:"params"`
	Title        *string           `json:"title,omitempty"`
	IsShared     bool              `json:"is_shared,omitempty"`
	Continuation *NodeContinuation `json:"continuation,omitempty"`
}

type VariantManager interface {
	PrepareCreate(ctx context.Context, p VariantPrepareRequest) (VariantPrepareResult, error)
	GetVariantByID(ctx context.Context, workspaceID, id string) (repository.Variant, error)
}

type VersionManager interface {
	GetByID(ctx context.Context, id string) (repository.AssetVersion, error)
	NextVersionNum(ctx context.Context, assetID string) (int64, error)
	Create(ctx context.Context, v repository.AssetVersion) (repository.AssetVersion, error)
	SetCurrent(ctx context.Context, assetID, versionID string) error
	SetAssetThumbnail(ctx context.Context, assetID string, key *string) error
}

// NodeContinuation carries the data needed to resume a workflow run
// after an async job completes.
// NodeContinuation is an alias for [jobspec.NodeContinuation]: it lives in
// jobspec so job payload structs there can embed it without importing
// internal/workflow (which itself imports internal/queue).
type NodeContinuation = jobspec.NodeContinuation

type ShareCreateParams struct {
	CreatedBy     string
	Label         string
	TargetType    string
	TargetID      string
	ExpiresInDays *int
	AllowComments bool
	AllowDownload bool
}

type ShareManager interface {
	Create(ctx context.Context, workspaceID string, p ShareCreateParams) (string, error)
}

type TagManager interface {
	AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (string, error)
}

type FieldValueInput struct {
	FieldID string
	Value   any
}

type AssetFieldManager interface {
	SetValues(ctx context.Context, workspaceID, assetID, userID string, inputs []FieldValueInput) error
}

// AIProviderStatus is a capability-filtered, minimal view of one AI provider's
// configuration state — workflow nodes never need provider model lists.
type AIProviderStatus struct {
	ID         ai.ProviderID
	Configured bool
}

type WorkspaceManager interface {
	// ListAIProviders returns only the providers that declare at least one of
	// the requested capabilities, with their current key-configured status.
	ListAIProviders(
		ctx context.Context,
		workspaceID string,
		capabilities ai.Capability,
	) ([]AIProviderStatus, error)
}

// providerConfigured reports whether the given provider ID is present and
// configured in the list.
func providerConfigured(providers []AIProviderStatus, id ai.ProviderID) bool {
	for _, p := range providers {
		if p.ID == id && p.Configured {
			return true
		}
	}
	return false
}

// anyProviderConfigured reports whether any provider in the list is configured.
func anyProviderConfigured(providers []AIProviderStatus) bool {
	for _, p := range providers {
		if p.Configured {
			return true
		}
	}
	return false
}

// TextTrackCreateParams carries everything needed to enqueue an AI image
// description job. Continuation, when set, is embedded in the job payload so
// the job worker can resume the suspended workflow run once the description
// is ready.
type TextTrackCreateParams struct {
	AssetID      string
	StorageKey   string
	MimeType     string
	Model        string
	Prompt       string
	Lang         string
	Continuation *NodeContinuation
}

// TextTrackCreateOCRParams carries everything needed to enqueue an OCR job.
// Continuation, when set, is embedded in the job payload so the job worker
// can resume the suspended workflow run once the OCR text is ready.
type TextTrackCreateOCRParams struct {
	AssetID      string
	StorageKey   string
	MimeType     string
	Lang         string
	OutputFormat string
	Continuation *NodeContinuation
}

type TextTrackManager interface {
	CreateAIImageDescription(
		ctx context.Context,
		workspaceID string,
		p TextTrackCreateParams,
	) (trackID string, err error)
	CreateOCR(
		ctx context.Context,
		workspaceID string,
		p TextTrackCreateOCRParams,
	) (trackID string, err error)
}

// AutoTagManager lets the action.auto_tag workflow node enqueue AI tagging
// for an asset. Continuation, when set, is embedded in the job payload so
// the job worker can resume the suspended workflow run once tagging
// finishes.
type AutoTagManager interface {
	Enqueue(
		ctx context.Context,
		workspaceID, assetID, mode string,
		continuation *NodeContinuation,
	) error
}

type RunWorkflowPayload struct {
	RunID string `json:"run_id"`
}

func NewToken() (string, error) { return newToken() }

func Sha256Hex(raw string) string { return sha256Hex(raw) }

func newID() string { return uuid.NewString() }

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func jsonToMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	id := uuid.NewString() + uuid.NewString()
	if id == "" {
		return "", errors.New("failed to generate token")
	}
	return id, nil
}

func actorUserID(ctx context.Context, rc *RunContext) string {
	if userID, ok := rcGetString(rc, "workflow_created_by"); ok && userID != "" {
		return userID
	}
	if actorUserID := auth.ActorFromCtx(ctx).UserID; actorUserID != nil {
		return *actorUserID
	}
	return ""
}

func nowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}

func ptr(s string) *string { return &s }

func rcGetString(rc *RunContext, key string) (string, bool) {
	val, ok := rc.Get(key)
	if !ok || val == nil {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func rcRequireString(rc *RunContext, key string) (string, error) {
	val, ok := rcGetString(rc, key)
	if !ok || strings.TrimSpace(val) == "" {
		return "", fmt.Errorf("%s is required in workflow context: %w", key, apperr.ErrInvalidInput)
	}
	return val, nil
}

// rcGetVersionNum reads an integer "version_num" from the run context,
// tolerating both float64 (from JSON-decoded contexts) and int64. Returns 0
// if absent or of an unexpected type.
func rcGetVersionNum(rc *RunContext) int64 {
	v, ok := rc.Get("version_num")
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	default:
		return 0
	}
}

func retryPolicyFromConfig(_ json.RawMessage) RetryPolicy {
	return DefaultRetryPolicy()
}
