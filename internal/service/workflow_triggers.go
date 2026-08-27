package service

import (
	"context"
	"log/slog"
	"maps"
)

// workflowAssetTrigger holds the standard asset context passed to workflow trigger events.
// Filename is the display name consumed by workflow filter nodes; it defaults to OriginalFilename
// when empty. OriginalFilename is always the raw DB value. Both keys are emitted in toMap() because
// nodes_builtin.go reads "filename" first and falls back to "original_filename".
type workflowAssetTrigger struct {
	AssetID          string
	WorkspaceID      string
	ProjectID        string
	FolderID         string
	MimeType         string
	Size             int64
	OriginalFilename string
	Filename         string // display name for workflow nodes; defaults to OriginalFilename when empty
	VersionID        string
	VersionNum       int64
	StorageKey       string
}

func (t workflowAssetTrigger) toMap() map[string]any {
	filename := t.Filename
	if filename == "" {
		filename = t.OriginalFilename
	}
	return map[string]any{
		keyAssetID:          t.AssetID,
		keyWorkspaceID:      t.WorkspaceID,
		keyProjectID:        t.ProjectID,
		keyFolderID:         t.FolderID,
		keyMimeType:         t.MimeType,
		keySize:             t.Size,
		keyOriginalFilename: t.OriginalFilename,
		keyFilename:         filename,
		keyVersionID:        t.VersionID,
		keyVersionNum:       t.VersionNum,
		keyStorageKey:       t.StorageKey,
	}
}

type nopWorkflowTriggerPublisher struct{}

func (nopWorkflowTriggerPublisher) Dispatch(context.Context, string, map[string]any) error {
	return nil
}

func workflowTriggerPublisherOrNop(publishers ...WorkflowTriggerPublisher) WorkflowTriggerPublisher {
	if len(publishers) > 0 && publishers[0] != nil {
		return publishers[0]
	}
	return nopWorkflowTriggerPublisher{}
}

func publishWorkflowTriggerAsync(
	ctx context.Context,
	publisher WorkflowTriggerPublisher,
	eventType string,
	data map[string]any,
) {
	if _, ok := publisher.(nopWorkflowTriggerPublisher); ok {
		return
	}

	payload := make(map[string]any, len(data))
	maps.Copy(payload, data)

	bgCtx := context.WithoutCancel(ctx)
	go func() {
		if err := publisher.Dispatch(bgCtx, eventType, payload); err != nil {
			slog.WarnContext(bgCtx, "workflow trigger dispatch failed", "trigger_type", eventType, "error", err)
		}
	}()
}
