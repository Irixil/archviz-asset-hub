// Package events provides SSE event broadcasting.
package events

import (
	"encoding/json"
	"log/slog"
)

// Event is an envelope sent to connected clients. Payload is the type-specific
// body, shaped by the constructors below — producers must not build Event
// literals directly.
type Event struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	WorkspaceID string          `json:"-"` // routing only, never serialized
	Payload     json.RawMessage `json:"payload,omitempty"`
}

// Event type strings. Kept stable across the payload restructuring so
// existing frontend type switches don't need to change.
const (
	TypeThumbnailReady        = "thumbnail_ready"
	TypeVariantReady          = "variant_ready"
	TypeVariantFailed         = "variant_failed"
	TypeStackMergeDone        = "stack_merge_done"
	TypeVariantDraftReady     = "variant_draft.ready"
	TypeVariantDraftError     = "variant_draft.error"
	TypeWorkflowRunFailed     = "workflow_run_failed"
	TypeWorkflowRunStepUpdate = "workflow_run_step_updated"
	// TypeResync tells the client its Last-Event-ID has fallen out of the
	// replay buffer; it must refetch state instead of trusting the stream.
	TypeResync = "resync"
)

// ThumbnailReadyPayload is emitted after an asset's thumbnail finishes generating.
type ThumbnailReadyPayload struct {
	AssetID      string `json:"asset_id"`
	ThumbnailKey string `json:"thumbnail_key,omitempty"`
}

// VariantReadyPayload is emitted after a variant job completes successfully.
type VariantReadyPayload struct {
	AssetID   string `json:"asset_id"`
	VariantID string `json:"variant_id"`
}

// VariantFailedPayload is emitted after a variant job fails.
type VariantFailedPayload struct {
	AssetID string `json:"asset_id"`
	JobID   string `json:"job_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// StackMergeDonePayload is emitted after a stack GIF/PDF merge job completes.
type StackMergeDonePayload struct {
	AssetID string `json:"asset_id"`
	JobID   string `json:"job_id,omitempty"`
}

// VariantDraftReadyPayload is emitted after a variant draft preview finishes rendering.
type VariantDraftReadyPayload struct {
	Nonce      string `json:"nonce"`
	AssetID    string `json:"asset_id"`
	PreviewURL string `json:"preview_url,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

// VariantDraftErrorPayload is emitted when a variant draft preview fails to render.
type VariantDraftErrorPayload struct {
	Nonce string `json:"nonce"`
	Error string `json:"error,omitempty"`
}

// WorkflowRunFailedPayload is emitted when a workflow run fails.
type WorkflowRunFailedPayload struct {
	AssetID    string `json:"asset_id,omitempty"`
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	Error      string `json:"error,omitempty"`
}

// WorkflowRunStepUpdatedPayload is emitted when a workflow run step changes status.
type WorkflowRunStepUpdatedPayload struct {
	AssetID    string `json:"asset_id,omitempty"`
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	NodeID     string `json:"node_id"`
	Status     string `json:"status,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ResyncPayload tells the client which event type triggered the gap, if known.
type ResyncPayload struct {
	Reason string `json:"reason,omitempty"`
}

func newEvent(typ string, payload any) Event {
	b, err := json.Marshal(payload)
	if err != nil {
		// Payload types are static structs of strings today, so this should
		// never fire; if a future payload field breaks marshaling, degrade
		// to an empty payload rather than taking down this hot path.
		slog.Error("events: marshal payload failed", "type", typ, "error", err)
		b = json.RawMessage(`{}`)
	}
	return Event{Type: typ, Payload: b}
}

// ThumbnailReady builds a thumbnail_ready event.
func ThumbnailReady(assetID, thumbnailKey string) Event {
	return newEvent(TypeThumbnailReady, ThumbnailReadyPayload{AssetID: assetID, ThumbnailKey: thumbnailKey})
}

// VariantReady builds a variant_ready event.
func VariantReady(assetID, variantID string) Event {
	return newEvent(TypeVariantReady, VariantReadyPayload{AssetID: assetID, VariantID: variantID})
}

// VariantFailed builds a variant_failed event.
func VariantFailed(assetID, jobID, errMsg string) Event {
	return newEvent(TypeVariantFailed, VariantFailedPayload{AssetID: assetID, JobID: jobID, Error: errMsg})
}

// StackMergeDone builds a stack_merge_done event.
func StackMergeDone(assetID, jobID string) Event {
	return newEvent(TypeStackMergeDone, StackMergeDonePayload{AssetID: assetID, JobID: jobID})
}

// VariantDraftReady builds a variant_draft.ready event.
func VariantDraftReady(nonce, assetID, previewURL, expiresAt string) Event {
	return newEvent(TypeVariantDraftReady, VariantDraftReadyPayload{
		Nonce:      nonce,
		AssetID:    assetID,
		PreviewURL: previewURL,
		ExpiresAt:  expiresAt,
	})
}

// VariantDraftError builds a variant_draft.error event.
func VariantDraftError(nonce, errMsg string) Event {
	return newEvent(TypeVariantDraftError, VariantDraftErrorPayload{Nonce: nonce, Error: errMsg})
}

// WorkflowRunFailed builds a workflow_run_failed event.
func WorkflowRunFailed(assetID, workflowID, runID, errMsg string) Event {
	return newEvent(TypeWorkflowRunFailed, WorkflowRunFailedPayload{
		AssetID:    assetID,
		WorkflowID: workflowID,
		RunID:      runID,
		Error:      errMsg,
	})
}

// WorkflowRunStepUpdated builds a workflow_run_step_updated event.
func WorkflowRunStepUpdated(assetID, workflowID, runID, nodeID, status, errMsg string) Event {
	return newEvent(TypeWorkflowRunStepUpdate, WorkflowRunStepUpdatedPayload{
		AssetID:    assetID,
		WorkflowID: workflowID,
		RunID:      runID,
		NodeID:     nodeID,
		Status:     status,
		Error:      errMsg,
	})
}

// Resync builds a resync event telling the client its Last-Event-ID has
// fallen out of the replay buffer and it must refetch state.
func Resync(reason string) Event {
	return newEvent(TypeResync, ResyncPayload{Reason: reason})
}
