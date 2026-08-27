package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

type workspaceRepo struct {
	d *db.DB
}

// NewWorkspaceRepo returns a repository.WorkspaceRepository backed by sqlc-generated queries.
func NewWorkspaceRepo(d *db.DB) repository.WorkspaceRepository {
	return &workspaceRepo{d: d}
}

func (r *workspaceRepo) GetByID(ctx context.Context, id string) (repository.Workspace, error) {
	row, err := r.d.RQ.GetWorkspaceByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Workspace{}, apperr.ErrNotFound
		}
		return repository.Workspace{}, err
	}
	return toWorkspace(row), nil
}

// Update applies exif + version-retention settings.
func (r *workspaceRepo) Update(ctx context.Context, w repository.Workspace) (repository.Workspace, error) {
	if err := r.d.WQ.UpdateWorkspaceExifSettings(ctx, dbgen.UpdateWorkspaceExifSettingsParams{
		ID:          w.ID,
		ExifKeep:    boolToInt64(w.ExifKeep),
		ExifKeepGps: boolToInt64(w.ExifKeepGps),
	}); err != nil {
		return repository.Workspace{}, err
	}
	if err := r.d.WQ.UpdateWorkspaceVersionRetention(ctx, dbgen.UpdateWorkspaceVersionRetentionParams{
		ID:                    w.ID,
		VersionRetentionCount: w.VersionRetentionCount,
	}); err != nil {
		return repository.Workspace{}, err
	}
	return r.GetByID(ctx, w.ID)
}

func (r *workspaceRepo) CountAssets(ctx context.Context, workspaceID string) (int64, error) {
	return r.d.RQ.CountWorkspaceAssets(ctx, workspaceID)
}

func (r *workspaceRepo) GetAIProviderKey(ctx context.Context, workspaceID, providerName string) (string, error) {
	key, err := r.d.RQ.GetWorkspaceAIProviderKey(ctx, dbgen.GetWorkspaceAIProviderKeyParams{
		ProviderName: providerName,
		ID:           workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", apperr.ErrNotFound
		}
		return "", err
	}
	return key, nil
}

func (r *workspaceRepo) SetAIProviderKey(ctx context.Context, workspaceID, providerName, encKey string) error {
	return r.setOrClearAIProviderKey(ctx, workspaceID, providerName, &encKey)
}

func (r *workspaceRepo) ClearAIProviderKey(ctx context.Context, workspaceID, providerName string) error {
	return r.setOrClearAIProviderKey(ctx, workspaceID, providerName, nil)
}

func (r *workspaceRepo) setOrClearAIProviderKey(
	ctx context.Context,
	workspaceID, providerName string,
	encKey *string,
) error {
	switch ai.ProviderID(providerName) {
	case ai.ProviderOpenRouter:
		return r.d.WQ.SetWorkspaceOpenRouterKey(ctx, dbgen.SetWorkspaceOpenRouterKeyParams{
			OpenrouterApiKeyEnc: encKey,
			ID:                  workspaceID,
		})
	case ai.ProviderImageRouter:
		return r.d.WQ.SetWorkspaceImageRouterKey(ctx, dbgen.SetWorkspaceImageRouterKeyParams{
			ImagerouterApiKeyEnc: encKey,
			ID:                   workspaceID,
		})
	default:
		return fmt.Errorf("unknown ai provider %q: %w", providerName, apperr.ErrNotFound)
	}
}

func (r *workspaceRepo) GetMember(ctx context.Context, workspaceID, userID string) (repository.Member, error) {
	row, err := r.d.RQ.GetMember(ctx, dbgen.GetMemberParams{WorkspaceID: workspaceID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Member{}, apperr.ErrNotFound
		}
		return repository.Member{}, err
	}
	user, err := r.d.RQ.GetUserByID(ctx, userID)
	if err != nil {
		return repository.Member{}, err
	}
	return repository.Member{
		WorkspaceID: row.WorkspaceID,
		UserID:      row.UserID,
		Email:       user.Email,
		Name:        user.Name,
		Role:        row.Role,
		InvitedBy:   row.InvitedBy,
		JoinedAt:    row.CreatedAt,
	}, nil
}

func (r *workspaceRepo) ListMembers(ctx context.Context, workspaceID string) ([]repository.Member, error) {
	rows, err := r.d.RQ.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Member, len(rows))
	for i, row := range rows {
		out[i] = repository.Member{
			WorkspaceID: row.WorkspaceID,
			UserID:      row.UserID,
			Email:       row.Email,
			Name:        row.Name,
			Role:        row.Role,
			JoinedAt:    row.CreatedAt,
		}
	}
	return out, nil
}

func (r *workspaceRepo) CountMembers(ctx context.Context, workspaceID string) (int64, error) {
	return r.d.RQ.CountWorkspaceMembers(ctx, workspaceID)
}

func (r *workspaceRepo) CreateMember(ctx context.Context, m repository.Member) error {
	return r.d.WQ.CreateMember(ctx, dbgen.CreateMemberParams{
		WorkspaceID: m.WorkspaceID,
		UserID:      m.UserID,
		Role:        m.Role,
		InvitedBy:   m.InvitedBy,
	})
}

func (r *workspaceRepo) DeleteMember(ctx context.Context, workspaceID, userID string) error {
	return r.d.WQ.DeleteMember(ctx, dbgen.DeleteMemberParams{WorkspaceID: workspaceID, UserID: userID})
}

func (r *workspaceRepo) UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) error {
	return r.d.WQ.UpdateMemberRole(ctx, dbgen.UpdateMemberRoleParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
	})
}

func (r *workspaceRepo) CreateInvite(ctx context.Context, inv repository.Invite) (repository.Invite, error) {
	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}
	if inv.Token == "" {
		inv.Token = uuid.New().String()
	}
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	}
	row, err := r.d.WQ.CreateInvite(ctx, dbgen.CreateInviteParams{
		ID:          inv.ID,
		WorkspaceID: inv.WorkspaceID,
		Email:       inv.Email,
		Token:       inv.Token,
		Role:        inv.Role,
		InvitedBy:   inv.InvitedBy,
		ExpiresAt:   inv.ExpiresAt,
	})
	if err != nil {
		return repository.Invite{}, err
	}
	return toInvite(row), nil
}

func (r *workspaceRepo) ListPendingInvites(ctx context.Context, workspaceID string) ([]repository.Invite, error) {
	rows, err := r.d.RQ.ListPendingInvites(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.Invite, len(rows))
	for i, row := range rows {
		out[i] = toInvite(row)
	}
	return out, nil
}

func (r *workspaceRepo) GetInviteByToken(ctx context.Context, token string) (repository.Invite, error) {
	row, err := r.d.RQ.GetInviteByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Invite{}, apperr.ErrNotFound
		}
		return repository.Invite{}, err
	}
	return toInvite(row), nil
}

func (r *workspaceRepo) DeleteInvite(ctx context.Context, workspaceID, inviteID string) error {
	return r.d.WQ.DeleteInvite(ctx, dbgen.DeleteInviteParams{WorkspaceID: workspaceID, ID: inviteID})
}

func (r *workspaceRepo) AcceptInvite(ctx context.Context, inviteID string) error {
	return r.d.WQ.AcceptInvite(ctx, inviteID)
}

func (r *workspaceRepo) ListByUserID(ctx context.Context, userID string) ([]repository.WorkspaceWithRole, error) {
	rows, err := r.d.RQ.ListWorkspacesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.WorkspaceWithRole, len(rows))
	for i, row := range rows {
		out[i] = repository.WorkspaceWithRole{
			Workspace: repository.Workspace{
				ID:        row.ID,
				Name:      row.Name,
				CreatedAt: row.CreatedAt,
				UpdatedAt: row.UpdatedAt,
			},
			Role: row.Role,
		}
	}
	return out, nil
}

func toWorkspace(w dbgen.Workspace) repository.Workspace {
	return repository.Workspace{
		ID:                       w.ID,
		Name:                     w.Name,
		IngestToken:              w.IngestToken,
		VersionRetentionCount:    w.VersionRetentionCount,
		EventLogRetentionDays:    w.EventLogRetentionDays,
		DownloadLogRetentionDays: w.DownloadLogRetentionDays,
		IconAssetID:              w.IconAssetID,
		IconVersionID:            w.IconVersionID,
		ExifKeep:                 w.ExifKeep != 0,
		ExifKeepGps:              w.ExifKeepGps != 0,
		LockedTaxonomy:           w.LockedTaxonomy != 0,
		AutoTagEnabled:           w.AutoTagEnabled != 0,
		AutoTagMode:              w.AutoTagMode,
		DuplicateDetectionMode:   w.DuplicateDetectionMode,
		CreatedAt:                w.CreatedAt,
		UpdatedAt:                w.UpdatedAt,
	}
}

func (r *workspaceRepo) UpdateLockedTaxonomy(
	ctx context.Context,
	workspaceID string,
	locked bool,
) (repository.Workspace, error) {
	val := int64(0)
	if locked {
		val = 1
	}
	if err := r.d.WQ.UpdateWorkspaceLockedTaxonomy(ctx, dbgen.UpdateWorkspaceLockedTaxonomyParams{
		ID:             workspaceID,
		LockedTaxonomy: val,
	}); err != nil {
		return repository.Workspace{}, err
	}
	return r.GetByID(ctx, workspaceID)
}

func (r *workspaceRepo) UpdateAutoTagSettings(
	ctx context.Context,
	workspaceID string,
	enabled bool,
	mode string,
) (repository.Workspace, error) {
	val := int64(0)
	if enabled {
		val = 1
	}
	if err := r.d.WQ.UpdateWorkspaceAutoTagSettings(ctx, dbgen.UpdateWorkspaceAutoTagSettingsParams{
		ID:             workspaceID,
		AutoTagEnabled: val,
		AutoTagMode:    mode,
	}); err != nil {
		return repository.Workspace{}, err
	}
	return r.GetByID(ctx, workspaceID)
}

func (r *workspaceRepo) UpdateDuplicateDetectionMode(
	ctx context.Context,
	workspaceID string,
	mode string,
) (repository.Workspace, error) {
	if err := r.d.WQ.UpdateWorkspaceDuplicateDetectionMode(ctx, dbgen.UpdateWorkspaceDuplicateDetectionModeParams{
		ID:                     workspaceID,
		DuplicateDetectionMode: mode,
	}); err != nil {
		return repository.Workspace{}, err
	}
	return r.GetByID(ctx, workspaceID)
}

func (r *workspaceRepo) Create(ctx context.Context, w repository.Workspace) (repository.Workspace, error) {
	row, err := r.d.WQ.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID:   w.ID,
		Name: w.Name,
	})
	if err != nil {
		return repository.Workspace{}, err
	}
	return toWorkspace(row), nil
}

func (r *workspaceRepo) RunInTx(ctx context.Context, fn func(repository.WorkspaceRepository) error) error {
	tx, err := r.d.Writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.
	if err = fn(&workspaceRepo{d: r.d.WithTx(tx)}); err != nil {
		return err
	}
	return tx.Commit()
}

// RunRegistrationTx opens a single DB transaction and provides tx-scoped
// UserRepository and WorkspaceRepository to fn. Used only by UserService.Register.
func (r *workspaceRepo) RunRegistrationTx(
	ctx context.Context,
	fn func(context.Context, repository.UserRepository, repository.WorkspaceRepository) error,
) error {
	tx, err := r.d.Writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after read-only queries or commit.
	txDB := r.d.WithTx(tx)
	txUsers := &userRepo{d: txDB, writerDB: tx, readerDB: tx}
	txWorkspaces := &workspaceRepo{d: txDB}
	if err = fn(ctx, txUsers, txWorkspaces); err != nil {
		return err
	}
	return tx.Commit()
}

func toInvite(i dbgen.WorkspaceInvite) repository.Invite {
	return repository.Invite{
		ID:          i.ID,
		WorkspaceID: i.WorkspaceID,
		Email:       i.Email,
		Token:       i.Token,
		Role:        i.Role,
		InvitedBy:   i.InvitedBy,
		ExpiresAt:   i.ExpiresAt,
		AcceptedAt:  i.AcceptedAt,
		CreatedAt:   i.CreatedAt,
	}
}
