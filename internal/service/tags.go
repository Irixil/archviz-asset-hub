package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/repository"
	"damask/server/internal/systemtags"
	apptelemetry "damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

var ErrSystemTagProtected = errors.New("system tags cannot be deleted")

// TagDTO is the output of TagService methods.
type TagDTO struct {
	ID          string
	WorkspaceID string
	Name        string
	Color       *string
	GroupName   *string
	AssetCount  int64
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

// CreateTagParams is the input for TagService.Create.
type CreateTagParams struct {
	Name      string
	Color     *string
	GroupName *string
}

func (p *CreateTagParams) Validate() error {
	p.Name = strings.ToLower(strings.TrimSpace(p.Name))
	if p.Name == "" {
		return fmt.Errorf("name is required: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// PatchTagParams is the input for TagService.Patch.
// Nil fields mean "keep existing value".
type PatchTagParams struct {
	Name      *string
	Color     *string
	GroupName *string
}

func (p *PatchTagParams) Validate() error {
	if p.Name != nil {
		*p.Name = strings.ToLower(strings.TrimSpace(*p.Name))
		if *p.Name == "" {
			return fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}

type tagService struct {
	tags     repository.TagRepository
	assets   repository.AssetRepository
	audit    audit.Writer
	triggers WorkflowTriggerPublisher
}

type TagServiceDeps struct {
	Assets   repository.AssetRepository
	Triggers WorkflowTriggerPublisher
}

// NewTagService returns a TagService.
func NewTagService(tags repository.TagRepository, aw audit.Writer, deps ...TagServiceDeps) TagService {
	cfg := TagServiceDeps{}
	if len(deps) > 0 {
		cfg = deps[0]
	}
	return &tagService{
		tags:     tags,
		assets:   cfg.Assets,
		audit:    aw,
		triggers: workflowTriggerPublisherOrNop(cfg.Triggers),
	}
}

func (s *tagService) GetByName(ctx context.Context, workspaceID, name string) (*TagDTO, error) {
	tag, err := s.tags.GetByName(ctx, workspaceID, strings.ToLower(strings.TrimSpace(name)))
	if err != nil {
		return nil, err
	}
	return toTagDTO(tag), nil
}

func (s *tagService) List(ctx context.Context, workspaceID string, includeSystem bool) ([]*TagDTO, error) {
	rows, err := s.tags.List(ctx, workspaceID, includeSystem)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toTagDTO)
	return out, nil
}

func (s *tagService) EnsureSystemTag(ctx context.Context, workspaceID, name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if !systemtags.IsSystem(name) {
		return fmt.Errorf("unknown system tag %q: %w", name, apperr.ErrInvalidInput)
	}
	return s.tags.EnsureSystemTag(ctx, workspaceID, name)
}

func (s *tagService) Create(ctx context.Context, workspaceID string, p CreateTagParams) (*TagDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	_, err := s.tags.GetByName(ctx, workspaceID, p.Name)
	if err == nil {
		return nil, fmt.Errorf("tag %q already exists: %w", p.Name, apperr.ErrConflict)
	}
	if !isNotFound(err) {
		return nil, err
	}
	tag, err := s.tags.Upsert(ctx, workspaceID, p.Name)
	if err != nil {
		return nil, err
	}
	if p.Color != nil || p.GroupName != nil {
		if metaErr := s.tags.UpdateMetadata(ctx, workspaceID, tag.Name, p.Color, p.GroupName); metaErr != nil {
			return nil, metaErr
		}
		tag.Color = p.Color
		tag.GroupName = p.GroupName
	}
	return toTagDTO(tag), nil
}

func (s *tagService) Patch(ctx context.Context, workspaceID, currentName string, p PatchTagParams) (*TagDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	currentName = strings.ToLower(strings.TrimSpace(currentName))
	existing, err := s.tags.GetByName(ctx, workspaceID, currentName)
	if err != nil {
		return nil, err
	}

	finalName := existing.Name
	if p.Name != nil && *p.Name != existing.Name {
		if existing.GroupName != nil && *existing.GroupName == systemtags.GroupName {
			return nil, ErrSystemTagProtected
		}
		_, checkErr := s.tags.GetByName(ctx, workspaceID, *p.Name)
		if checkErr == nil {
			return nil, fmt.Errorf("tag %q already exists: %w", *p.Name, apperr.ErrConflict)
		}
		if !isNotFound(checkErr) {
			return nil, checkErr
		}
		if renameErr := s.tags.Rename(ctx, workspaceID, existing.Name, *p.Name); renameErr != nil {
			return nil, renameErr
		}
		finalName = *p.Name
	}

	if p.Color != nil || p.GroupName != nil {
		reloaded, reloadErr := s.tags.GetByName(ctx, workspaceID, finalName)
		if reloadErr != nil {
			return nil, reloadErr
		}
		newColor := reloaded.Color
		if p.Color != nil {
			newColor = p.Color
		}
		newGroup := reloaded.GroupName
		if p.GroupName != nil {
			newGroup = p.GroupName
		}
		if metaErr := s.tags.UpdateMetadata(ctx, workspaceID, finalName, newColor, newGroup); metaErr != nil {
			return nil, metaErr
		}
	}

	updated, err := s.tags.GetByName(ctx, workspaceID, finalName)
	if err != nil {
		return nil, err
	}
	return toTagDTO(updated), nil
}

func (s *tagService) Delete(ctx context.Context, workspaceID string, names []string) error {
	if err := s.guardMutableTags(ctx, workspaceID, names...); err != nil {
		return err
	}
	return s.tags.Delete(ctx, workspaceID, names)
}

func (s *tagService) BulkDelete(
	ctx context.Context,
	workspaceID string,
	names []string,
) (result BulkDeleteTagsResult, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.tags.bulk_delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.tags.requested_count", len(names)),
	)
	defer func() {
		span.SetAttributes(
			attribute.Int("damask.tags.deleted_count", result.Deleted),
			attribute.Int64("damask.assets.affected_count", result.RemovedFromAssets),
		)
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"tag bulk delete failed",
				"workspace_id",
				workspaceID,
				"tag_count",
				len(names),
				"error",
				err,
			)
		}
	}()

	err = s.tags.RunInTx(ctx, func(tx repository.TagRepository) error {
		if guardErr := guardMutableTagsRepo(ctx, tx, workspaceID, names...); guardErr != nil {
			return guardErr
		}
		for _, name := range names {
			tag, getErr := tx.GetByName(ctx, workspaceID, name)
			if isNotFound(getErr) {
				continue
			}
			if getErr != nil {
				return getErr
			}
			count, countErr := tx.CountAssets(ctx, tag.ID)
			if countErr != nil {
				return countErr
			}
			result.RemovedFromAssets += count
			if delErr := tx.Delete(ctx, workspaceID, []string{name}); delErr != nil {
				return delErr
			}
			result.Deleted++
		}
		return nil
	})
	return result, err
}

func (s *tagService) Merge(
	ctx context.Context,
	workspaceID string,
	sources []string,
	target string,
) (result MergeTagsResult, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.tags.merge",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.tags.source_count", len(sources)),
		attribute.String("damask.tags.target", target),
	)
	defer func() {
		span.SetAttributes(attribute.Int64("damask.assets.affected_count", result.MergedAssets))
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"tag merge failed",
				"workspace_id",
				workspaceID,
				"source_count",
				len(sources),
				"target",
				target,
				"error",
				err,
			)
		}
	}()

	err = s.tags.RunInTx(ctx, func(tx repository.TagRepository) error {
		tgt, upsertErr := tx.Upsert(ctx, workspaceID, target)
		if upsertErr != nil {
			return upsertErr
		}
		if guardErr := guardMutableTagsRepo(ctx, tx, workspaceID, sources...); guardErr != nil {
			return guardErr
		}
		for _, src := range sources {
			srcTag, getErr := tx.GetByName(ctx, workspaceID, src)
			if isNotFound(getErr) {
				continue
			}
			if getErr != nil {
				return getErr
			}
			count, countErr := tx.CountAssets(ctx, srcTag.ID)
			if countErr != nil {
				return countErr
			}
			result.MergedAssets += count
			if reassignErr := tx.ReassignAssets(ctx, srcTag.ID, tgt.ID); reassignErr != nil {
				return reassignErr
			}
			if delErr := tx.Delete(ctx, workspaceID, []string{src}); delErr != nil {
				return delErr
			}
		}
		reloaded, reloadErr := tx.GetByName(ctx, workspaceID, tgt.Name)
		if reloadErr != nil {
			return reloadErr
		}
		count, countErr := tx.CountAssets(ctx, reloaded.ID)
		if countErr != nil {
			return countErr
		}
		reloaded.AssetCount = count
		result.Target = toTagDTO(reloaded)
		return nil
	})
	return result, err
}

func (s *tagService) ResolveSystemTag(
	ctx context.Context,
	workspaceID, tagName string,
	scope SystemTagScope,
) (*AssetDTO, error) {
	tagName = strings.ToLower(strings.TrimSpace(tagName))
	if !systemtags.IsSystem(tagName) {
		return nil, fmt.Errorf("unknown system tag %q: %w", tagName, apperr.ErrInvalidInput)
	}

	if scope.FolderID != nil {
		asset, err := s.tags.FindAssetBySystemTagInFolder(ctx, workspaceID, tagName, *scope.FolderID)
		if err == nil {
			return toAssetDTO(asset), nil
		}
		if !isNotFound(err) {
			return nil, err
		}
	}

	if scope.ProjectID != nil {
		asset, err := s.tags.FindAssetBySystemTagInProject(ctx, workspaceID, tagName, *scope.ProjectID)
		if err == nil {
			return toAssetDTO(asset), nil
		}
		if !isNotFound(err) {
			return nil, err
		}
	}

	asset, err := s.tags.FindAssetBySystemTagInWorkspace(ctx, workspaceID, tagName)
	if err != nil {
		if isNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return toAssetDTO(asset), nil
}

func (s *tagService) TouchLastUsed(ctx context.Context, workspaceID, name string) error {
	return s.tags.TouchLastUsed(ctx, workspaceID, name)
}

func (s *tagService) ListForAsset(ctx context.Context, assetID string) ([]*TagDTO, error) {
	rows, err := s.tags.ListForAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(rows, toTagDTO)
	return out, nil
}

func (s *tagService) BatchTagsForAssets(ctx context.Context, assetIDs []string) (map[string][]string, error) {
	return s.tags.BatchTagsForAssets(ctx, assetIDs)
}

func (s *tagService) AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (*TagDTO, error) {
	tagName = strings.ToLower(strings.TrimSpace(tagName))
	slog.DebugContext(
		ctx,
		"tags: AddToAsset called",
		"workspace_id",
		workspaceID,
		"asset_id",
		assetID,
		"tag_name",
		tagName,
	)
	if tagName == "" {
		return nil, fmt.Errorf("tag name is required: %w", apperr.ErrInvalidInput)
	}
	var asset *repository.Asset
	if s.assets != nil {
		row, err := s.assets.GetByID(ctx, workspaceID, assetID)
		if err != nil {
			return nil, err
		}
		asset = &row
	}
	if systemtags.IsSystem(tagName) {
		if err := s.tags.EnsureSystemTag(ctx, workspaceID, tagName); err != nil {
			return nil, err
		}
	}
	tag, err := s.tags.Upsert(ctx, workspaceID, tagName)
	if err != nil {
		return nil, err
	}
	slog.DebugContext(ctx, "tags: upserted tag", "tag_id", tag.ID, "tag_name", tag.Name)
	// AddToAsset is idempotent: duplicate links are silently ignored at the repo level.
	if addErr := s.tags.AddToAsset(ctx, assetID, tag.ID); addErr != nil {
		return nil, addErr
	}
	slog.DebugContext(ctx, "tags: linked tag to asset", "asset_id", assetID, "tag_id", tag.ID)
	dto := toTagDTO(tag)
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetTagged,
		Payload:     audit.AssetTaggedPayload{V: 1, Tag: dto.Name},
	})
	if asset != nil {
		triggerData := workflowAssetTrigger{
			AssetID:          assetID,
			WorkspaceID:      workspaceID,
			ProjectID:        ptrStr(asset.ProjectID),
			FolderID:         ptrStr(asset.FolderID),
			MimeType:         asset.MimeType,
			OriginalFilename: asset.OriginalFilename,
			VersionID:        ptrStr(asset.CurrentVersionID),
			StorageKey:       asset.StorageKey,
		}.toMap()
		triggerData["tag"] = dto.Name
		publishWorkflowTriggerAsync(ctx, s.triggers, "trigger.tag_added", triggerData)
	}
	return dto, nil
}

func (s *tagService) ApplyTag(ctx context.Context, workspaceID, assetID, tagName string) error {
	_, err := s.AddToAsset(ctx, workspaceID, assetID, tagName)
	return err
}

func (s *tagService) RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	tagName = strings.ToLower(strings.TrimSpace(tagName))
	if _, err := s.tags.GetByName(ctx, workspaceID, tagName); err != nil {
		return err
	}
	if err := s.tags.RemoveFromAsset(ctx, workspaceID, assetID, tagName); err != nil {
		return err
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetUntagged,
		Payload:     audit.AssetUntaggedPayload{V: 1, Tag: tagName},
	})
	return nil
}

// UpsertForAsset upserts the tag by name and links it to the asset.
// Used by bulk-tag operations. Returns the tag without error if already linked.
func (s *tagService) UpsertForAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	tagName = strings.ToLower(strings.TrimSpace(tagName))
	if tagName == "" {
		return fmt.Errorf("tag name is required: %w", apperr.ErrInvalidInput)
	}
	if systemtags.IsSystem(tagName) {
		if err := s.tags.EnsureSystemTag(ctx, workspaceID, tagName); err != nil {
			return err
		}
	}
	tag, err := s.tags.Upsert(ctx, workspaceID, tagName)
	if err != nil {
		return err
	}
	return s.tags.AddToAsset(ctx, assetID, tag.ID)
}

func (s *tagService) guardMutableTags(ctx context.Context, workspaceID string, names ...string) error {
	return guardMutableTagsRepo(ctx, s.tags, workspaceID, names...)
}

func guardMutableTagsRepo(
	ctx context.Context,
	repo repository.TagRepository,
	workspaceID string,
	names ...string,
) error {
	for _, name := range names {
		tag, err := repo.GetByName(ctx, workspaceID, name)
		if isNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
		if tag.GroupName != nil && *tag.GroupName == systemtags.GroupName {
			return ErrSystemTagProtected
		}
	}
	return nil
}

func toTagDTO(t repository.Tag) *TagDTO {
	return &TagDTO{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		Name:        t.Name,
		Color:       t.Color,
		GroupName:   t.GroupName,
		AssetCount:  t.AssetCount,
		CreatedAt:   t.CreatedAt,
		LastUsedAt:  t.LastUsedAt,
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, apperr.ErrNotFound)
}

// ensure uuid import is used.
var _ = uuid.NewString
