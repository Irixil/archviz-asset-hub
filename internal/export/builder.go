package export

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
)

// BuildProgress reports incremental progress during Build.
type BuildProgress struct {
	AssetsExported int
	AssetsSkipped  int
	BytesWritten   int64
}

// BuildResult is returned by Build on success.
type BuildResult struct {
	AssetsTotal    int
	AssetsExported int
	AssetsSkipped  int
	BytesWritten   int64
	ManifestJSON   []byte
}

// BuildParams holds all dependencies needed by Build.
type BuildParams struct {
	Config     repository.ExportConfig
	Run        repository.ExportRun
	Dest       Destination
	Storage    storage.Storage
	Queries    *dbgen.Queries
	SQLite     *sql.DB
	AppSecret  string
	Project    dbgen.Project
	OnProgress func(p BuildProgress) // called every 10 assets, safe to nil
}

// remoteKey builds the incremental dedup key for an item.
func remoteKey(assetID string, versionNum int64, variantID string) string {
	return fmt.Sprintf("%s:%d:%s", assetID, versionNum, variantID)
}

// assetVersionRow is the common shape produced by both asset-query paths.
type assetVersionRow struct {
	assetID          string
	originalFilename string
	folderName       *string
	versionID        string
	versionNum       int64
	storageKey       string
	contentHash      string
	mimeType         string
	size             int64
	comment          *string
	versionCreatedAt string
	isCurrent        bool
}

// itemInfo pairs a resolved ZIP path with its source row and optional variant.
type itemInfo struct {
	row          assetVersionRow
	variant      *dbgen.Variant // nil for originals
	resolvedPath string
}

// buildCtx holds the state accumulated while running a single Build call.
type buildCtx struct {
	p            BuildParams
	remoteHashes map[string]string
	tagsMap      map[string][]string
	fieldsMap    map[string][]ManifestFieldVal
	variantsMap  map[string][]dbgen.Variant // versionID → variants
	manifest     Manifest
	assetIndex   map[string]int // assetID → index in manifest.Assets
	result       BuildResult
}

// Build assembles the ZIP archive and writes it to the destination.
// The temp file is cleaned up automatically.
func Build(ctx context.Context, p BuildParams) (BuildResult, error) {
	b := &buildCtx{
		p:            p,
		remoteHashes: map[string]string{},
		assetIndex:   map[string]int{},
	}

	if err := b.loadRemoteHashes(ctx); err != nil {
		slog.WarnContext(ctx, "export: read remote manifest failed", "error", err)
	}

	avRows, err := b.queryAssetVersions(ctx)
	if err != nil {
		return BuildResult{}, err
	}

	err = b.queryTagsAndFields(ctx)
	if err != nil {
		return BuildResult{}, err
	}

	err = b.queryVariants(ctx, collectVersionIDs(avRows))
	if err != nil {
		return BuildResult{}, err
	}

	items := b.resolveItems(avRows)
	b.manifest = newManifest(p)

	f, err := os.CreateTemp("", "damask-export-*.zip")
	if err != nil {
		return BuildResult{}, fmt.Errorf("export: create temp file: %w", err)
	}
	defer os.Remove(f.Name())

	zw := zip.NewWriter(f)

	for i, item := range items {
		written, skipped, itemErr := b.writeItem(ctx, item, zw)
		if itemErr != nil {
			_ = zw.Close()
			_ = f.Close()
			return BuildResult{}, itemErr
		}
		if skipped {
			b.result.AssetsSkipped++
		} else {
			b.result.AssetsExported++
			b.result.BytesWritten += written
		}
		size := item.row.size
		if item.variant != nil && item.variant.Size != nil {
			size = *item.variant.Size
		}
		if size == 0 {
			size = written
		}
		b.appendToManifest(item, size)

		if p.OnProgress != nil && (i+1)%10 == 0 {
			p.OnProgress(BuildProgress{
				AssetsExported: b.result.AssetsExported,
				AssetsSkipped:  b.result.AssetsSkipped,
				BytesWritten:   b.result.BytesWritten,
			})
		}
	}

	manifestJSON, err := b.writeManifestToZip(zw)
	if err != nil {
		return BuildResult{}, err
	}

	err = b.writeToDestination(ctx, f, manifestJSON)
	if err != nil {
		return BuildResult{}, err
	}

	b.result.AssetsTotal = b.result.AssetsExported + b.result.AssetsSkipped
	b.result.ManifestJSON = manifestJSON
	return b.result, nil
}

// loadRemoteHashes reads the sidecar manifest from the destination and populates
// b.remoteHashes for incremental dedup. A missing manifest is not an error.
func (b *buildCtx) loadRemoteHashes(ctx context.Context) error {
	sidecarPath := slugify(b.p.Project.Name) + "__manifest.json"
	manifestBytes, err := b.p.Dest.ReadManifest(ctx, sidecarPath)
	if err != nil {
		return err
	}
	if manifestBytes == nil {
		return nil
	}
	var prev Manifest
	if jsonErr := json.Unmarshal(manifestBytes, &prev); jsonErr != nil {
		return jsonErr
	}
	for _, a := range prev.Assets {
		for _, v := range a.Versions {
			b.remoteHashes[remoteKey(a.ID, v.VersionNum, "")] = v.ContentHash
			for _, vr := range v.Variants {
				b.remoteHashes[remoteKey(a.ID, v.VersionNum, vr.ID)] = vr.ContentHash
			}
		}
	}
	return nil
}

// queryAssetVersions fetches asset versions based on the export config's Versions setting.
func (b *buildCtx) queryAssetVersions(ctx context.Context) ([]assetVersionRow, error) {
	projectID := b.p.Config.ProjectID
	workspaceID := b.p.Config.WorkspaceID

	if b.p.Config.Versions == "all" {
		rows, err := b.p.Queries.GetProjectAllVersionsForExport(ctx, dbgen.GetProjectAllVersionsForExportParams{
			ProjectID:   &projectID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return nil, fmt.Errorf("export: query all versions: %w", err)
		}
		out := make([]assetVersionRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, assetVersionRow{
				assetID:          r.AssetID,
				originalFilename: r.OriginalFilename,
				folderName:       r.FolderName,
				versionID:        r.VersionID,
				versionNum:       r.VersionNum,
				storageKey:       r.StorageKey,
				contentHash:      r.ContentHash,
				mimeType:         r.MimeType,
				size:             r.Size,
				comment:          r.Comment,
				versionCreatedAt: r.VersionCreatedAt,
				isCurrent:        r.IsCurrent == 1,
			})
		}
		return out, nil
	}

	rows, err := b.p.Queries.GetProjectAssetsForExport(ctx, dbgen.GetProjectAssetsForExportParams{
		ProjectID:   &projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("export: query current versions: %w", err)
	}
	out := make([]assetVersionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, assetVersionRow{
			assetID:          r.ID,
			originalFilename: r.OriginalFilename,
			folderName:       r.FolderName,
			versionID:        r.VersionID,
			versionNum:       r.VersionNum,
			storageKey:       r.StorageKey,
			contentHash:      r.ContentHash,
			mimeType:         r.MimeType,
			size:             r.Size,
			comment:          r.Comment,
			versionCreatedAt: r.VersionCreatedAt,
			isCurrent:        true,
		})
	}
	return out, nil
}

// queryTagsAndFields populates b.tagsMap and b.fieldsMap.
func (b *buildCtx) queryTagsAndFields(ctx context.Context) error {
	projectID := b.p.Config.ProjectID
	workspaceID := b.p.Config.WorkspaceID

	tagRows, err := b.p.Queries.GetAssetTagsForProject(ctx, dbgen.GetAssetTagsForProjectParams{
		ProjectID:   &projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("export: query tags: %w", err)
	}
	b.tagsMap = make(map[string][]string, len(tagRows))
	for _, tr := range tagRows {
		b.tagsMap[tr.AssetID] = append(b.tagsMap[tr.AssetID], tr.TagName)
	}

	fieldRows, err := b.p.Queries.GetFieldValuesForProject(ctx, dbgen.GetFieldValuesForProjectParams{
		ProjectID:   &projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("export: query fields: %w", err)
	}
	b.fieldsMap = make(map[string][]ManifestFieldVal, len(fieldRows))
	for _, fr := range fieldRows {
		var val any
		switch fr.FieldType {
		case "boolean":
			val = fr.ValueBoolean != nil && *fr.ValueBoolean == 1
		case "number":
			val = fr.ValueNumber
		case "date":
			val = fr.ValueDate
		default:
			val = fr.ValueText
		}
		b.fieldsMap[fr.AssetID] = append(b.fieldsMap[fr.AssetID], ManifestFieldVal{
			Name:  fr.FieldName,
			Type:  fr.FieldType,
			Value: val,
		})
	}
	return nil
}

// queryVariants populates b.variantsMap when IncludeVariants is set.
func (b *buildCtx) queryVariants(ctx context.Context, versionIDs []string) error {
	if !b.p.Config.IncludeVariants || len(versionIDs) == 0 {
		b.variantsMap = map[string][]dbgen.Variant{}
		return nil
	}
	variants, err := queryVariantsForVersionIDs(ctx, b.p.SQLite, versionIDs, b.p.Config.WorkspaceID)
	if err != nil {
		return fmt.Errorf("export: query variants: %w", err)
	}
	b.variantsMap = make(map[string][]dbgen.Variant, len(variants))
	for _, v := range variants {
		b.variantsMap[v.AssetVersionID] = append(b.variantsMap[v.AssetVersionID], v)
	}
	return nil
}

// resolveItems builds the flat list of items to write into the ZIP, with paths
// already deduplicated by PathRegistry.
func (b *buildCtx) resolveItems(avRows []assetVersionRow) []itemInfo {
	reg := NewPathRegistry()
	includeVersionSuffix := b.p.Config.Versions == "all"
	items := make([]itemInfo, 0, len(avRows))

	for _, r := range avRows {
		stem, ext := splitFilename(r.originalFilename)
		var versionSuffix string
		if includeVersionSuffix {
			versionSuffix = "__v" + itoa(int(r.versionNum))
		}
		folderName := ""
		if r.folderName != nil {
			folderName = *r.folderName
		}
		rp := reg.Resolve(b.p.Project.Name, folderName, stem, ext, versionSuffix, "")
		items = append(items, itemInfo{row: r, resolvedPath: rp})

		if b.p.Config.IncludeVariants {
			for i := range b.variantsMap[r.versionID] {
				v := b.variantsMap[r.versionID][i]
				title := v.Type
				if v.Title != nil && *v.Title != "" {
					title = *v.Title
				}
				variantSlug := "__" + slugify(title)
				vrp := reg.Resolve(b.p.Project.Name, folderName, stem, ext, versionSuffix, variantSlug)
				items = append(items, itemInfo{row: r, variant: &b.variantsMap[r.versionID][i], resolvedPath: vrp})
			}
		}
	}
	return items
}

// writeItem copies one item from storage into the ZIP writer.
// Returns (bytesWritten, skipped, error). A fatal zip error is returned as err;
// storage/copy errors are warned and reported as skipped.
func (b *buildCtx) writeItem(
	ctx context.Context,
	item itemInfo,
	zw *zip.Writer,
) (written int64, skipped bool, err error) {
	r := item.row

	var storageKey, contentHash string
	var rkey string

	if item.variant != nil {
		storageKey = item.variant.StorageKey
		rkey = remoteKey(r.assetID, r.versionNum, item.variant.ID)
	} else {
		storageKey = r.storageKey
		contentHash = r.contentHash
		rkey = remoteKey(r.assetID, r.versionNum, "")
	}

	if prevHash, ok := b.remoteHashes[rkey]; ok && contentHash != "" && prevHash == contentHash {
		return 0, true, nil
	}

	rc, storageErr := b.p.Storage.Get(ctx, storageKey)
	if storageErr != nil {
		slog.WarnContext(ctx, "export: storage get failed, skipping asset",
			"storage_key", storageKey,
			"error", storageErr)
		return 0, true, nil
	}

	w, zipErr := zw.Create(item.resolvedPath)
	if zipErr != nil {
		_ = rc.Close()
		return 0, false, fmt.Errorf("export: zip create entry: %w", zipErr)
	}

	n, copyErr := io.Copy(w, rc)
	_ = rc.Close()
	if copyErr != nil {
		slog.WarnContext(ctx, "export: copy to zip failed", "error", copyErr)
		return 0, true, nil
	}
	return n, false, nil
}

// appendToManifest records an item in b.manifest after it has been written.
func (b *buildCtx) appendToManifest(item itemInfo, size int64) {
	r := item.row

	if item.variant == nil {
		if _, exists := b.assetIndex[r.assetID]; !exists {
			folderName := (*string)(nil)
			if r.folderName != nil {
				fn := *r.folderName
				folderName = &fn
			}
			b.manifest.Assets = append(b.manifest.Assets, ManifestAsset{
				ID:               r.assetID,
				OriginalFilename: r.originalFilename,
				Folder:           folderName,
				Tags:             b.tagsMap[r.assetID],
				CustomFields:     b.fieldsMap[r.assetID],
				Versions:         []ManifestVersion{},
			})
			b.assetIndex[r.assetID] = len(b.manifest.Assets) - 1
		}
		idx := b.assetIndex[r.assetID]
		versionCreatedAt, _ := time.Parse("2006-01-02 15:04:05", r.versionCreatedAt)
		b.manifest.Assets[idx].Versions = append(b.manifest.Assets[idx].Versions, ManifestVersion{
			VersionNum:  r.versionNum,
			IsCurrent:   r.isCurrent,
			ContentHash: r.contentHash,
			Size:        size,
			MimeType:    r.mimeType,
			Comment:     r.comment,
			CreatedAt:   versionCreatedAt,
			Path:        item.resolvedPath,
			Variants:    []ManifestVariant{},
		})
		return
	}

	idx, ok := b.assetIndex[r.assetID]
	if !ok {
		return
	}
	for vi := range b.manifest.Assets[idx].Versions {
		if b.manifest.Assets[idx].Versions[vi].VersionNum == r.versionNum {
			title := ""
			if item.variant.Title != nil {
				title = *item.variant.Title
			}
			b.manifest.Assets[idx].Versions[vi].Variants = append(
				b.manifest.Assets[idx].Versions[vi].Variants,
				ManifestVariant{
					ID:    item.variant.ID,
					Type:  item.variant.Type,
					Title: title,
					Path:  item.resolvedPath,
					Size:  size,
				},
			)
			break
		}
	}
}

// newManifest constructs the Manifest header from build params.
func newManifest(p BuildParams) Manifest {
	return Manifest{
		DamaskExportVersion: "1",
		ExportedAt:          time.Now().UTC(),
		ExportConfigID:      p.Config.ID,
		ExportRunID:         p.Run.ID,
		Project: ManifestProject{
			ID:          p.Project.ID,
			Name:        p.Project.Name,
			Description: p.Project.Description,
			Color:       p.Project.Color,
		},
		Assets: []ManifestAsset{},
	}
}

// writeManifestToZip marshals b.manifest and writes it as manifest.json into zw.
func (b *buildCtx) writeManifestToZip(zw *zip.Writer) ([]byte, error) {
	manifestJSON, err := json.MarshalIndent(b.manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("export: marshal manifest: %w", err)
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("export: zip manifest entry: %w", err)
	}
	if _, writeErr := mw.Write(manifestJSON); writeErr != nil {
		return nil, fmt.Errorf("export: write manifest to zip: %w", writeErr)
	}
	if closeErr := zw.Close(); closeErr != nil {
		return nil, fmt.Errorf("export: close zip: %w", closeErr)
	}
	return manifestJSON, nil
}

// writeToDestination uploads the ZIP and sidecar manifest to the destination.
func (b *buildCtx) writeToDestination(ctx context.Context, f *os.File, manifestJSON []byte) error {
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
		return fmt.Errorf("export: seek temp file: %w", seekErr)
	}
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("export: stat temp file: %w", err)
	}

	zipRemotePath := slugify(b.p.Project.Name) + "__export.zip"
	err = b.p.Dest.Write(ctx, zipRemotePath, f, fi.Size(), "")
	if err != nil {
		return fmt.Errorf("export: write zip to destination: %w", err)
	}

	sidecarPath := slugify(b.p.Project.Name) + "__manifest.json"
	err = b.p.Dest.WriteManifest(ctx, sidecarPath, manifestJSON)
	if err != nil {
		return fmt.Errorf("export: write manifest to destination: %w", err)
	}
	return nil
}

// collectVersionIDs extracts the versionID from each row.
func collectVersionIDs(rows []assetVersionRow) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.versionID
	}
	return ids
}

// splitFilename splits a filename into stem and extension.
// "hero-shot.jpg" → ("hero-shot", ".jpg")
// "archive.tar.gz" → ("archive.tar", ".gz").
func splitFilename(name string) (stem, ext string) {
	ext = filepath.Ext(name)
	stem = strings.TrimSuffix(name, ext)
	return stem, ext
}

// queryVariantsForVersionIDs fetches ready variants for a set of version IDs.
// Uses raw SQL with IN clause since sqlc doesn't support slices properly.
func queryVariantsForVersionIDs(
	ctx context.Context,
	db *sql.DB,
	versionIDs []string,
	workspaceID string,
) ([]dbgen.Variant, error) {
	if len(versionIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(versionIDs))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf( //nolint:gosec // only placeholders
		`SELECT id, workspace_id, asset_version_id, type, storage_key, transform_params, size, status,
		        thumbnail_key, thumbnail_content_type, title, is_shared, created_at
		 FROM variants
		 WHERE asset_version_id IN (%s)
		   AND workspace_id = ?
		   AND status = 'ready'
		 ORDER BY asset_version_id, type, title`,
		placeholders,
	)
	args := make([]any, 0, len(versionIDs)+1)
	for _, id := range versionIDs {
		args = append(args, id)
	}
	args = append(args, workspaceID)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var variants []dbgen.Variant
	for rows.Next() {
		var v dbgen.Variant
		err = rows.Scan(
			&v.ID,
			&v.WorkspaceID,
			&v.AssetVersionID,
			&v.Type,
			&v.StorageKey,
			&v.TransformParams,
			&v.Size,
			&v.Status,
			&v.ThumbnailKey,
			&v.ThumbnailContentType,
			&v.Title,
			&v.IsShared,
			&v.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}
