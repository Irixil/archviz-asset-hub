// Package api contains HTTP handlers for the REST API.
package api

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"mime"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/service"
	"damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const maxPageSize = int64(50)

type AssetContributor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AssetDetailResponse embeds AssetResponse and adds contributor fields.
// Returned only by handleGetAsset; list endpoints keep returning AssetResponse.
type AssetDetailResponse struct {
	AssetResponse

	CreatedBy *AssetContributor  `json:"created_by"`
	Authors   []AssetContributor `json:"authors"`
}

type AssetResponse struct {
	ID                 string  `json:"id"`
	WorkspaceID        string  `json:"workspace_id"`
	ProjectID          *string `json:"project_id,omitempty"`
	FolderID           *string `json:"folder_id,omitempty"`
	DerivedFromAssetID *string `json:"derived_from_asset_id,omitempty"`
	OriginalFilename   string  `json:"original_filename"`
	// StorageKey is returned to authenticated workspace clients so the desktop
	// shell can map an asset to the same shared-storage root used by the server.
	// Public share responses deliberately leave it empty.
	StorageKey             string                  `json:"storage_key,omitempty"`
	MimeType               string                  `json:"mime_type"`
	Size                   int64                   `json:"size"`
	Width                  *int64                  `json:"width,omitempty"`
	Height                 *int64                  `json:"height,omitempty"`
	ThumbnailKey           *string                 `json:"thumbnail_key,omitempty"`
	ThumbnailContentType   *string                 `json:"thumbnail_content_type,omitempty"`
	Metadata               *string                 `json:"metadata,omitempty"`
	Tags                   []string                `json:"tags"`
	ClassificationStatus   string                  `json:"classification_status"`
	ClassificationCategory *string                 `json:"classification_category,omitempty"`
	VersionCount           int64                   `json:"version_count"`
	VariantCount           int64                   `json:"variant_count"`
	VariantsRebuilding     bool                    `json:"variants_rebuilding"`
	SharedVariants         []SharedVariantResponse `json:"shared_variants,omitempty"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
	// DuplicateOf is set only on the POST /assets response, when the
	// workspace's duplicate_detection_mode is "warn" and a content-hash match
	// was found elsewhere in the workspace. Omitted entirely (not null) when
	// there is no match, and never populated by GET/list responses.
	DuplicateOf *DuplicateOfResponse `json:"duplicate_of,omitempty"`
}

type AssetClassificationResponse struct {
	Status   string `json:"classification_status"`
	Category string `json:"classification_category"`
}

// DuplicateOfResponse describes the existing asset a just-uploaded file's
// content matches, per the workspace's duplicate_detection_mode.
type DuplicateOfResponse struct {
	AssetID          string    `json:"asset_id"`
	VersionID        string    `json:"version_id"`
	OriginalFilename string    `json:"original_filename"`
	ProjectID        *string   `json:"project_id,omitempty"`
	ThumbnailURL     *string   `json:"thumbnail_url,omitempty"`
	IsDeletedVersion bool      `json:"is_deleted_version"`
	StorageAvailable bool      `json:"storage_available"`
	CreatedAt        time.Time `json:"created_at"`
}

func duplicateOfToResponse(d *assetio.DuplicateMatch) *DuplicateOfResponse {
	if d == nil {
		return nil
	}
	var thumbURL *string
	if d.ThumbnailKey != nil {
		u := versionThumbURL(d.AssetID, d.VersionID)
		thumbURL = &u
	}
	return &DuplicateOfResponse{
		AssetID:          d.AssetID,
		VersionID:        d.VersionID,
		OriginalFilename: d.OriginalFilename,
		ProjectID:        d.ProjectID,
		ThumbnailURL:     thumbURL,
		IsDeletedVersion: d.IsDeletedVersion,
		StorageAvailable: d.StorageAvailable,
		CreatedAt:        d.CreatedAt,
	}
}

type AssetListResponse struct {
	Assets              []AssetResponse      `json:"assets"`
	NextCursor          *string              `json:"next_cursor,omitempty"`
	Total               *int                 `json:"total,omitempty"`
	Similarity          *AssetSimilarityMeta `json:"similarity,omitempty"`
	SimilarToNotIndexed bool                 `json:"similar_to_not_indexed,omitempty"`
	SimilarToNoMatches  bool                 `json:"similar_to_no_matches,omitempty"`
}

type AssetSimilarityMeta struct {
	AnchorAssetID  string `json:"anchor_asset_id"`
	AnchorFilename string `json:"anchor_filename"`
	ResultCount    int    `json:"result_count"`
}

func assetToResponse(a dbgen.Asset) AssetResponse {
	return assetToResponseWithCount(a, nil, 0, 0, false)
}

func assetToResponseWithCount(
	a dbgen.Asset,
	tags []string,
	versionCount int64,
	variantCount int64,
	variantsRebuilding bool,
) AssetResponse {
	if tags == nil {
		tags = []string{}
	}
	classificationStatus, classificationCategory := assetClassification(
		a.OriginalFilename,
		a.MimeType,
		tags,
	)
	return AssetResponse{
		ID:                     a.ID,
		WorkspaceID:            a.WorkspaceID,
		ProjectID:              a.ProjectID,
		FolderID:               a.FolderID,
		DerivedFromAssetID:     a.DerivedFromAssetID,
		OriginalFilename:       a.OriginalFilename,
		StorageKey:             a.StorageKey,
		MimeType:               a.MimeType,
		Size:                   a.Size,
		Width:                  a.Width,
		Height:                 a.Height,
		ThumbnailKey:           a.ThumbnailKey,
		Metadata:               a.Metadata,
		Tags:                   tags,
		ClassificationStatus:   classificationStatus,
		ClassificationCategory: classificationCategory,
		VersionCount:           versionCount,
		VariantCount:           variantCount,
		VariantsRebuilding:     variantsRebuilding,
		CreatedAt:              a.CreatedAt,
		UpdatedAt:              a.UpdatedAt,
	}
}

func assetClassification(filename, mimeType string, tags []string) (string, *string) {
	category := service.PrimaryAssetCategory(filename, mimeType)
	if category == "" {
		return "manual_required", nil
	}
	for _, tag := range tags {
		if strings.EqualFold(tag, category) {
			return "classified", &category
		}
	}
	return "needs_review", &category
}

func assetToDetailResponse(
	base AssetResponse,
	createdBy *AssetContributor,
	authors []AssetContributor,
) AssetDetailResponse {
	return AssetDetailResponse{AssetResponse: base, CreatedBy: createdBy, Authors: authors}
}

// dtoToDBAsset converts a service.AssetDTO back to a dbgen.Asset for use
// with response builders that have not yet been migrated to the service layer.
func dtoToDBAsset(d *service.AssetDTO) dbgen.Asset {
	return dbgen.Asset{
		ID:                 d.ID,
		WorkspaceID:        d.WorkspaceID,
		ProjectID:          d.ProjectID,
		FolderID:           d.FolderID,
		DerivedFromAssetID: d.DerivedFromAssetID,
		OriginalFilename:   d.OriginalFilename,
		StorageKey:         d.StorageKey,
		MimeType:           d.MimeType,
		Size:               d.Size,
		Width:              d.Width,
		Height:             d.Height,
		ThumbnailKey:       d.ThumbnailKey,
		Metadata:           d.Metadata,
		CurrentVersionID:   d.CurrentVersionID,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
}

// handleUploadAsset uploads a file and creates a new asset.
//
// @Summary Upload an asset
// @Description Uploads a file as a new asset in the workspace. The request must be a multipart form with a <code>file</code> field. Optional form fields: <ul> <li><strong>project_id</strong> — assign the asset to a project on creation.</li> <li><strong>folder_id</strong> — assign the asset to a folder on creation.</li> </ul> On success a thumbnail generation job is enqueued automatically. An <code>asset_created</code> audit event is written and custom fields are inherited from the project if applicable.
// @Tags Assets
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Param project_id formData string false "Project ID"
// @Param folder_id formData string false "Folder ID"
// @Success 201 {object} AssetResponse
// @Failure 400 {object} ErrorResponse "file field is required"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/assets [post].
func (s *Server) handleUploadAsset(c fiber.Ctx) (err error) {
	claims := auth.GetClaims(c)
	var asset *service.AssetDTO

	ctx, rootSpan := telemetry.StartSpan(c.Context(), "api.assets.upload",
		attribute.String("damask.workspace_id", claims.WorkspaceID),
	)
	defer func() {
		telemetry.EndSpan(rootSpan, err)
		if err != nil {
			assetID := ""
			if asset != nil {
				assetID = asset.ID
			}
			rootSpan.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(
				ctx,
				"uploaded asset",
				"workspace_id",
				claims.WorkspaceID,
				"asset_id",
				assetID,
				apiErrorKey,
				err,
			)
		}
	}()

	// Demo upload cap enforcement (DM-4.2) — no-op in non-demo builds
	var blocked bool
	blocked, err = s.checkDemoUploadCap(c, claims)
	if err != nil || blocked {
		return err
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "file field is required")
	}

	if err = s.storageSvc.CheckLimit(c.Context(), claims.WorkspaceID, fh.Size); err != nil {
		if errors.Is(err, service.ErrStorageLimitReached) {
			return c.Status(fiber.StatusInsufficientStorage).JSON(fiber.Map{
				apiErrorKey: "storage_limit_reached",
				"message":   "Workspace storage limit reached. Delete assets or contact your administrator.",
			})
		}
		return ErrorStatusResponse(c, err)
	}

	f, err := fh.Open()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "cannot open uploaded file")
	}
	defer f.Close()

	var uploadProjectID *string
	if pid := c.FormValue("project_id"); pid != "" {
		uploadProjectID = &pid
		rootSpan.SetAttributes(attribute.String("damask.project_id", pid))
	}

	var uploadFolderID *string
	if fid := c.FormValue("folder_id"); fid != "" {
		uploadFolderID = &fid
		rootSpan.SetAttributes(attribute.String("damask.folder_id", fid))
	}

	asset, err = s.upload.Ingest(c.Context(), claims.WorkspaceID, f, service.UploadMeta{
		OriginalFilename: fh.Filename,
		ProjectID:        uploadProjectID,
		FolderID:         uploadFolderID,
		UserID:           claims.UserID,
		InheritFields:    s.newInheritProjectFieldsFunc(),
	})
	if err != nil {
		var dupErr *service.DuplicateConflictError
		if errors.As(err, &dupErr) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				apiErrorKey:    "duplicate_content",
				"duplicate_of": duplicateOfToResponse(&dupErr.Match),
			})
		}
		return ErrorStatusResponse(c, err)
	}

	tagDTOs, tagErr := s.tags.ListForAsset(c.Context(), asset.ID)
	if tagErr != nil {
		slog.WarnContext(ctx, "could not load automatic classification", "asset_id", asset.ID, apiErrorKey, tagErr)
	}
	tagNames := make([]string, len(tagDTOs))
	for i, tag := range tagDTOs {
		tagNames[i] = tag.Name
	}

	resp := assetToResponseWithCount(dtoToDBAsset(asset), tagNames, 0, 0, false)
	resp.DuplicateOf = duplicateOfToResponse(asset.DuplicateOf)
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// handleClassifyAsset retries the deterministic format-level classification.
//
// @Summary Retry asset classification
// @Description Applies the architecture library's format-level category tag to an existing asset.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} AssetClassificationResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "File type requires manual classification"
// @Router /api/v1/assets/{id}/classify [post].
func (s *Server) handleClassifyAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	category := service.PrimaryAssetCategory(asset.OriginalFilename, asset.MimeType)
	if category == "" {
		return errRes(c, fiber.StatusUnprocessableEntity, "file type requires manual classification")
	}
	if err = s.tags.UpsertForAsset(c.Context(), claims.WorkspaceID, assetID, category); err != nil {
		return ErrorStatusResponse(c, err)
	}
	_ = s.tags.TouchLastUsed(c.Context(), claims.WorkspaceID, category)

	return c.JSON(AssetClassificationResponse{
		Status:   "classified",
		Category: category,
	})
}

// handleListAssets lists assets in the workspace with filtering, sorting, and cursor pagination.
//
// @Summary List assets
// @Description Returns a paginated list of assets. The behaviour is controlled by query parameters: <ul> <li><strong>q</strong> — Full-text search across filenames and custom field text values.</li> <li><strong>tags</strong> — Comma-separated tag names. Returns only assets that have ALL listed tags (AND logic).</li> <li><strong>folder_id</strong> — Filter by folder. Use <code>root</code> to list assets with no folder in a project.</li> <li><strong>project_id</strong> — Filter by project (required when folder_id=root).</li> <li><strong>mime</strong> — Filter by MIME type prefix (e.g. <code>image/</code> or <code>video/mp4</code>).</li> <li><strong>sort</strong> — Sort order: <code>created_at_desc</code> (default), <code>created_at_asc</code>, <code>size_asc</code>, <code>size_desc</code>, <code>id_asc</code>, <code>id_desc</code>, <code>taken_at</code>, <code>taken_at_desc</code>.</li> <li><strong>limit</strong> — Page size, 1–100 (default 50).</li> <li><strong>cursor</strong> — Opaque cursor from the previous page's <code>next_cursor</code> field.</li> <li><strong>field[key]</strong> — Filter by custom field value, e.g. <code>field[status]=published</code>.</li> </ul>
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param q query string false "Full-text search query"
// @Param tags query string false "Comma-separated tag names (AND filter)"
// @Param folder_id query string false "Folder ID (use 'root' for unfoldered assets in a project)"
// @Param project_id query string false "Project ID"
// @Param mime query string false "MIME type prefix filter"
// @Param sort query string false "Sort order"
// @Param limit query int false "Page size (1-100, default 50)"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} AssetListResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/assets [get].
func (s *Server) handleListAssets(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	_, span := telemetry.StartSpan(c.Context(), "parse.args")
	limit := maxPageSize
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 64); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	// Field value filter — handled separately (different pagination scheme).
	if hasFieldFilters(c) {
		return s.handleListAssetsByFields(c, claims.WorkspaceID, limit)
	}

	lp, err := s.parseListAssetsParams(c, claims.WorkspaceID, limit)
	if err != nil {
		return err
	}

	sim, err := s.parseSimilarityFilter(c, claims.WorkspaceID, &lp)
	if err != nil {
		return err
	}
	span.End()

	ctx, span := telemetry.StartSpan(c.Context(), "fetch")
	assets, err := s.assets.List(c.Context(), lp)
	if err != nil {
		slog.ErrorContext(ctx, "could not list assets", apiErrorKey, err)
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}
	telemetry.EndSpan(span, err)

	similarAssets := assets
	resultCount := len(assets)
	if sim.Meta != nil {
		sim.Meta.ResultCount = resultCount
		if sim.Anchor != nil && lp.CursorID == "" {
			deduped := make([]*service.AssetDTO, 0, len(assets)+1)
			deduped = append(deduped, sim.Anchor)
			for _, a := range assets {
				if a.ID == sim.Anchor.ID {
					continue
				}
				deduped = append(deduped, a)
			}
			assets = deduped
		}
	}

	_, span = telemetry.StartSpan(c.Context(), "batch.counts")
	ids := make([]string, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}
	versionCounts, _ := s.assets.BatchVersionCounts(c.Context(), ids)
	variantCounts, _ := s.assets.BatchVariantCounts(c.Context(), ids)
	tagsByAsset, _ := s.tags.BatchTagsForAssets(c.Context(), ids)
	span.End()

	response := buildAssetListResponseFromDTOs(assets, limit, lp.SortField, versionCounts, variantCounts, tagsByAsset)
	if sim.Meta != nil {
		total := resultCount
		response.NextCursor = nextCursorFor(similarAssets, limit, lp.SortField)
		response.Total = &total
		response.Similarity = sim.Meta
		response.SimilarToNotIndexed = sim.NotIndexed
		response.SimilarToNoMatches = sim.NoMatches
	}
	return c.JSON(response)
}

func (s *Server) parseListAssetsParams(c fiber.Ctx, workspaceID string, limit int64) (service.ListAssetsParams, error) {
	lp := service.ListAssetsParams{
		WorkspaceID: workspaceID,
		Limit:       limit,
	}

	if q := c.Query("q"); q != "" {
		lp.SearchQuery = q
	}

	if tagsParam := c.Query("tags"); tagsParam != "" {
		for t := range strings.SplitSeq(tagsParam, ",") {
			lp.TagNames = append(lp.TagNames, strings.TrimSpace(strings.ToLower(t)))
		}
	}

	folderID, projectID, isRoot, err := parseFolderFilter(c.Query("folder_id"), c.Query("project_id"))
	if err != nil {
		return lp, errRes(c, fiber.StatusBadRequest, err.Error())
	}
	lp.FolderID = folderID
	lp.ProjectID = projectID
	lp.FolderIsRoot = isRoot

	if cid := c.Query("collection_id"); cid != "" {
		lp.CollectionID = &cid
	}

	if mime := c.Query("mime"); mime != "" {
		lp.MimePrefix = &mime
	}

	switch c.Query("sort") {
	case "size_asc":
		lp.SortField = sortFieldSize
	case "size_desc":
		lp.SortField = sortFieldSize
		lp.SortDesc = true
	case "id_asc":
		lp.SortField = "id"
	case "id_desc":
		lp.SortField = "id"
		lp.SortDesc = true
	case "created_at_asc":
		lp.SortField = "created_at_asc"
	case sortFieldTakenAt:
		lp.SortField = sortFieldTakenAt
	case "taken_at_desc":
		lp.SortField = sortFieldTakenAt
		lp.SortDesc = true
	default:
		lp.SortDesc = true
	}

	if cursor := c.Query("cursor"); cursor != "" {
		if cv, cvErr := decodeCursor(cursor); cvErr == nil {
			lp.CursorField = cv.Field
			lp.CursorValue = cv.Value
			lp.CursorID = cv.ID
		}
	}

	return lp, nil
}

type similarityFilterResult struct {
	Meta       *AssetSimilarityMeta
	Anchor     *service.AssetDTO
	NotIndexed bool
	NoMatches  bool
}

func (s *Server) parseSimilarityFilter(
	c fiber.Ctx,
	workspaceID string,
	lp *service.ListAssetsParams,
) (similarityFilterResult, error) {
	similarTo := c.Query("similar_to")
	if similarTo == "" {
		return similarityFilterResult{}, nil
	}

	anchorDTO, err := s.assets.Get(c.Context(), workspaceID, similarTo)
	if err != nil {
		return similarityFilterResult{}, ErrorStatusResponse(c, err)
	}
	if !strings.HasPrefix(anchorDTO.MimeType, "image/") {
		return similarityFilterResult{}, c.Status(fiber.StatusUnprocessableEntity).
			JSON(fiber.Map{"error": "not_an_image"})
	}

	result := similarityFilterResult{
		Anchor: anchorDTO,
		Meta: &AssetSimilarityMeta{
			AnchorAssetID:  anchorDTO.ID,
			AnchorFilename: anchorDTO.OriginalFilename,
		},
	}

	if anchorDTO.CurrentVersionID == nil || s.visualSimilaritySvc == nil {
		lp.SimilarToIDs = []string{}
		result.NotIndexed = true
		return result, nil
	}

	similar, err := s.visualSimilaritySvc.FindSimilarEnriched(
		c.Context(),
		workspaceID,
		*anchorDTO.CurrentVersionID,
	)
	if err != nil {
		return similarityFilterResult{}, errRes(c, fiber.StatusInternalServerError, "could not find similar assets")
	}

	seen := map[string]struct{}{anchorDTO.ID: {}}
	lp.SimilarToIDs = make([]string, 0, len(similar))
	for _, item := range similar {
		if _, ok := seen[item.AssetID]; ok {
			continue
		}
		seen[item.AssetID] = struct{}{}
		lp.SimilarToIDs = append(lp.SimilarToIDs, item.AssetID)
	}
	if len(lp.SimilarToIDs) == 0 {
		result.NoMatches = true
	}
	return result, nil
}

// buildAssetListResponseFromDTOs builds an AssetListResponse from service.AssetDTO slice.
func buildAssetListResponseFromDTOs(
	assets []*service.AssetDTO,
	limit int64,
	sortField string,
	versionCounts, variantCounts map[string]int64,
	tagsByAsset map[string][]string,
) AssetListResponse {
	items := make([]AssetResponse, len(assets))
	for i, a := range assets {
		var vc, nVariants int64
		if versionCounts != nil {
			vc = versionCounts[a.ID]
		}
		if variantCounts != nil {
			nVariants = variantCounts[a.ID]
		}
		tags := tagsByAsset[a.ID]
		if tags == nil {
			tags = []string{}
		}
		items[i] = AssetResponse{
			ID:                   a.ID,
			WorkspaceID:          a.WorkspaceID,
			ProjectID:            a.ProjectID,
			FolderID:             a.FolderID,
			DerivedFromAssetID:   a.DerivedFromAssetID,
			OriginalFilename:     a.OriginalFilename,
			MimeType:             a.MimeType,
			Size:                 a.Size,
			Width:                a.Width,
			Height:               a.Height,
			ThumbnailKey:         a.ThumbnailKey,
			ThumbnailContentType: &a.ThumbnailContentType,
			CreatedAt:            a.CreatedAt,
			UpdatedAt:            a.UpdatedAt,
			Tags:                 tags,
			VersionCount:         vc,
			VariantCount:         nVariants,
		}
	}
	return AssetListResponse{Assets: items, NextCursor: nextCursorFor(assets, limit, sortField)}
}

func parseFolderFilter(folderParam, projectParam string) (folderID *string, projectID *string, isRoot bool, err error) {
	switch folderParam {
	case "":
		if projectParam != "" {
			return nil, &projectParam, false, nil
		}
		return nil, nil, false, nil
	case apiFolderRoot:
		if projectParam == "" {
			return nil, nil, false, errors.New("project_id is required when using folder_id=root")
		}
		return nil, &projectParam, true, nil
	default:
		return &folderParam, nil, false, nil
	}
}

func nextCursorFor(assets []*service.AssetDTO, limit int64, sortField string) *string {
	if int64(len(assets)) != limit || len(assets) == 0 {
		return nil
	}
	last := assets[len(assets)-1]
	var cv cursorVal
	cv.ID = last.ID
	switch sortField {
	case sortFieldSize:
		cv.Field = sortFieldSize
		cv.Value = strconv.FormatInt(last.Size, 10)
	case "id":
		cv.Field = "id"
		cv.Value = last.ID
	default:
		cv.Field = "created_at"
		cv.Value = last.CreatedAt.UTC().Format("2006-01-02 15:04:05")
	}
	encoded := encodeCursor(cv)
	return &encoded
}

type cursorVal struct {
	Field string // apiCreatedAtField, "size", or "id"
	Value string // stringified sort-field value
	ID    string // asset UUID tiebreaker
}

func encodeCursor(v cursorVal) string {
	raw := v.Field + "|" + v.Value + "|" + v.ID
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (cursorVal, error) {
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return cursorVal{}, err
	}
	parts := strings.SplitN(string(b), "|", 3)
	return cursorVal{Field: parts[0], Value: parts[1], ID: parts[2]}, nil
}

// handleGetComments returns all share comments on an asset.
//
// @Summary Get asset comments
// @Description Returns all public share comments that have been posted on this asset across all shares.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {array} CommentResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/comments [get].
func (s *Server) handleGetComments(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	dtos, err := s.assets.GetComments(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]CommentResponse, len(dtos))
	for i, d := range dtos {
		out[i] = CommentResponse{
			ID:          d.ID,
			ShareID:     d.ShareID,
			AssetID:     d.AssetID,
			AuthorName:  d.AuthorName,
			AuthorEmail: d.AuthorEmail,
			Body:        d.Body,
			CreatedAt:   d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return c.JSON(out)
}

// handleGetAsset returns a single asset by ID.
//
// @Summary Get an asset
// @Description Returns the full asset record including tags, version count, and a flag indicating whether variants are currently being rebuilt.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id} [get].
func (s *Server) handleGetAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	dto, err := s.assets.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	tagDTOs, _ := s.tags.ListForAsset(c.Context(), id)
	tagNames := make([]string, len(tagDTOs))
	for i, t := range tagDTOs {
		tagNames[i] = t.Name
	}

	versionCount, _ := s.assets.CountVersionsByAsset(c.Context(), id)

	variantsRebuilding := false
	if dto.CurrentVersionID != nil {
		rebuilding, _ := s.assets.IsRebuildingVariants(c.Context(), *dto.CurrentVersionID)
		variantsRebuilding = rebuilding
	}

	variantCount, _ := s.assets.CountVariantsByCurrentVersion(c.Context(), id)

	// Reconstruct a dbgen.Asset from the DTO to reuse the existing response builder.
	asset := dtoToDBAsset(dto)
	base := assetToResponseWithCount(asset, tagNames, versionCount, variantCount, variantsRebuilding)

	createdBy := s.resolveAssetCreator(c, id)
	authors := s.resolveAssetAuthors(c, id)

	return c.JSON(assetToDetailResponse(base, createdBy, authors))
}

// handleGetAssetFile streams the current version of an asset file.
//
// @Summary Download asset file
// @Description Streams the raw file bytes of the asset's current version. The response Content-Type matches the asset's MIME type and Content-Disposition is set to <code>inline</code> with the original filename. An <code>asset_downloaded</code> audit event is recorded (browser image prefetch requests are excluded).
// @Tags Assets
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset or file not found"
// @Router /api/v1/assets/{id}/file [get].
func (s *Server) handleGetAssetFile(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	assetDTO, err := s.assets.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	version, err := s.versions.GetCurrentByAsset(c.Context(), id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if setCacheHeaders(c, version.ContentHash, version.CreatedAt, false) {
		return nil
	}

	_, storageSpan := telemetry.StartSpan(c.Context(), "api.assets.storage_get",
		attribute.String("damask.asset_id", id),
		attribute.String("damask.storage.key", version.StorageKey),
	)
	rc, err := s.storage.Get(c.Context(), version.StorageKey)
	telemetry.EndSpan(storageSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "file not found")
	}

	if !audit.IsBrowserPrefetch(c.Get("Sec-Fetch-Dest")) {
		s.assets.WriteAssetDownloadedAsync(claims.WorkspaceID, assetDTO.ID, claims.UserID)
	}

	c.Set("Content-Type", assetDTO.MimeType)
	c.Set(
		"Content-Disposition",
		mime.FormatMediaType("inline", map[string]string{apiFilenameKey: assetDTO.OriginalFilename}),
	)
	if version.Size > 0 {
		c.Set("Content-Length", strconv.FormatInt(version.Size, 10))
	}
	return c.SendStream(rc)
}

// handleGetAssetThumb serves the asset thumbnail as a JPEG image.
//
// @Summary Get asset thumbnail
// @Description Streams the JPEG thumbnail for the asset. Thumbnails are generated asynchronously after upload; returns 404 if generation has not yet completed.
// @Tags Assets
// @Produce image/jpeg
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {file} binary
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found or thumbnail not ready"
// @Router /api/v1/assets/{id}/thumb [get].
func (s *Server) handleGetAssetThumb(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	dto, err := s.assets.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if dto.ThumbnailKey == nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not ready")
	}

	thumbETag := dto.ID + "_" + strconv.FormatInt(dto.UpdatedAt.Unix(), 10)
	if setCacheHeaders(c, thumbETag, dto.UpdatedAt, false) {
		return nil
	}

	_, storageSpan := telemetry.StartSpan(c.Context(), "api.assets.thumbnail_storage_get",
		attribute.String("damask.asset_id", id),
		attribute.String("damask.storage.key", *dto.ThumbnailKey),
	)
	rc, err := s.storage.Get(c.Context(), *dto.ThumbnailKey)
	telemetry.EndSpan(storageSpan, err)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "thumbnail not found")
	}

	ct := dto.ThumbnailContentType
	if ct == "" {
		ct = contentTypeImageJPEG
	}
	c.Set("Content-Type", ct)
	return c.SendStream(rc)
}

// handleRegenerateThumbnail re-enqueues a version_thumbnail job for the asset's current version.
func (s *Server) handleRegenerateThumbnail(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	slog.DebugContext(c.Context(), "regenerating thumbnail", "workspace_id", claims.WorkspaceID, "asset_id", assetID)

	jobIDs, err := s.assets.RegenerateThumbnail(c.Context(), claims.WorkspaceID, []string{assetID})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(CreateVariantResponse{
		JobID:   jobIDs[0],
		Status:  apiStatusPending,
		Message: "thumbnail regeneration queued",
	})
}

// handleDeleteAsset permanently deletes an asset and its stored files.
//
// @Summary Delete an asset
// @Description Permanently deletes the asset record, its storage file, and its thumbnail. All associated variants, versions, tags, and field values are also removed via cascade. This action cannot be undone.
// @Tags Assets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id} [delete].
func (s *Server) handleDeleteAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.assets.HardDelete(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// handleBulkTag adds a tag to multiple assets at once.
//
// @Summary Bulk tag assets
// @Description Adds a single tag to all specified assets. If the tag does not exist in the workspace it is created automatically. Assets not found in the workspace are silently skipped.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkTagRequest true "Asset IDs and tag name"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/bulk/tag [post].
func (s *Server) handleBulkTag(c fiber.Ctx) error {
	body, ok := decodeAndValidate(c, &BulkTagRequest{})
	if !ok {
		return nil
	}
	claims := auth.GetClaims(c)
	if body.Mode == "remove" {
		if err := s.assets.BulkRemoveTag(c.Context(), claims.WorkspaceID, body.TagName, body.AssetIDs); err != nil {
			return ErrorStatusResponse(c, err)
		}
	} else {
		if err := s.assets.BulkSetTag(c.Context(), claims.WorkspaceID, body.TagName, body.AssetIDs); err != nil {
			return ErrorStatusResponse(c, err)
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleBulkProject assigns or unassigns a project for multiple assets.
//
// @Summary Bulk assign assets to a project
// @Description Assigns all listed assets to the given project. Set <code>project_id</code> to null to remove assets from their current project. Assets not found in the workspace are silently skipped.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkProjectRequest true "Asset IDs and optional project ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/bulk/project [post].
func (s *Server) handleBulkProject(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &BulkProjectRequest{})
	if !ok {
		return nil
	}

	// If project_id provided, verify it belongs to workspace.
	if body.ProjectID != nil {
		if _, err := s.projects.Get(c.Context(), claims.WorkspaceID, *body.ProjectID); err != nil {
			return ErrorStatusResponse(c, err)
		}
	}

	if err := s.assets.BulkMoveProject(c.Context(), claims.WorkspaceID, body.AssetIDs, body.ProjectID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleUpdateAssetFolder moves an asset to a different folder or project.
//
// @Summary Move asset to a folder
// @Description Updates the asset's <code>folder_id</code> and/or <code>project_id</code>. Set either field to null to remove the assignment. The target folder and project must belong to the same workspace.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body UpdateAssetFolderRequest true "New folder and/or project assignment"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/{id} [patch].
func (s *Server) handleUpdateAssetFolder(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateAssetFolderRequest{})
	if !ok {
		return nil
	}

	p := service.MoveAssetParams{FolderID: body.FolderID}
	updated, err := s.assets.Move(c.Context(), claims.WorkspaceID, id, p)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(assetToResponse(dtoToDBAsset(updated)))
}

// handleRenameAsset updates the display name of an asset.
//
// @Summary Rename an asset
// @Description Updates the asset's <code>original_filename</code>. The new name must include the correct file extension matching the asset's MIME type.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body RenameAssetRequest true "New filename"
// @Success 200 {object} AssetResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/{id}/rename [put].
func (s *Server) handleRenameAsset(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &RenameAssetRequest{})
	if !ok {
		return nil
	}

	updated, err := s.assets.Rename(c.Context(), claims.WorkspaceID, id, body.Name)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(assetToResponse(dtoToDBAsset(updated)))
}

// handleBulkDelete permanently deletes multiple assets.
//
// @Summary Bulk delete assets
// @Description Permanently deletes all listed assets, their storage files, thumbnails, variants, and associated data. Assets not found in the workspace are silently skipped. This action cannot be undone.
// @Tags Assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkDeleteRequest true "Asset IDs to delete"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/assets/bulk [delete].
func (s *Server) handleBulkDelete(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &BulkDeleteRequest{})
	if !ok {
		return nil
	}

	if err := s.assets.BulkHardDelete(c.Context(), claims.WorkspaceID, body.AssetIDs); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) resolveAssetCreator(c fiber.Ctx, assetID string) *AssetContributor {
	firstVer, err := s.versions.GetFirstByAsset(c.Context(), assetID)
	if err != nil || firstVer == nil || firstVer.CreatedBy == nil {
		return nil
	}
	cb := &AssetContributor{ID: *firstVer.CreatedBy}
	if u, uErr := s.users.GetByID(c.Context(), *firstVer.CreatedBy); uErr == nil {
		cb.Name = u.Name
	}
	return cb
}

func (s *Server) resolveAssetAuthors(c fiber.Ctx, assetID string) []AssetContributor {
	authors := []AssetContributor{}
	versions, err := s.versions.List(c.Context(), assetID)
	if err != nil {
		return authors
	}
	seen := make(map[string]struct{})
	for _, v := range versions {
		if v.CreatedBy == nil {
			continue
		}
		uid := *v.CreatedBy
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		name := ""
		if u, uErr := s.users.GetByID(c.Context(), uid); uErr == nil {
			name = u.Name
		}
		authors = append(authors, AssetContributor{ID: uid, Name: name})
	}
	return authors
}
