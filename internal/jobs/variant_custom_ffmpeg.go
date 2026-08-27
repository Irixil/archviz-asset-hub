package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

// CustomFFmpegParams holds the user-supplied command for the custom_ffmpeg
// variant type. {input} and {output} are substituted with real temp paths
// at execution time — see transform.RunCustomFFmpeg.
type CustomFFmpegParams struct {
	Command string `json:"command"`
}

// customFFmpegBuild is the variantBuildFn for the custom_ffmpeg variant type.
func (s *JobServer) customFFmpegBuild(
	_ context.Context, _, _, workspaceID string, params json.RawMessage,
) (variantTransformer, error) {
	return s.customFFmpegTransformer(workspaceID, params)
}

// customFFmpegCanonical returns canonical JSON for custom_ffmpeg params.
func customFFmpegCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p CustomFFmpegParams
	_ = json.Unmarshal(params, &p)
	b, err := json.Marshal(p)
	return string(b), err
}

// customFFmpegTransformer returns a variantTransformer that runs a
// user-supplied ffmpeg command against the source file and detects the
// output MIME type via ffprobe.
func (s *JobServer) customFFmpegTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p CustomFFmpegParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse custom ffmpeg params: %w", err)
	}
	if err := transform.ValidateCustomCommand(p.Command); err != nil {
		return nil, err
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "custom_ffmpeg"),
		)
		rc, err := s.storage.Get(ctx, sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()

		outDir, err := os.MkdirTemp("", "damask-custom-ffmpeg-*")
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(outDir)

		refs, refErr := s.resolveCommandRefs(ctx, p.Command, workspaceID, outDir)
		if refErr != nil {
			telemetry.EndSpan(span, refErr)
			return nil, "", fmt.Errorf("resolve refs: %w", refErr)
		}

		outputPath, runErr := s.trf.RunCustomFFmpeg(ctx, p.Command, srcPath, outDir, refs)
		if runErr != nil {
			telemetry.EndSpan(span, runErr)
			return nil, "", runErr
		}

		data, readErr := os.ReadFile(outputPath)
		if readErr != nil {
			telemetry.EndSpan(span, readErr)
			return nil, "", fmt.Errorf("read output: %w", readErr)
		}

		mimeType := s.trf.DetectOutputMIME(ctx, outputPath)
		telemetry.EndSpan(span, nil)
		return data, mimeType, nil
	}, nil
}

// resolveCommandRefs downloads all {asset:ID} and {variant:ID} tokens found
// in cmd to temporary files under outDir. Returns a map from token string to
// local path, suitable for passing to transform.RunCustomFFmpeg. The map is
// empty (never nil) when cmd has no ref tokens.
//
// If any referenced asset or variant cannot be resolved (not found, wrong
// workspace, non-ready variant), the returned error carries a human-readable
// message intended for display via variant.error — the caller should let it
// propagate as an ordinary job error so wrapVariantJob's existing
// failed-status / SSE handling takes over; no separate "mark failed" call is
// needed here.
func (s *JobServer) resolveCommandRefs(
	ctx context.Context,
	cmd, workspaceID, outDir string,
) (map[string]string, error) {
	refs := transform.ExtractRefTokens(cmd)
	if len(refs) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string, len(refs))
	for _, ref := range refs {
		localPath, err := s.resolveOneRef(ctx, ref, workspaceID, outDir)
		if err != nil {
			return nil, err
		}
		result[ref.Token] = localPath
	}
	return result, nil
}

// resolveOneRef looks up the storage key for a single ref token (verifying
// workspace ownership) and downloads it to a temp file under outDir.
func (s *JobServer) resolveOneRef(
	ctx context.Context,
	ref transform.RefToken,
	workspaceID, outDir string,
) (string, error) {
	var storageKey string

	switch ref.Kind {
	case "asset":
		asset, err := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
			ID:          ref.ID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", fmt.Errorf("referenced asset %q not found in this workspace", ref.ID)
			}
			return "", fmt.Errorf("resolveOneRef: get asset %q: %w", ref.ID, err)
		}
		storageKey = asset.StorageKey

	case "variant":
		variant, err := s.queries.GetVariantByID(ctx, dbgen.GetVariantByIDParams{
			ID:          ref.ID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", fmt.Errorf("referenced variant %q not found in this workspace", ref.ID)
			}
			return "", fmt.Errorf("resolveOneRef: get variant %q: %w", ref.ID, err)
		}
		if variant.Status != variantStatusReady {
			return "", fmt.Errorf("referenced variant %q is not ready (status: %s)", ref.ID, variant.Status)
		}
		storageKey = variant.StorageKey

	default:
		return "", fmt.Errorf("resolveOneRef: unknown ref kind %q", ref.Kind)
	}

	rc, err := s.storage.Get(ctx, storageKey)
	if err != nil {
		return "", fmt.Errorf("resolveOneRef: download %s %q: %w", ref.Kind, ref.ID, err)
	}
	defer rc.Close()

	dst := filepath.Join(outDir, fmt.Sprintf("ref_%s_%s.tmp", ref.Kind, ref.ID))
	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("resolveOneRef: create temp file: %w", err)
	}
	defer f.Close()

	if _, copyErr := io.Copy(f, rc); copyErr != nil {
		return "", fmt.Errorf("resolveOneRef: copy %s %q to disk: %w", ref.Kind, ref.ID, copyErr)
	}
	return dst, nil
}
