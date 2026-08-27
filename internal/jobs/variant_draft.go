package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

// customFFmpegDraftTrimSeconds is how much of the source the "Test" preview
// runs against. Kept short so the draft job timeout (110s) comfortably covers it.
const customFFmpegDraftTrimSeconds = 5

const (
	scratchPrefix   = "scratch/"
	draftJobTimeout = 110 * time.Second
	scratchMaxAge   = 24 * time.Hour
)

// CreateVariantDraftPayload is the job payload for create_variant_draft.
type CreateVariantDraftPayload struct {
	Nonce       string          `json:"nonce"`
	WorkspaceID string          `json:"workspace_id"`
	UserID      string          `json:"user_id"`
	AssetID     string          `json:"asset_id"`
	Type        string          `json:"type"`
	Params      json.RawMessage `json:"params"`
}

// scratchDraftMeta is the sidecar JSON written alongside each draft output.
type scratchDraftMeta struct {
	AssetID         string `json:"asset_id"`
	AssetVersionID  string `json:"asset_version_id"`
	WorkspaceID     string `json:"workspace_id"`
	UserID          string `json:"user_id"`
	VariantType     string `json:"variant_type"`
	TransformParams string `json:"transform_params"`
	ContentType     string `json:"content_type"`
	CreatedAt       string `json:"created_at"`
}

func scratchKey(workspaceID, userID, nonce string) string {
	return fmt.Sprintf("%s%s/%s/%s", scratchPrefix, workspaceID, userID, nonce)
}

func scratchMetaKey(workspaceID, userID, nonce string) string {
	return scratchKey(workspaceID, userID, nonce) + ".meta"
}

func (s *JobServer) jobCreateVariantDraft(ctx context.Context, job dbgen.Job) error {
	var p CreateVariantDraftPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse draft payload: %w", err)
	}

	publishCtx := context.WithoutCancel(ctx)
	ctx, cancel := context.WithTimeout(ctx, draftJobTimeout)
	defer cancel()

	publishErr := func(msg string) {
		// Detach from the timeout-bound ctx so a timed-out job doesn't swallow the
		// error event, while keeping trace linkage to the originating job context.
		s.hub.Publish(publishCtx, p.WorkspaceID, events.VariantDraftError(p.Nonce, msg))
	}

	ver, err := s.queries.GetCurrentVersion(ctx, p.AssetID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		publishErr("asset not found")
		return nil
	case err != nil:
		return err // transient DB error — let the queue retry
	case ver.WorkspaceID != job.WorkspaceID:
		publishErr("asset not found")
		return nil
	}

	trf, buildErr := s.draftTransformer(ctx, p.WorkspaceID, p.Type, p.Params)
	if buildErr != nil {
		publishErr(buildErr.Error())
		return nil
	}

	outputBytes, contentType, callErr := trf(ctx, ver.StorageKey)
	if callErr != nil {
		publishErr(callErr.Error())
		return nil
	}

	sk := scratchKey(p.WorkspaceID, p.UserID, p.Nonce)
	mk := scratchMetaKey(p.WorkspaceID, p.UserID, p.Nonce)

	if err = s.storage.Put(ctx, sk, bytes.NewReader(outputBytes)); err != nil {
		publishErr("failed to store draft output")
		return nil
	}

	meta := scratchDraftMeta{
		AssetID:         p.AssetID,
		AssetVersionID:  ver.ID,
		WorkspaceID:     p.WorkspaceID,
		UserID:          p.UserID,
		VariantType:     p.Type,
		TransformParams: string(p.Params),
		ContentType:     contentType,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	metaBytes, _ := json.Marshal(meta)
	if err = s.storage.Put(ctx, mk, bytes.NewReader(metaBytes)); err != nil {
		// Non-fatal — clean up output and report error.
		_ = s.storage.Delete(ctx, sk)
		publishErr("failed to store draft metadata")
		return nil
	}

	expiresAt := nextPurgeTime(s.cfg.Scratch)
	s.hub.Publish(ctx, p.WorkspaceID, events.VariantDraftReady(
		p.Nonce,
		p.AssetID,
		fmt.Sprintf("/api/v1/assets/%s/variants/draft/%s/preview", p.AssetID, p.Nonce),
		expiresAt.Format(time.RFC3339),
	))
	return nil
}

// draftTransformer returns a variantTransformer for the given draft type.
// The draft watermark case needs a version storage key loaded ahead of time,
// so ctx and ver.StorageKey are not used — the transformer fetches the source itself.
func (s *JobServer) draftTransformer(
	ctx context.Context,
	workspaceID, variantType string,
	params json.RawMessage,
) (variantTransformer, error) {
	switch variantType {
	case queue.JobTypeImageBgRemove:
		return s.imageBgRemoveTransformer(workspaceID, params)

	case queue.JobTypeImageWithPrompt:
		return s.imageWithPromptTransformer(workspaceID, params)

	case queue.JobTypeImageWatermark:
		var p transform.WatermarkParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errors.New("invalid params")
		}
		if p.WatermarkAssetID == "" {
			return nil, errors.New("watermark asset id is required")
		}
		wm, err := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
			ID:          p.WatermarkAssetID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return nil, errors.New("watermark asset not found")
		}
		return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
			wmRC, e := s.storage.Get(ctx, wm.StorageKey)
			if e != nil {
				return nil, "", errors.New("failed to load watermark file")
			}
			srcRC, e := s.storage.Get(ctx, sourceKey)
			if e != nil {
				_ = wmRC.Close()
				return nil, "", errors.New("failed to load asset file")
			}
			data, contentType, wmErr := s.trf.ImageWatermark(srcRC, wmRC, p)
			_ = srcRC.Close()
			_ = wmRC.Close()
			if wmErr != nil {
				return nil, "", wmErr
			}
			return data, contentType, nil
		}, nil

	case queue.JobTypeCustomFFmpeg:
		return s.customFFmpegDraftTransformer(workspaceID, params)

	default:
		return nil, fmt.Errorf("unsupported draft type: %s", variantType)
	}
}

// customFFmpegDraftTransformer returns a variantTransformer that trims the
// source to a short preview clip before running the user's command, so the
// "Test" dry-run stays fast regardless of source file length.
func (s *JobServer) customFFmpegDraftTransformer(
	workspaceID string,
	params json.RawMessage,
) (variantTransformer, error) {
	var p CustomFFmpegParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, errors.New("invalid params")
	}
	if err := transform.ValidateCustomCommand(p.Command); err != nil {
		return nil, err
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		rc, err := s.storage.Get(ctx, sourceKey)
		if err != nil {
			return nil, "", errors.New("failed to load asset file")
		}
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		_ = rc.Close()
		if err != nil {
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()

		outDir, err := os.MkdirTemp("", "damask-custom-ffmpeg-draft-*")
		if err != nil {
			return nil, "", fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(outDir)

		// Trim to a short preview clip; falls back to the full file if
		// trimming fails so the test never errors out for that reason.
		trimmed, _ := s.trf.TrimToSeconds(ctx, srcPath, customFFmpegDraftTrimSeconds, outDir)

		refs, refErr := s.resolveCommandRefs(ctx, p.Command, workspaceID, outDir)
		if refErr != nil {
			return nil, "", refErr
		}

		outputPath, runErr := s.trf.RunCustomFFmpeg(ctx, p.Command, trimmed, outDir, refs)
		if runErr != nil {
			return nil, "", runErr
		}
		data, readErr := os.ReadFile(outputPath)
		if readErr != nil {
			return nil, "", fmt.Errorf("read output: %w", readErr)
		}
		mimeType := s.trf.DetectOutputMIME(ctx, outputPath)
		return data, mimeType, nil
	}, nil
}

// nextPurgeTime returns today's purge time if it hasn't passed yet, otherwise tomorrow's.
func nextPurgeTime(cfg config.ScratchConfig) time.Time {
	h, m := cfg.PurgeHourMinute()
	now := time.Now().UTC()
	t := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.UTC)
	if !t.After(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

// ---- Purge job ----

func (s *JobServer) jobPurgeScratchVariants(ctx context.Context, _ dbgen.Job) error {
	keys, err := s.storage.List(ctx, scratchPrefix)
	if err != nil {
		return fmt.Errorf("list scratch keys: %w", err)
	}

	cutoff := time.Now().UTC().Add(-scratchMaxAge)

	// Index output keys for orphan-meta detection.
	outputKeys := make(map[string]bool, len(keys))
	for _, k := range keys {
		if !strings.HasSuffix(k, ".meta") {
			outputKeys[k] = true
		}
	}

	deletedOutputs := 0
	deletedMetas := 0

	// First pass: delete old output files (and their metas) using the meta
	// sidecar's CreatedAt field as the age source.
	for _, k := range keys {
		if strings.HasSuffix(k, ".meta") {
			continue
		}
		metaK := k + ".meta"
		createdAt, metaErr := scratchMetaCreatedAt(ctx, s.storage, metaK)
		if metaErr != nil {
			// No meta or unreadable — treat as old if output key itself has no meta.
			createdAt = time.Time{}
		}
		if createdAt.After(cutoff) {
			continue
		}
		if delErr := s.storage.Delete(ctx, k); delErr != nil {
			slog.WarnContext(ctx, "purge_scratch: delete output failed", "key", k, "error", delErr)
		} else {
			deletedOutputs++
		}
		if delErr := s.storage.Delete(ctx, metaK); delErr != nil && !errors.Is(delErr, storage.ErrNotFound) {
			slog.WarnContext(ctx, "purge_scratch: delete meta failed", "key", metaK, "error", delErr)
		} else if delErr == nil {
			deletedMetas++
		}
	}

	// Second pass: orphan metas (meta exists but output does not).
	for _, k := range keys {
		if !strings.HasSuffix(k, ".meta") {
			continue
		}
		outputK := strings.TrimSuffix(k, ".meta")
		if outputKeys[outputK] {
			continue
		}
		createdAt, metaErr := scratchMetaCreatedAt(ctx, s.storage, k)
		if metaErr != nil {
			createdAt = time.Time{}
		}
		if createdAt.After(cutoff) {
			continue
		}
		if delErr := s.storage.Delete(ctx, k); delErr != nil {
			slog.WarnContext(ctx, "purge_scratch: delete orphan meta failed", "key", k, "error", delErr)
		} else {
			deletedMetas++
		}
	}

	slog.InfoContext(ctx, "purge_scratch_variants: done",
		"deleted_outputs", deletedOutputs,
		"deleted_orphan_metas", deletedMetas,
	)
	return nil
}

// scratchMetaCreatedAt reads a .meta sidecar and returns its CreatedAt timestamp.
func scratchMetaCreatedAt(ctx context.Context, stor storage.Storage, metaKey string) (time.Time, error) {
	rc, err := stor.Get(ctx, metaKey)
	if err != nil {
		return time.Time{}, err
	}
	defer rc.Close()
	var meta scratchDraftMeta
	if err = json.NewDecoder(rc).Decode(&meta); err != nil {
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339, meta.CreatedAt)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// ---- Scratch Purge Scheduler ----

// ScratchPurgeScheduler fires the purge_scratch_variants job once per day
// at the configured purge time.
type ScratchPurgeScheduler struct {
	queue queue.JobQueue
	cfg   *config.Config
}

// NewScratchPurgeScheduler creates a ScratchPurgeScheduler.
func NewScratchPurgeScheduler(q queue.JobQueue, cfg *config.Config) *ScratchPurgeScheduler {
	return &ScratchPurgeScheduler{queue: q, cfg: cfg}
}

// Start launches the scheduler goroutine. It exits when ctx is cancelled.
func (s *ScratchPurgeScheduler) Start(ctx context.Context) {
	go func() {
		for {
			h, m := s.cfg.Scratch.PurgeHourMinute()
			next := NextDaily(h, m)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := s.queue.Enqueue(ctx, "system", queue.JobTypePurgeScratchVariants, "{}"); err != nil {
					slog.ErrorContext(ctx, "scratch purge scheduler: enqueue", "error", err)
				} else {
					slog.InfoContext(ctx, "scratch purge scheduler: enqueued purge_scratch_variants")
				}
			}
		}
	}()
}
