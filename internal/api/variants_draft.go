package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"

	"damask/server/internal/auth"
	"damask/server/internal/jobs"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/gofiber/fiber/v3"
)

// ---- Response types ----

type DraftGenerateResponse struct {
	DraftKey string `json:"draft_key"`
}

// ---- Request types ----

type CommitDraftRequest struct {
	Name string `json:"name"`
}

func (r *CommitDraftRequest) Valid(_ context.Context) map[string]string {
	return nil
}

// ---- Handlers ----

// @Summary Generate a draft variant
// @Tags Variants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 202 {object} DraftGenerateResponse
// @Router /api/v1/assets/{id}/variants/draft [post].
func (s *Server) handleGenerateDraft(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")

	asset, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if asset.CurrentVersionID == nil {
		return errRes(c, fiber.StatusUnprocessableEntity, "asset has no current version")
	}

	body, ok := decodeAndValidate(c, &CreateVariantRequest{})
	if !ok {
		return nil
	}

	nonce, err := generateNonce()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate nonce")
	}

	payload, _ := json.Marshal(jobs.CreateVariantDraftPayload{
		Nonce:       nonce,
		WorkspaceID: claims.WorkspaceID,
		UserID:      claims.UserID,
		AssetID:     assetID,
		Type:        body.Type,
		Params:      body.Params,
	})

	if _, err = s.queue.Enqueue(
		c.Context(),
		claims.WorkspaceID,
		queue.JobTypeCreateVariantDraft,
		string(payload),
	); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue draft job")
	}

	return c.Status(fiber.StatusAccepted).JSON(DraftGenerateResponse{DraftKey: nonce})
}

// handlePreviewDraft handles GET /api/v1/assets/:id/variants/draft/:nonce/preview.
func (s *Server) handlePreviewDraft(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	nonce := c.Params("nonce")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	meta, err := draftReadMeta(c.Context(), s.storage, claims.WorkspaceID, claims.UserID, nonce)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "draft not found or expired")
	}

	sk := draftScratchKey(claims.WorkspaceID, claims.UserID, nonce)
	rc, err := s.storage.Get(c.Context(), sk)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "draft not found or expired")
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to read draft")
	}

	c.Set("Cache-Control", "no-store")
	c.Set("Content-Type", meta.ContentType)
	return c.Send(data)
}

// handleCommitDraft handles POST /api/v1/assets/:id/variants/draft/:nonce/commit.
func (s *Server) handleCommitDraft(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	nonce := c.Params("nonce")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	meta, err := draftReadMeta(c.Context(), s.storage, claims.WorkspaceID, claims.UserID, nonce)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "draft not found or expired")
	}

	if meta.AssetID != assetID {
		return errRes(c, fiber.StatusBadRequest, "draft does not belong to this asset")
	}

	var nameReq CommitDraftRequest
	if bindErr := c.Bind().JSON(&nameReq); bindErr != nil {
		slog.WarnContext(c.Context(), "commit_draft: bind name request", "error", bindErr)
	}

	currentVer, err := s.versions.GetCurrentByAsset(c.Context(), assetID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load current version")
	}
	if meta.AssetVersionID != "" && meta.AssetVersionID != currentVer.ID {
		return errRes(
			c,
			fiber.StatusConflict,
			"asset has been updated since draft was created; please generate a new draft",
		)
	}

	paramsHash := draftParamsHash(meta.TransformParams)
	ext := transform.MimeToExt(meta.ContentType)
	permanentKey := storage.VersionedVariantKey(
		claims.WorkspaceID, assetID, currentVer.VersionNum,
		meta.VariantType, paramsHash, ext,
	)

	sk := draftScratchKey(claims.WorkspaceID, claims.UserID, nonce)
	if err = draftMoveKey(c.Context(), s.storage, sk, permanentKey); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "failed to commit draft")
	}

	paramsStr := meta.TransformParams
	var variantTitle *string
	if nameReq.Name != "" {
		variantTitle = &nameReq.Name
	}
	variant, err := s.variants.CommitDraft(c.Context(), service.CommitDraftParams{
		WorkspaceID:     claims.WorkspaceID,
		AssetID:         assetID,
		AssetVersionID:  currentVer.ID,
		VariantType:     meta.VariantType,
		StorageKey:      permanentKey,
		TransformParams: &paramsStr,
		ContentType:     meta.ContentType,
		Title:           variantTitle,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if thumbPayload, merr := json.Marshal(jobs.VariantThumbnailJobPayload{
		VariantID:   variant.ID,
		WorkspaceID: claims.WorkspaceID,
		AssetID:     assetID,
		StorageKey:  permanentKey,
		MimeType:    meta.ContentType,
	}); merr == nil {
		_, _ = s.queue.Enqueue(c.Context(), claims.WorkspaceID, queue.JobTypeVariantThumbnail, string(thumbPayload))
	}

	return c.Status(fiber.StatusCreated).JSON(variantDTOToResponse(assetID, variant))
}

// handleDiscardDraft handles DELETE /api/v1/assets/:id/variants/draft/:nonce.
func (s *Server) handleDiscardDraft(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	assetID := c.Params("id")
	nonce := c.Params("nonce")

	if _, err := s.assets.Get(c.Context(), claims.WorkspaceID, assetID); err != nil {
		return ErrorStatusResponse(c, err)
	}

	sk := draftScratchKey(claims.WorkspaceID, claims.UserID, nonce)
	mk := draftMetaKey(claims.WorkspaceID, claims.UserID, nonce)

	if err := s.storage.Delete(c.Context(), sk); err != nil && !errors.Is(err, storage.ErrNotFound) {
		slog.WarnContext(c.Context(), "discard_draft: delete output failed", "key", sk, "error", err)
	}
	if err := s.storage.Delete(c.Context(), mk); err != nil && !errors.Is(err, storage.ErrNotFound) {
		slog.WarnContext(c.Context(), "discard_draft: delete meta failed", "key", mk, "error", err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ---- Local helpers ----

func generateNonce() (string, error) {
	b := make([]byte, 8) //nolint:mnd // nonce is 16 hex chars.
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

const draftScratchPrefix = "scratch/"

func draftScratchKey(workspaceID, userID, nonce string) string {
	return fmt.Sprintf("%s%s/%s/%s", draftScratchPrefix, workspaceID, userID, nonce)
}

func draftMetaKey(workspaceID, userID, nonce string) string {
	return draftScratchKey(workspaceID, userID, nonce) + ".meta"
}

type draftMeta struct {
	AssetID         string `json:"asset_id"`
	AssetVersionID  string `json:"asset_version_id"`
	WorkspaceID     string `json:"workspace_id"`
	UserID          string `json:"user_id"`
	VariantType     string `json:"variant_type"`
	TransformParams string `json:"transform_params"`
	ContentType     string `json:"content_type"`
	CreatedAt       string `json:"created_at"`
}

func draftReadMeta(
	ctx context.Context, stor storage.Storage, workspaceID, userID, nonce string,
) (*draftMeta, error) {
	mk := draftMetaKey(workspaceID, userID, nonce)
	rc, err := stor.Get(ctx, mk)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var m draftMeta
	if err = json.NewDecoder(rc).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// draftMoveKey reads src into dst (Put), then deletes src.
// If dst already exists and src is gone, the move is treated as complete (idempotent).
func draftMoveKey(ctx context.Context, stor storage.Storage, src, dst string) error {
	rc, err := stor.Get(ctx, src)
	if err != nil {
		// src gone — check dst already present.
		dstRC, dstErr := stor.Get(ctx, dst)
		if dstErr == nil {
			_ = dstRC.Close()
			return nil
		}
		return fmt.Errorf("read scratch file: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read scratch data: %w", err)
	}
	if err = stor.Put(ctx, dst, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write permanent file: %w", err)
	}
	if delErr := stor.Delete(ctx, src); delErr != nil && !errors.Is(delErr, storage.ErrNotFound) {
		slog.WarnContext(
			ctx,
			"draftMoveKey: delete scratch src failed; will be purged nightly",
			"key",
			src,
			"error",
			delErr,
		)
	}
	return nil
}

// draftParamsHash returns the first 8 hex chars of SHA-256 of sorted-key JSON params.
// Returns "00000000" for empty/non-JSON input so the hash is stable across calls.
func draftParamsHash(paramsJSON string) string {
	if paramsJSON == "" {
		return "00000000"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(paramsJSON), &m); err != nil {
		h := sha256.Sum256([]byte(paramsJSON))
		return hex.EncodeToString(h[:])[:8]
	}
	b, _ := json.Marshal(draftSortedMap(m))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:8]
}

func draftSortedMap(m map[string]any) map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(m))
	for _, k := range keys {
		v := m[k]
		if sub, ok := v.(map[string]any); ok {
			v = draftSortedMap(sub)
		}
		out[k] = v
	}
	return out
}
