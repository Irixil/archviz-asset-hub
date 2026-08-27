package visualsimilarity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

// Service handles storing and querying perceptual hashes.
type Service struct {
	q  *dbgen.Queries
	db *sql.DB
}

func NewService(q *dbgen.Queries, db *sql.DB) *Service {
	return &Service{q: q, db: db}
}

// Store persists hashes for an asset version. Safe to call multiple times — uses INSERT OR REPLACE.
func (s *Service) Store(ctx context.Context, workspaceID, assetVersionID string, h Hashes) error {
	hashSet, err := MarshalHashSet(h.HashSet)
	if err != nil {
		return fmt.Errorf("marshal hash set: %w", err)
	}
	_, span := telemetry.StartSpan(ctx, "visual_similarity.store",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_version_id", assetVersionID),
	)
	err = s.q.UpsertVisualSimilarityHash(ctx, dbgen.UpsertVisualSimilarityHashParams{
		AssetVersionID: assetVersionID,
		WorkspaceID:    workspaceID,
		CentralHash:    int64(h.CentralHash), //nolint:gosec // SQLite INTEGER is signed 64-bit
		HashSet:        hashSet,
	})
	telemetry.EndSpan(span, err)
	return err
}

// SimilarAsset holds the enriched data returned by FindSimilarEnriched.
type SimilarAsset struct {
	AssetVersionID   string
	AssetID          string
	OriginalFilename string
	MimeType         string
	Width            *int64
	Height           *int64
	ThumbnailKey     *string
}

// FindSimilarEnriched returns enriched asset+version data for visually similar images in a
// single JOIN query, avoiding the N+1 that FindSimilar + per-ID lookups would cause.
func (s *Service) FindSimilarEnriched(ctx context.Context, workspaceID, assetVersionID string) ([]SimilarAsset, error) {
	row, err := s.q.GetVisualSimilarityHash(ctx, assetVersionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []SimilarAsset{}, nil
		}
		return nil, fmt.Errorf("get hash: %w", err)
	}

	hashSet, err := UnmarshalHashSet(row.HashSet)
	if err != nil {
		return nil, fmt.Errorf("unmarshal hash set: %w", err)
	}
	if len(hashSet) == 0 {
		return []SimilarAsset{}, nil
	}

	placeholders := strings.Repeat("?,", len(hashSet))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf( //nolint:gosec // only placeholders
		`SELECT av.id, av.asset_id, a.original_filename, av.mime_type, av.width, av.height, av.thumbnail_key
		FROM asset_versions av
		JOIN assets a ON a.id = av.asset_id
		WHERE av.id IN (
			SELECT DISTINCT asset_version_id
			FROM asset_visual_similarity_hashes
			WHERE workspace_id = ? AND central_hash IN (%s) AND asset_version_id != ?
		)
		AND a.workspace_id = ?`, placeholders)

	//nolint:mnd // The number of args is dynamic based on the hash set size.
	args := make([]any, 0, 1+len(hashSet)+2)
	args = append(args, workspaceID)
	for _, v := range hashSet {
		args = append(args, v)
	}
	args = append(args, assetVersionID, workspaceID)

	_, span := telemetry.StartSpan(ctx, "visual_similarity.find_enriched",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_version_id", assetVersionID),
	)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, fmt.Errorf("query similar enriched: %w", err)
	}
	defer rows.Close()

	var results []SimilarAsset
	for rows.Next() {
		var r SimilarAsset
		if err = rows.Scan(
			&r.AssetVersionID,
			&r.AssetID,
			&r.OriginalFilename,
			&r.MimeType,
			&r.Width,
			&r.Height,
			&r.ThumbnailKey,
		); err != nil {
			telemetry.EndSpan(span, err)
			return nil, fmt.Errorf("scan: %w", err)
		}
		results = append(results, r)
	}
	if err = rows.Err(); err != nil {
		telemetry.EndSpan(span, err)
		return nil, err
	}
	telemetry.EndSpan(span, nil)
	if results == nil {
		results = []SimilarAsset{}
	}
	return results, nil
}

// FindSimilar returns asset version IDs whose central_hash appears in the query version's
// hash_set, excluding the query version itself. Results are deduplicated.
func (s *Service) FindSimilar(ctx context.Context, workspaceID, assetVersionID string) ([]string, error) {
	row, err := s.q.GetVisualSimilarityHash(ctx, assetVersionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("get hash: %w", err)
	}

	hashSet, err := UnmarshalHashSet(row.HashSet)
	if err != nil {
		return nil, fmt.Errorf("unmarshal hash set: %w", err)
	}
	if len(hashSet) == 0 {
		return []string{}, nil
	}

	// Build dynamic IN clause: WHERE workspace_id = ? AND central_hash IN (?, ?, ...) AND asset_version_id != ?
	placeholders := strings.Repeat("?,", len(hashSet))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf( //nolint:gosec // only placeholders
		`SELECT DISTINCT asset_version_id FROM asset_visual_similarity_hashes
		 WHERE workspace_id = ? AND central_hash IN (%s) AND asset_version_id != ?`,
		placeholders,
	)

	args := make([]any, 0, 1+len(hashSet)+1)
	args = append(args, workspaceID)
	for _, v := range hashSet {
		args = append(args, v)
	}
	args = append(args, assetVersionID)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query similar: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		results = append(results, id)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []string{}
	}
	return results, nil
}
