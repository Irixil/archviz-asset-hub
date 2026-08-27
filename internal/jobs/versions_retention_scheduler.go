package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

// purgeVersionStoragePayload carries the data needed by the purge job.
type purgeVersionStoragePayload struct {
	VersionID   string `json:"version_id"`
	WorkspaceID string `json:"workspace_id"`
}

// jobEnforceVersionRetention runs the retention policy for all workspaces
// that have version_retention_count > 0.
func (s *JobServer) jobEnforceVersionRetention(ctx context.Context, _ dbgen.Job) error {
	workspaces, err := s.queries.ListWorkspacesWithRetention(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	for _, ws := range workspaces {
		if ws.VersionRetentionCount <= 0 {
			continue
		}
		if err = s.EnforceRetentionForWorkspace(ctx, ws); err != nil {
			slog.ErrorContext(ctx, "retention: workspace failed", "workspace_id", ws.ID, "error", err)
		}
	}
	return nil
}

func (s *JobServer) EnforceRetentionForWorkspace(ctx context.Context, ws dbgen.Workspace) error {
	assetIDs, err := s.queries.ListAssetsWithVersions(ctx, ws.ID)
	if err != nil {
		return fmt.Errorf("list assets: %w", err)
	}

	keep := ws.VersionRetentionCount

	for _, assetID := range assetIDs {
		count, countErr := s.queries.CountActiveVersions(ctx, assetID)
		if countErr != nil {
			slog.ErrorContext(ctx, "retention: count versions", "asset_id", assetID, "error", countErr)
			continue
		}
		if count <= keep {
			continue
		}

		// The query returns non-current versions ordered by version_num DESC,
		// skipping the `keep` most recent, so what comes back should be deleted.
		beyond, beyondErr := s.queries.ListVersionsBeyondRetention(ctx, dbgen.ListVersionsBeyondRetentionParams{
			AssetID: assetID,
			Offset:  keep, // skip the `keep` most recent non-current versions
		})
		if beyondErr != nil {
			slog.ErrorContext(ctx, "retention: list beyond retention", "asset_id", assetID, "error", beyondErr)
			continue
		}

		for _, v := range beyond {
			// Safety: never soft-delete the current version.
			if v.IsCurrent == 1 {
				continue
			}
			// Safety: skip versions referenced as a project cover or workspace icon.
			if refs, refErr := s.queries.IsVersionReferencedAsCover(ctx, dbgen.IsVersionReferencedAsCoverParams{
				CoverVersionID: &v.ID,
				IconVersionID:  &v.ID,
			}); refErr == nil && refs > 0 {
				slog.DebugContext(ctx, "retention: skipping version in use as cover/icon", "version_id", v.ID)
				continue
			}
			if delErr := s.queries.SoftDeleteVersion(ctx, v.ID); delErr != nil {
				slog.ErrorContext(ctx, "retention: soft-delete version", "version_id", v.ID, "error", delErr)
				continue
			}
			// Enqueue physical purge after 7-day grace period.
			payload, _ := json.Marshal(purgeVersionStoragePayload{
				VersionID:   v.ID,
				WorkspaceID: ws.ID,
			})
			if _, enqErr := s.queue.Enqueue(
				ctx,
				ws.ID,
				queue.JobTypePurgeVersionStorage,
				string(payload),
			); enqErr != nil {
				slog.ErrorContext(ctx, "retention: enqueue purge", "version_id", v.ID, "error", enqErr)
			}
		}
	}
	return nil
}

// jobPurgeVersionStorage physically removes a soft-deleted version's files
// from storage after the 7-day grace period, then hard-deletes the row.
func (s *JobServer) jobPurgeVersionStorage(ctx context.Context, job dbgen.Job) error {
	var p purgeVersionStoragePayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	ver, err := s.queries.GetVersionByIDUnchecked(ctx, p.VersionID)
	if err != nil {
		return nil //nolint:nilerr // already hard-deleted; nothing to do.
	}
	if ver.WorkspaceID != job.WorkspaceID {
		slog.WarnContext(ctx, "purge-version-storage: version not in job workspace — skipping",
			"version_id", p.VersionID, "workspace_id", job.WorkspaceID)
		return nil
	}

	// Safety guard: never purge a current version.
	if ver.IsCurrent == 1 {
		slog.WarnContext(ctx, "purge-version-storage: version is current — skipping", "version_id", p.VersionID)
		return nil
	}

	// Enforce 7-day grace period.
	if ver.DeletedAt == nil {
		slog.ErrorContext(ctx, "purge-version-storage: version has no deleted_at — skipping", "version_id", p.VersionID)
		return nil
	}
	deletedAt, err := ParseSQLiteTime(*ver.DeletedAt)
	if err != nil {
		return fmt.Errorf("parse deleted_at: %w", err)
	}
	if time.Since(deletedAt) < 7*24*time.Hour {
		// Grace period not yet elapsed — re-enqueue so the job is retried rather
		// than silently dropped. It will check the elapsed time again on next run.
		slog.InfoContext(
			ctx,
			"purge-version-storage: grace period not elapsed — re-enqueuing",
			"version_id",
			p.VersionID,
		)
		payload, _ := json.Marshal(p)
		if _, enqErr := s.queue.Enqueue(
			ctx,
			p.WorkspaceID,
			queue.JobTypePurgeVersionStorage,
			string(payload),
		); enqErr != nil {
			slog.ErrorContext(ctx, "purge-version-storage: re-enqueue", "version_id", p.VersionID, "error", enqErr)
		}
		return nil
	}

	// Delete variant storage files for this version (VV-3.3).
	// DB rows are cleaned up by ON DELETE CASCADE when the version row is hard-deleted.
	variants, varErr := s.queries.ListVariantsByVersion(ctx, p.VersionID)
	if varErr != nil {
		slog.ErrorContext(ctx, "purge-version-storage: list variants", "version_id", p.VersionID, "error", varErr)
	}
	for _, v := range variants {
		if delErr := s.storage.Delete(ctx, v.StorageKey); delErr != nil {
			slog.ErrorContext(
				ctx,
				"purge-version-storage: delete variant storage",
				"storage_key",
				v.StorageKey,
				"error",
				delErr,
			)
		}
		if v.ThumbnailKey != nil {
			if delErr := s.storage.Delete(ctx, *v.ThumbnailKey); delErr != nil {
				slog.ErrorContext(
					ctx,
					"purge-version-storage: delete variant thumb",
					"storage_key",
					*v.ThumbnailKey,
					"error",
					delErr,
				)
			}
		}
	}

	// Delete source + thumbnail storage files.
	if delErr := s.storage.Delete(ctx, ver.StorageKey); delErr != nil {
		slog.ErrorContext(ctx, "purge-version-storage: delete storage", "storage_key", ver.StorageKey, "error", delErr)
	}
	if ver.ThumbnailKey != nil {
		if delErr := s.storage.Delete(ctx, *ver.ThumbnailKey); delErr != nil {
			slog.ErrorContext(
				ctx,
				"purge-version-storage: delete thumb",
				"storage_key",
				*ver.ThumbnailKey,
				"error",
				delErr,
			)
		}
	}

	// Hard-delete the row (ON DELETE CASCADE removes variant DB rows automatically).
	return s.queries.HardDeleteVersion(ctx, p.VersionID)
}
