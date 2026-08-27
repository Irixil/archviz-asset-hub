package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/transform"
	"damask/server/internal/workflow"
)

// autoTagPayload mirrors jobs.AutoTagPayload's JSON shape. Kept local to this
// package (rather than importing internal/jobs) following the same
// convention used by other job-enqueuing services (see stack.go).
type autoTagPayload struct {
	WorkspaceID          string `json:"workspace_id"`
	AssetID              string `json:"asset_id"`
	AssetVersionID       string `json:"asset_version_id"`
	StorageKey           string `json:"storage_key"`
	ThumbnailKey         string `json:"thumbnail_key"`
	ThumbnailContentType string `json:"thumbnail_content_type"`
	MimeType             string `json:"mime_type"`
	Mode                 string `json:"mode"`
	Size                 int64  `json:"size"`
	// Continuation, when set, resumes a suspended workflow run once tagging
	// finishes (see action.auto_tag workflow node). Mirrors
	// jobs.AutoTagPayload.Continuation exactly — same JSON key.
	Continuation *workflow.NodeContinuation `json:"continuation,omitempty"`
}

const (
	autoTagModePending = "pending"
	autoTagModeSilent  = "silent"
)

type autoTagService struct {
	assets           repository.AssetRepository
	workspaces       repository.WorkspaceRepository
	suggestions      repository.AutoTagSuggestionRepository
	queue            queue.JobQueue
	tags             TagService
	aiAPIKeyResolver ai.KeyResolver
}

// NewAutoTagService returns an AutoTagService.
func NewAutoTagService(
	assets repository.AssetRepository,
	workspaces repository.WorkspaceRepository,
	suggestions repository.AutoTagSuggestionRepository,
	q queue.JobQueue,
	tags TagService,
	aiAPIKeyResolver ai.KeyResolver,
) AutoTagService {
	return &autoTagService{
		assets:           assets,
		workspaces:       workspaces,
		suggestions:      suggestions,
		queue:            q,
		tags:             tags,
		aiAPIKeyResolver: aiAPIKeyResolver,
	}
}

func (s *autoTagService) Enqueue(ctx context.Context, workspaceID, assetID string, manual bool) error {
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if !transform.IsAutoTaggable(asset.MimeType) {
		return nil
	}

	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !manual && !ws.AutoTagEnabled {
		return nil
	}

	var versionID string
	if asset.CurrentVersionID != nil {
		versionID = *asset.CurrentVersionID
	}
	var thumbKey string
	if asset.ThumbnailKey != nil {
		thumbKey = *asset.ThumbnailKey
	}

	payload, err := json.Marshal(autoTagPayload{
		WorkspaceID:          workspaceID,
		AssetID:              assetID,
		AssetVersionID:       versionID,
		StorageKey:           asset.StorageKey,
		ThumbnailKey:         thumbKey,
		ThumbnailContentType: asset.ThumbnailContentType,
		MimeType:             asset.MimeType,
		Mode:                 ws.AutoTagMode,
		Size:                 asset.Size,
	})
	if err != nil {
		return fmt.Errorf("auto_tag: marshal payload: %w", err)
	}
	_, err = s.queue.Enqueue(ctx, workspaceID, queue.JobTypeAutoTag, string(payload))
	return err
}

func (s *autoTagService) EnqueueForWorkflow(
	ctx context.Context,
	workspaceID, assetID, mode string,
	continuation *workflow.NodeContinuation,
) error {
	if mode != autoTagModePending && mode != autoTagModeSilent {
		return fmt.Errorf("auto_tag: unsupported mode %q: %w", mode, apperr.ErrInvalidInput)
	}

	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if !transform.IsAutoTaggable(asset.MimeType) {
		return fmt.Errorf(
			"auto_tag: asset mime type %q is not eligible for auto-tagging: %w",
			asset.MimeType, apperr.ErrInvalidInput,
		)
	}

	var versionID string
	if asset.CurrentVersionID != nil {
		versionID = *asset.CurrentVersionID
	}
	var thumbKey string
	if asset.ThumbnailKey != nil {
		thumbKey = *asset.ThumbnailKey
	}

	payload, err := json.Marshal(autoTagPayload{
		WorkspaceID:          workspaceID,
		AssetID:              assetID,
		AssetVersionID:       versionID,
		StorageKey:           asset.StorageKey,
		ThumbnailKey:         thumbKey,
		ThumbnailContentType: asset.ThumbnailContentType,
		MimeType:             asset.MimeType,
		Mode:                 mode,
		Size:                 asset.Size,
		Continuation:         continuation,
	})
	if err != nil {
		return fmt.Errorf("auto_tag: marshal payload: %w", err)
	}
	_, err = s.queue.Enqueue(ctx, workspaceID, queue.JobTypeAutoTag, string(payload))
	return err
}

func (s *autoTagService) IsProviderAvailable(ctx context.Context, workspaceID, mimeType string) bool {
	if !transform.IsAutoTaggable(mimeType) {
		return false
	}
	for _, providerID := range []ai.ProviderID{ai.ProviderOpenRouter, ai.ProviderImageRouter} {
		key, source, err := s.aiAPIKeyResolver(ctx, workspaceID, string(providerID))
		if err != nil || key == "" {
			continue
		}
		provider, err := ai.NewProvider(providerID, key, source)
		if err != nil {
			continue
		}
		if provider.Capabilities()&ai.CapVisionTag != 0 {
			return true
		}
	}
	return false
}

func (s *autoTagService) ListSuggestions(
	ctx context.Context,
	workspaceID, assetID string,
) ([]AutoTagSuggestionDTO, error) {
	rows, err := s.suggestions.List(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]AutoTagSuggestionDTO, len(rows))
	for i, r := range rows {
		out[i] = AutoTagSuggestionDTO{ID: r.ID, AssetID: r.AssetID, TagName: r.TagName, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

func (s *autoTagService) AcceptSuggestion(
	ctx context.Context,
	workspaceID, assetID, suggestionID string,
) (*TagDTO, error) {
	sug, err := s.suggestions.Get(ctx, workspaceID, suggestionID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, fmt.Errorf("suggestion %q: %w", suggestionID, apperr.ErrNotFound)
		}
		return nil, err
	}
	if sug.AssetID != assetID {
		return nil, fmt.Errorf("suggestion %q: %w", suggestionID, apperr.ErrNotFound)
	}
	tag, err := s.tags.AddToAsset(ctx, workspaceID, sug.AssetID, sug.TagName)
	if err != nil {
		return nil, err
	}
	if err = s.suggestions.Delete(ctx, workspaceID, suggestionID); err != nil {
		return nil, err
	}
	return tag, nil
}

func (s *autoTagService) AcceptAll(ctx context.Context, workspaceID, assetID string) (int, error) {
	sugs, err := s.suggestions.List(ctx, workspaceID, assetID)
	if err != nil {
		return 0, err
	}
	accepted := 0
	failed := 0
	for _, sug := range sugs {
		if _, err = s.tags.AddToAsset(ctx, workspaceID, sug.AssetID, sug.TagName); err != nil {
			slog.WarnContext(ctx, "auto_tag: accept-all add tag failed", "tag", sug.TagName, "error", err)
			failed++
			continue
		}
		if err = s.suggestions.Delete(ctx, workspaceID, sug.ID); err != nil {
			slog.WarnContext(ctx, "auto_tag: accept-all delete suggestion failed", "id", sug.ID, "error", err)
			failed++
			continue
		}
		accepted++
	}
	if failed > 0 && accepted == 0 {
		return 0, fmt.Errorf("accept all: all %d suggestions failed", failed)
	}
	return accepted, nil
}

func (s *autoTagService) DismissSuggestion(ctx context.Context, workspaceID, assetID, suggestionID string) error {
	sug, err := s.suggestions.Get(ctx, workspaceID, suggestionID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return fmt.Errorf("suggestion %q: %w", suggestionID, apperr.ErrNotFound)
		}
		return err
	}
	if sug.AssetID != assetID {
		return fmt.Errorf("suggestion %q: %w", suggestionID, apperr.ErrNotFound)
	}
	return s.suggestions.Delete(ctx, workspaceID, suggestionID)
}

func (s *autoTagService) DismissAll(ctx context.Context, workspaceID, assetID string) error {
	return s.suggestions.DeleteByAsset(ctx, workspaceID, assetID)
}
