package api

import (
	"encoding/json"
	"log/slog"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/jobspec"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

// WorkspaceResponse is the public representation of a workspace.
type WorkspaceResponse struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	VersionRetentionCount    int64     `json:"version_retention_count"`
	EventLogRetentionDays    int64     `json:"event_log_retention_days"`
	DownloadLogRetentionDays int64     `json:"download_log_retention_days"`
	IconAssetID              *string   `json:"icon_asset_id,omitempty"`
	IconVersionID            *string   `json:"icon_version_id,omitempty"`
	ExifKeep                 bool      `json:"exif_keep"`
	ExifKeepGps              bool      `json:"exif_keep_gps"`
	LockedTaxonomy           bool      `json:"locked_taxonomy"`
	AutoTagEnabled           bool      `json:"auto_tag_enabled"`
	AutoTagMode              string    `json:"auto_tag_mode"`
	DuplicateDetectionMode   string    `json:"duplicate_detection_mode"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func workspaceDTOToResponse(w *service.WorkspaceDTO) WorkspaceResponse {
	return WorkspaceResponse{
		ID:                       w.ID,
		Name:                     w.Name,
		VersionRetentionCount:    w.VersionRetentionCount,
		EventLogRetentionDays:    w.EventLogRetentionDays,
		DownloadLogRetentionDays: w.DownloadLogRetentionDays,
		IconAssetID:              w.IconAssetID,
		IconVersionID:            w.IconVersionID,
		ExifKeep:                 w.ExifKeep,
		ExifKeepGps:              w.ExifKeepGps,
		LockedTaxonomy:           w.LockedTaxonomy,
		AutoTagEnabled:           w.AutoTagEnabled,
		AutoTagMode:              w.AutoTagMode,
		DuplicateDetectionMode:   w.DuplicateDetectionMode,
		CreatedAt:                w.CreatedAt,
		UpdatedAt:                w.UpdatedAt,
	}
}

type WorkspaceMeResponse struct {
	Workspace       WorkspaceResponse `json:"workspace"`
	User            UserResponse      `json:"user"`
	Role            auth.Role         `json:"role"`
	TotalAssetCount int64             `json:"total_asset_count"`
}

// handleWorkspaceMe returns the current user, their active workspace, and their role.
//
// @Summary Get current workspace context
// @Description Returns the workspace embedded in the auth token, the authenticated user's profile, and their membership role in that workspace. Use this endpoint on app startup to hydrate the session.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {object} WorkspaceMeResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Workspace or user not found"
// @Router /api/v1/workspace/me [get].
func (s *Server) handleWorkspaceMe(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	me, err := s.workspace.Me(c.Context(), claims.WorkspaceID, claims.UserID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(WorkspaceMeResponse{
		Workspace: workspaceDTOToResponse(me.Workspace),
		User: UserResponse{
			ID:        me.UserID,
			Email:     me.UserEmail,
			Name:      me.UserName,
			CreatedAt: me.UserCreatedAt,
		},
		Role:            auth.Role(me.Role),
		TotalAssetCount: me.TotalAssetCount,
	})
}

// handleCreateWorkspace creates a new workspace for the authenticated user.
//
// @Summary Create a workspace
// @Description Creates a new workspace and makes the authenticated user its owner. The user may belong to multiple workspaces; use <code>POST /api/v1/workspace/switch</code> to activate a different workspace in subsequent requests.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateWorkspaceRequest true "Workspace details"
// @Success 201 {object} AuthResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace [post].
func (s *Server) handleCreateWorkspace(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &CreateWorkspaceRequest{})
	if !ok {
		return nil
	}

	ws, err := s.users.CreateWorkspace(c.Context(), claims.UserID, req.Name)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	wr := workspaceDTOToResponse(ws)
	return c.Status(fiber.StatusCreated).JSON(AuthResponse{Workspace: &wr})
}

type WorkspaceWithRoleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	MemberCount int64  `json:"member_count"`
	AssetCount  int64  `json:"asset_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// handleListWorkspaces lists all workspaces the authenticated user belongs to.
//
// @Summary List workspaces
// @Description Returns every workspace the authenticated user is a member of, along with their role in each.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {array} WorkspaceWithRoleResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspaces [get].
func (s *Server) handleListWorkspaces(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.workspace.ListForUser(c.Context(), claims.UserID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]WorkspaceWithRoleResponse, len(rows))
	for i, r := range rows {
		result[i] = WorkspaceWithRoleResponse{
			ID:          r.ID,
			Name:        r.Name,
			Role:        r.Role,
			MemberCount: r.MemberCount,
			AssetCount:  r.AssetCount,
			CreatedAt:   r.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
		}
	}
	return c.JSON(result)
}

type SwitchWorkspaceResponse struct {
	Token     string            `json:"token"`
	Workspace WorkspaceResponse `json:"workspace"`
	Role      auth.Role         `json:"role"`
}

// handleSwitchWorkspace issues a new JWT scoped to a different workspace.
//
// @Summary Switch active workspace
// @Description Issues a fresh JWT scoped to the requested workspace and updates the <code>auth_token</code> cookie accordingly. All subsequent authenticated requests will operate in the new workspace context. The user must be a member of the target workspace.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body SwitchWorkspaceRequest true "Target workspace ID"
// @Success 200 {object} SwitchWorkspaceResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Not a member of this workspace"
// @Router /api/v1/workspace/switch [post].
func (s *Server) handleSwitchWorkspace(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &SwitchWorkspaceRequest{})
	if !ok {
		return nil
	}

	member, err := s.workspace.GetMember(c.Context(), req.WorkspaceID, claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	ws, err := s.workspace.Get(c.Context(), req.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	token, err := s.auth.CreateToken(claims.UserID, req.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.JSON(SwitchWorkspaceResponse{
		Token:     token,
		Workspace: workspaceDTOToResponse(ws),
		Role:      auth.Role(member.Role),
	})
}

// handleUpdateWorkspaceSettings updates workspace-level configuration.
//
// @Summary Update workspace settings
// @Description Updates the workspace's version retention policy and EXIF metadata settings. <ul> <li><strong>version_retention_count</strong>: Maximum number of asset versions to keep per asset. 0 = keep all.</li> <li><strong>exif_keep</strong>: Whether EXIF metadata is preserved on uploaded images.</li> <li><strong>exif_keep_gps</strong>: Whether GPS coordinates are preserved within EXIF data (requires exif_keep=true).</li> </ul>
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpdateWorkspaceSettingsRequest true "Settings to update"
// @Success 200 {object} WorkspaceResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace/settings [put].
func (s *Server) handleUpdateWorkspaceSettings(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &UpdateWorkspaceSettingsRequest{})
	if !ok {
		return nil
	}

	ws, err := s.workspace.Update(c.Context(), claims.WorkspaceID, service.UpdateWorkspaceParams{
		VersionRetentionCount:  &body.VersionRetentionCount,
		ExifKeep:               &body.ExifKeep,
		ExifKeepGps:            &body.ExifKeepGPS,
		LockedTaxonomy:         body.LockedTaxonomy,
		AutoTagEnabled:         body.AutoTagEnabled,
		AutoTagMode:            body.AutoTagMode,
		DuplicateDetectionMode: body.DuplicateDetectionMode,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(workspaceDTOToResponse(ws))
}

// triggerableJobs is the allowlist of job types that can be triggered via the
// generic POST /workspace/jobs/:type/trigger endpoint.
var triggerableJobs = map[string]string{
	"extract_exif":               queue.JobTypeExtractExif,
	"visual-similarity-backfill": queue.JobTypeVisualSimilarityBackfill,
}

// handleTriggerWorkspaceJob enqueues a workspace-wide background job.
//
// @Summary Trigger a workspace-wide background job
// @Description Enqueues a background job for the entire workspace. Currently supported job types: <ul> <li><strong>extract_exif</strong> — Backfills EXIF metadata (make, model, GPS, taken-at, etc.) for all image assets that do not yet have the <code>_exif_make</code> custom field populated.</li> </ul> Returns the number of assets enqueued for processing.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param type path string true "Job type (e.g. extract_exif)"
// @Success 202 {object} map[string]int "enqueued count"
// @Failure 400 {object} ErrorResponse "Unknown job type"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/jobs/{type}/trigger [post].
func (s *Server) handleTriggerWorkspaceJob(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	jobTypeKey := c.Params("type")

	jobType, ok := triggerableJobs[jobTypeKey]
	if !ok {
		return errRes(c, fiber.StatusBadRequest, "unknown job type")
	}

	switch jobType {
	case queue.JobTypeExtractExif:
		return s.triggerExtractExifBackfill(c, claims.WorkspaceID, claims.UserID)
	case queue.JobTypeVisualSimilarityBackfill:
		return s.triggerVisualSimilarityBackfill(c, claims.WorkspaceID)
	default:
		return errRes(c, fiber.StatusBadRequest, "unknown job type")
	}
}

func (s *Server) triggerExtractExifBackfill(c fiber.Ctx, workspaceID, userID string) error {
	pendingIDs, err := s.fields.ListAssetsMissingExif(c.Context(), workspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not determine pending assets")
	}

	if len(pendingIDs) == 0 {
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{apiEnqueuedKey: 0})
	}

	for _, assetID := range pendingIDs {
		payload, _ := json.Marshal(jobspec.ExtractExifPayload{
			AssetID:     assetID,
			WorkspaceID: workspaceID,
			UserID:      userID,
		})
		if _, err = s.queue.Enqueue(c.Context(), workspaceID, queue.JobTypeExtractExif, string(payload)); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not enqueue jobs")
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{apiEnqueuedKey: len(pendingIDs)})
}

func (s *Server) triggerVisualSimilarityBackfill(c fiber.Ctx, workspaceID string) error {
	_, span := telemetry.StartSpan(c.Context(), "api.workspace.trigger_visual_similarity_backfill",
		attribute.String("damask.workspace_id", workspaceID),
	)
	payload, _ := json.Marshal(map[string]string{"workspace_id": workspaceID})
	_, err := s.queue.Enqueue(c.Context(), workspaceID, queue.JobTypeVisualSimilarityBackfill, string(payload))
	telemetry.EndSpan(span, err)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue job")
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{apiEnqueuedKey: true})
}

type MemberResponse struct {
	UserID   string    `json:"user_id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// handleListMembers lists all members of the active workspace.
//
// @Summary List workspace members
// @Description Returns all members of the workspace embedded in the auth token, including their names, emails, roles, and join dates.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {array} MemberResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/members [get].
func (s *Server) handleListMembers(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	members, err := s.workspace.ListMembers(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]MemberResponse, len(members))
	for i, m := range members {
		result[i] = MemberResponse{
			UserID:   m.UserID,
			Name:     m.Name,
			Email:    m.Email,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		}
	}
	return c.JSON(result)
}

// handleRemoveMember removes a member from the workspace.
//
// @Summary Remove a workspace member
// @Description Removes a user from the active workspace. Safeguards: <ul> <li>A user cannot remove themselves.</li> <li>The last owner of a workspace cannot be removed.</li> </ul>
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID to remove"
// @Success 204
// @Failure 400 {object} ErrorResponse "Cannot remove yourself or the last owner"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/members/{userId} [delete].
func (s *Server) handleRemoveMember(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	targetUserID := c.Params("userId")

	if err := s.workspace.RemoveMember(c.Context(), claims.WorkspaceID, claims.UserID, targetUserID); err != nil {
		if isInvalidInput(err) {
			return errRes(c, fiber.StatusBadRequest, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}
	s.roleCache.invalidate(claims.WorkspaceID, targetUserID)
	return c.SendStatus(fiber.StatusNoContent)
}

// handleUpdateMemberRole changes a workspace member's role.
//
// @Summary Update a member's role
// @Description Changes the role (<code>owner</code>, <code>editor</code>, or <code>viewer</code>) of a workspace member. Safeguard: the last owner cannot demote themselves.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID whose role to change"
// @Param body body UpdateMemberRoleRequest true "New role"
// @Success 204
// @Failure 400 {object} ErrorResponse "Cannot demote the last owner"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace/members/{userId} [put].
func (s *Server) handleUpdateMemberRole(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	targetUserID := c.Params("userId")

	body, ok := decodeAndValidate(c, &UpdateMemberRoleRequest{})
	if !ok {
		return nil
	}

	if err := s.workspace.UpdateMemberRole(
		c.Context(),
		claims.WorkspaceID,
		claims.UserID,
		targetUserID,
		string(body.Role),
	); err != nil {
		if isInvalidInput(err) {
			return errRes(c, fiber.StatusBadRequest, err.Error())
		}
		return ErrorStatusResponse(c, err)
	}
	s.roleCache.invalidate(claims.WorkspaceID, targetUserID)
	return c.SendStatus(fiber.StatusNoContent)
}

type InviteResponse struct {
	ID          string    `json:"id"`
	InviteToken string    `json:"invite_token,omitempty"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// handleListInvites lists all pending invites for the workspace.
//
// @Summary List pending invites
// @Description Returns all workspace invites that have not yet been accepted and have not expired. The <code>invite_token</code> field is omitted from list responses for security — it is only returned once, when the invite is created.
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Success 200 {array} InviteResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/invites [get].
func (s *Server) handleListInvites(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	invites, err := s.workspace.ListInvites(c.Context(), claims.WorkspaceID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	result := make([]InviteResponse, len(invites))
	for i, inv := range invites {
		result[i] = InviteResponse{
			ID:        inv.ID,
			Email:     inv.Email,
			Role:      inv.Role,
			ExpiresAt: inv.ExpiresAt,
			CreatedAt: inv.CreatedAt,
		}
	}
	return c.JSON(result)
}

// handleDeleteInvite cancels a pending workspace invite.
//
// @Summary Delete an invite
// @Description Cancels a pending invite, preventing the recipient from accepting it. Has no effect on already-accepted invites (they are already marked as used).
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param inviteId path string true "Invite ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/workspace/invites/{inviteId} [delete].
func (s *Server) handleDeleteInvite(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	inviteID := c.Params("inviteId")

	if err := s.workspace.DeleteInvite(c.Context(), claims.WorkspaceID, inviteID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// handleCreateInvite generates a workspace invite for a new member.
//
// @Summary Create a workspace invite
// @Description Generates a time-limited invite token (7-day expiry) for the given email address. The <code>invite_token</code> in the response should be sent to the recipient; they redeem it via <code>POST /auth/invite/accept</code>. Only workspace owners may create invites.
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateInviteRequest true "Invite details (email and role)"
// @Success 201 {object} InviteResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Insufficient role — owner required"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/workspace/invites [post].
func (s *Server) handleCreateInvite(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &CreateInviteRequest{})
	if !ok {
		return nil
	}

	inv, err := s.workspace.CreateInvite(c.Context(), claims.WorkspaceID, claims.UserID, service.CreateInviteParams{
		Email: req.Email,
		Role:  req.Role,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if err = s.mailer.SendInvite(c.Context(), claims.WorkspaceID, inv.Email, inv.Role, inv.InviteToken); err != nil {
		slog.ErrorContext(c.Context(), "failed to send invitation mail", "error", err)
	}

	return c.Status(fiber.StatusCreated).JSON(InviteResponse{
		ID:          inv.ID,
		InviteToken: inv.InviteToken,
		Email:       inv.Email,
		Role:        inv.Role,
	})
}

// handleAcceptInvite redeems a workspace invite and creates the new user account.
//
// @Summary Accept a workspace invite
// @Description Redeems a one-time invite token: creates a new user account (name + password required) and adds them to the workspace at the role encoded in the invite. The invite token is marked as accepted and cannot be reused. Returns a JWT just like <code>POST /auth/register</code>.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body AcceptInviteRequest true "Invite token, name, and password"
// @Success 201 {object} AuthResponse
// @Failure 404 {object} ErrorResponse "Invalid or expired invite token"
// @Failure 409 {object} ErrorResponse "Email already in use"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /auth/invite/accept [post].
func (s *Server) handleAcceptInvite(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &AcceptInviteRequest{})
	if !ok {
		return nil
	}

	hash, err := s.bcryptHash(req.Password)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash password")
	}

	userID := uuid.New().String()
	result, err := s.workspace.AcceptInvite(c.Context(), service.AcceptInviteParams{
		Token:        req.Token,
		Name:         req.Name,
		PasswordHash: hash,
		UserID:       userID,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	if err = s.mailer.SendWelcome(c.Context(), result.UserEmail, result.UserName, result.WorkspaceID); err != nil {
		slog.ErrorContext(c.Context(), "failed to send welcome mail", "error", err)
	}

	if inviter, inviterErr := s.users.GetByID(c.Context(), result.InviterID); inviterErr == nil {
		if err = s.mailer.SendInviteAccepted(
			c.Context(),
			result.WorkspaceID,
			inviter.Email,
			result.UserName,
			result.UserEmail,
			result.InviteRole,
		); err != nil {
			slog.ErrorContext(c.Context(), "failed to send invite accepted mail", "error", err)
		}
	}

	token, err := s.auth.CreateToken(result.UserID, result.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(AuthResponse{
		Token: token,
		User: UserResponse{
			ID:        result.UserID,
			Email:     result.UserEmail,
			Name:      result.UserName,
			CreatedAt: result.UserCreatedAt,
		},
	})
}
