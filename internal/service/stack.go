package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

// ExportZipParams is the input for StackService.ExportZip.
type ExportZipParams struct {
	AssetIDs    []string
	Filename    string // without extension; defaults to "stack-export"
	VariantMode string // "" | "none" | "shared" | "all"
}

// MergeParams is the input for StackService.EnqueueMerge.
type MergeParams struct {
	AssetIDs       []string
	OutputType     string // "gif" | "pdf"
	OutputFilename string
	GifFrameMs     int
}

func (p MergeParams) Validate() error {
	if len(p.AssetIDs) == 0 {
		return fmt.Errorf("asset_ids must not be empty: %w", apperr.ErrInvalidInput)
	}
	if p.OutputType != "gif" && p.OutputType != "pdf" {
		return fmt.Errorf("output_type must be gif or pdf: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// stackMergePayload is the JSON payload stored in the jobs queue row.
type stackMergePayload struct {
	WorkspaceID    string   `json:"workspace_id"`
	CreatedBy      string   `json:"created_by"`
	AssetIDs       []string `json:"asset_ids"`
	OutputType     string   `json:"output_type"`
	OutputFilename string   `json:"output_filename"`
	GifFrameMs     int      `json:"gif_frame_ms"`
}

type stackService struct {
	assets   repository.AssetRepository
	versions repository.VersionRepository
	variants repository.VariantRepository
	storage  storage.Storage
	q        queue.JobQueue
}

// NewStackService returns a StackService.
func NewStackService(
	assets repository.AssetRepository,
	versions repository.VersionRepository,
	variants repository.VariantRepository,
	stor storage.Storage,
	q queue.JobQueue,
) StackService {
	return &stackService{assets: assets, versions: versions, variants: variants, storage: stor, q: q}
}

// zipEntry is a single file to be written into a ZIP archive.
type zipEntry struct {
	name       string
	storageKey string
}

// zipGroup is one asset's folder in the ZIP, containing its original file and any variants.
type zipGroup struct {
	folder  string // subdirectory inside the ZIP; empty = flat
	entries []zipEntry
}

// ExportZip streams a ZIP archive of the given assets into w.
// All asset IDs must belong to workspaceID; if any are missing the call returns ErrForbidden.
func (s *stackService) ExportZip(ctx context.Context, workspaceID string, p ExportZipParams, w io.Writer) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.stack.export_zip",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(p.AssetIDs)),
	)
	var written int
	defer func() {
		span.SetAttributes(attribute.Int("damask.stack.entries_written", written))
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "stack export failed",
				"workspace_id", workspaceID,
				"asset_count", len(p.AssetIDs),
				"error", err,
			)
		}
	}()

	if len(p.AssetIDs) == 0 {
		return fmt.Errorf("asset_ids must not be empty: %w", apperr.ErrInvalidInput)
	}

	filename := sanitizeStackFilename(p.Filename)
	if filename == "" {
		filename = "stack-export"
	}
	_ = filename // used by caller for Content-Disposition

	// Verify all IDs belong to this workspace.
	for _, id := range p.AssetIDs {
		if _, checkErr := s.assets.GetByID(ctx, workspaceID, id); checkErr != nil {
			return fmt.Errorf("asset %q not found in workspace: %w", id, apperr.ErrForbidden)
		}
	}

	groups, missingNames := s.buildZipGroups(ctx, workspaceID, p)

	zw := zip.NewWriter(w)
	missing := missingNames

	for _, g := range groups {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		m, groupErr := s.writeZipGroup(ctx, zw, g)
		written += len(g.entries) - len(m)
		missing = append(missing, m...)
		if groupErr != nil {
			return groupErr
		}
	}

	// A cancel during the last group breaks out of writeZipGroup without an
	// error; don't finish the archive as if it were complete.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	if len(missing) > 0 {
		fw, fwErr := zw.Create("_missing_files.txt")
		if fwErr == nil {
			for _, n := range missing {
				_, _ = fmt.Fprintln(fw, n)
			}
		}
	}

	return zw.Close()
}

// buildZipGroups resolves assets and their variants into groups ready for ZIP writing.
func (s *stackService) buildZipGroups(
	ctx context.Context,
	workspaceID string,
	p ExportZipParams,
) ([]zipGroup, []string) {
	includeVariants := p.VariantMode == "shared" || p.VariantMode == "all"
	useFolder := includeVariants || len(p.AssetIDs) > 1

	var groups []zipGroup
	var missing []string
	usedFolders := map[string]int{}

	for _, id := range p.AssetIDs {
		asset, err := s.assets.GetByID(ctx, workspaceID, id)
		if err != nil {
			continue
		}
		version, verErr := s.versions.GetCurrentByAsset(ctx, asset.ID)
		if verErr != nil {
			missing = append(missing, asset.OriginalFilename)
			continue
		}

		entries := []zipEntry{{name: asset.OriginalFilename, storageKey: version.StorageKey}}
		if includeVariants {
			entries = append(entries, s.variantEntries(ctx, workspaceID, asset.ID, p.VariantMode)...)
		}

		folder := ""
		if useFolder {
			folder = uniqueName(asset.OriginalFilename, usedFolders)
		}
		groups = append(groups, zipGroup{folder: folder, entries: entries})
	}
	return groups, missing
}

// variantEntries returns ready variant ZIP entries for an asset, filtered by mode.
func (s *stackService) variantEntries(ctx context.Context, workspaceID, assetID, mode string) []zipEntry {
	const idPrefixLen = 8
	all, err := s.variants.ListByAsset(ctx, workspaceID, assetID)
	if err != nil {
		slog.WarnContext(ctx, "stack export: list variants", "asset_id", assetID, "err", err)
		return nil
	}
	used := map[string]int{}
	var out []zipEntry
	for _, v := range all {
		if v.Status != "ready" {
			continue
		}
		if mode == "shared" && !v.IsShared {
			continue
		}
		ext := storageExt(v.StorageKey)
		var base string
		if v.Title != nil && *v.Title != "" {
			base = *v.Title + ext
		} else {
			short := v.ID
			if len(short) > idPrefixLen {
				short = short[:idPrefixLen]
			}
			base = v.Type + "_" + short + ext
		}
		out = append(out, zipEntry{name: uniqueName(base, used), storageKey: v.StorageKey})
	}
	return out
}

// writeZipGroup writes all entries in g to zw, returning the paths of any that
// could not be read from storage. A mid-copy failure corrupts the ZIP stream,
// so it aborts the group and is returned as an error.
func (s *stackService) writeZipGroup(ctx context.Context, zw *zip.Writer, g zipGroup) ([]string, error) {
	usedInFolder := map[string]int{}
	var missing []string
	for _, e := range g.entries {
		if ctx.Err() != nil {
			break
		}
		name := uniqueName(e.name, usedInFolder)
		zipPath := name
		if g.folder != "" {
			zipPath = g.folder + "/" + name
		}
		rc, rcErr := s.storage.Get(ctx, e.storageKey)
		if rcErr != nil {
			missing = append(missing, zipPath)
			continue
		}
		fw, fwErr := zw.Create(zipPath)
		if fwErr != nil {
			_ = rc.Close()
			missing = append(missing, zipPath)
			continue
		}
		_, copyErr := io.Copy(fw, rc)
		_ = rc.Close()
		if copyErr != nil {
			return missing, fmt.Errorf("zip copy %q: %w", zipPath, copyErr)
		}
	}
	return missing, nil //nolint:nilerr // unreadable entries go to the manifest; caller re-checks ctx.Err()
}

// EnqueueMerge enqueues a stack_merge job and returns the job ID.
// All asset IDs must belong to workspaceID.
func (s *stackService) EnqueueMerge(
	ctx context.Context,
	workspaceID, userID string,
	p MergeParams,
) (jobID string, err error) {
	ctx, span := telemetry.StartSpan(ctx, "service.stack.enqueue_merge",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Int("damask.assets.requested_count", len(p.AssetIDs)),
		attribute.String("damask.stack.output_type", p.OutputType),
	)
	defer func() {
		if jobID != "" {
			span.SetAttributes(attribute.String("damask.job_id", jobID))
		}
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"stack merge enqueue failed",
				"workspace_id",
				workspaceID,
				"asset_count",
				len(p.AssetIDs),
				"output_type",
				p.OutputType,
				"error",
				err,
			)
		}
	}()

	if valErr := p.Validate(); valErr != nil {
		return "", valErr
	}

	for _, id := range p.AssetIDs {
		if _, checkErr := s.assets.GetByID(ctx, workspaceID, id); checkErr != nil {
			return "", fmt.Errorf("asset %q not found in workspace: %w", id, apperr.ErrForbidden)
		}
	}

	payload := stackMergePayload{
		WorkspaceID:    workspaceID,
		CreatedBy:      userID,
		AssetIDs:       p.AssetIDs,
		OutputType:     p.OutputType,
		OutputFilename: p.OutputFilename,
		GifFrameMs:     p.GifFrameMs,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not encode payload: %w", err)
	}

	job, err := s.q.Enqueue(ctx, workspaceID, queue.JobTypeStackMerge, string(payloadJSON))
	if err != nil {
		return "", fmt.Errorf("could not enqueue merge job: %w", apperr.ErrConflict)
	}
	return job.ID, nil
}

// uniqueName returns name, deduplicating within used by appending _N suffixes.
func uniqueName(name string, used map[string]int) string {
	used[name]++
	if used[name] == 1 {
		return name
	}
	ext := ""
	stem := name
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		stem = name[:dot]
		ext = name[dot:]
	}
	return fmt.Sprintf("%s_%d%s", stem, used[name], ext)
}

// storageExt returns the file extension (including dot) from a storage key, or "".
func storageExt(key string) string {
	if dot := strings.LastIndex(key, "."); dot >= 0 {
		return key[dot:]
	}
	return ""
}

func sanitizeStackFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
