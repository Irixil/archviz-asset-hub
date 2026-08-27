// Package memory provides in-memory repository implementations for unit tests.
//
// Stub repos in this file are not yet exercised by service-layer tests.
// Philosophy: methods that return a domain value panic so that accidentally
// calling an unimplemented path produces an immediate, obvious test failure
// rather than a silent empty result. Methods that return a safe sentinel
// (ErrNotFound, false, 0, "") are left as-is and annotated. Echo-back Creates
// and RunInTx wrappers have correct semantics and are left unchanged.
// Promote a stub to its own file (with mapStore) when a service test needs it.
package memory

import (
	"context"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// TagRepo ------------------------------------------------------------------
// Stub: not exercised by service tests. Promote to mapStore when needed.

type TagRepo struct{}

func NewTagRepo() *TagRepo { return &TagRepo{} }

func (r *TagRepo) GetByName(_ context.Context, _, _ string) (repository.Tag, error) {
	panic("memory: TagRepo.GetByName not implemented")
}
func (r *TagRepo) List(_ context.Context, _ string, _ bool) ([]repository.Tag, error) {
	panic("memory: TagRepo.List not implemented")
}
func (r *TagRepo) Upsert(_ context.Context, _, _ string) (repository.Tag, error) {
	panic("memory: TagRepo.Upsert not implemented")
}
func (r *TagRepo) EnsureSystemTag(_ context.Context, _, _ string) error              { return nil }
func (r *TagRepo) UpdateMetadata(_ context.Context, _, _ string, _, _ *string) error { return nil }
func (r *TagRepo) Rename(_ context.Context, _, _, _ string) error                    { return nil }
func (r *TagRepo) Delete(_ context.Context, _ string, _ []string) error              { return nil }
func (r *TagRepo) ListForAsset(_ context.Context, _ string) ([]repository.Tag, error) {
	panic("memory: TagRepo.ListForAsset not implemented")
}
func (r *TagRepo) AddToAsset(_ context.Context, _, _ string) error { return nil }
func (r *TagRepo) BatchTagsForAssets(_ context.Context, _ []string) (map[string][]string, error) {
	// Returns empty map — callers treat missing tags as zero tags, so this is safe.
	return map[string][]string{}, nil
}
func (r *TagRepo) RemoveFromAsset(_ context.Context, _, _, _ string) error { return nil }
func (r *TagRepo) CountAssets(_ context.Context, _ string) (int64, error)  { return 0, nil }
func (r *TagRepo) ReassignAssets(_ context.Context, _, _ string) error     { return nil }
func (r *TagRepo) TouchLastUsed(_ context.Context, _, _ string) error      { return nil }
func (r *TagRepo) FindAssetBySystemTagInFolder(_ context.Context, _, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound // sentinel: "not found" is correct test-time behaviour
}
func (r *TagRepo) FindAssetBySystemTagInProject(_ context.Context, _, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound // sentinel
}
func (r *TagRepo) FindAssetBySystemTagInWorkspace(_ context.Context, _, _ string) (repository.Asset, error) {
	return repository.Asset{}, apperr.ErrNotFound // sentinel
}
func (r *TagRepo) RunInTx(_ context.Context, fn func(repository.TagRepository) error) error {
	return fn(r)
}

// VersionRepo --------------------------------------------------------------
// Stub: not exercised by service tests. Promote to mapStore when needed.

type VersionRepo struct{}

func NewVersionRepo() *VersionRepo { return &VersionRepo{} }

func (r *VersionRepo) GetByID(_ context.Context, _ string) (repository.AssetVersion, error) {
	panic("memory: VersionRepo.GetByID not implemented")
}
func (r *VersionRepo) GetByIDForWorkspace(_ context.Context, _, _ string) (repository.AssetVersion, error) {
	panic("memory: VersionRepo.GetByIDForWorkspace not implemented")
}
func (r *VersionRepo) GetCurrentByAsset(_ context.Context, _ string) (repository.AssetVersion, error) {
	panic("memory: VersionRepo.GetCurrentByAsset not implemented")
}
func (r *VersionRepo) GetFirstByAsset(_ context.Context, _ string) (repository.AssetVersion, error) {
	panic("memory: VersionRepo.GetFirstByAsset not implemented")
}
func (r *VersionRepo) ListByAsset(_ context.Context, _ string) ([]repository.AssetVersion, error) {
	panic("memory: VersionRepo.ListByAsset not implemented")
}
func (r *VersionRepo) Create(_ context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	return v, nil // echo-back: correct semantics for tests that only create, don't re-fetch
}
func (r *VersionRepo) SoftDelete(_ context.Context, _ string) error { return nil }
func (r *VersionRepo) Delete(_ context.Context, _ string) error     { return nil }
func (r *VersionRepo) CountByAsset(_ context.Context, _ string) (int64, error) {
	return 0, nil // sentinel: 0 is the correct answer when no versions are seeded
}
func (r *VersionRepo) IsReferencedAsCover(_ context.Context, _ string) (bool, error) {
	return false, nil // sentinel: safe default
}
func (r *VersionRepo) GetByHash(_ context.Context, _, _ string) (repository.AssetVersion, error) {
	return repository.AssetVersion{}, apperr.ErrNotFound // sentinel
}
func (r *VersionRepo) NextVersionNum(_ context.Context, _ string) (int64, error) {
	return 1, nil // sentinel: always 1 in tests that don't track version sequences
}
func (r *VersionRepo) SetCurrent(_ context.Context, _, _ string) error                { return nil }
func (r *VersionRepo) SetAssetThumbnail(_ context.Context, _ string, _ *string) error { return nil }
func (r *VersionRepo) RunInTx(_ context.Context, fn func(repository.VersionRepository) error) error {
	return fn(r)
}
func (r *VersionRepo) ListWithVariantCount(_ context.Context, _ string) ([]repository.AssetVersionWithCount, error) {
	panic("memory: VersionRepo.ListWithVariantCount not implemented")
}
func (r *VersionRepo) FindDuplicateVersions(
	_ context.Context, _, _, _ string,
) ([]repository.DuplicateVersionMatch, error) {
	return nil, nil // sentinel: no duplicates in tests that don't seed hash matches
}

// WorkspaceRepo ------------------------------------------------------------
// Stub: not exercised by service tests. Promote to mapStore when needed.

type WorkspaceRepo struct{}

func NewWorkspaceRepo() *WorkspaceRepo { return &WorkspaceRepo{} }

func (r *WorkspaceRepo) GetByID(_ context.Context, _ string) (repository.Workspace, error) {
	panic("memory: WorkspaceRepo.GetByID not implemented")
}
func (r *WorkspaceRepo) Create(_ context.Context, w repository.Workspace) (repository.Workspace, error) {
	return w, nil // echo-back
}
func (r *WorkspaceRepo) Update(_ context.Context, _ repository.Workspace) (repository.Workspace, error) {
	panic("memory: WorkspaceRepo.Update not implemented")
}
func (r *WorkspaceRepo) UpdateLockedTaxonomy(_ context.Context, _ string, _ bool) (repository.Workspace, error) {
	panic("memory: WorkspaceRepo.UpdateLockedTaxonomy not implemented")
}
func (r *WorkspaceRepo) UpdateAutoTagSettings(
	_ context.Context,
	_ string,
	_ bool,
	_ string,
) (repository.Workspace, error) {
	panic("memory: WorkspaceRepo.UpdateAutoTagSettings not implemented")
}
func (r *WorkspaceRepo) UpdateDuplicateDetectionMode(
	_ context.Context,
	_ string,
	_ string,
) (repository.Workspace, error) {
	panic("memory: WorkspaceRepo.UpdateDuplicateDetectionMode not implemented")
}
func (r *WorkspaceRepo) GetAIProviderKey(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (r *WorkspaceRepo) SetAIProviderKey(_ context.Context, _, _, _ string) error { return nil }
func (r *WorkspaceRepo) ClearAIProviderKey(_ context.Context, _, _ string) error  { return nil }
func (r *WorkspaceRepo) CountAssets(_ context.Context, _ string) (int64, error) {
	return 0, nil // sentinel
}
func (r *WorkspaceRepo) GetMember(_ context.Context, _, _ string) (repository.Member, error) {
	panic("memory: WorkspaceRepo.GetMember not implemented")
}
func (r *WorkspaceRepo) ListMembers(_ context.Context, _ string) ([]repository.Member, error) {
	panic("memory: WorkspaceRepo.ListMembers not implemented")
}
func (r *WorkspaceRepo) CountMembers(_ context.Context, _ string) (int64, error) {
	return 0, nil // sentinel
}
func (r *WorkspaceRepo) CreateMember(_ context.Context, _ repository.Member) error { return nil }
func (r *WorkspaceRepo) DeleteMember(_ context.Context, _, _ string) error         { return nil }
func (r *WorkspaceRepo) UpdateMemberRole(_ context.Context, _, _, _ string) error  { return nil }
func (r *WorkspaceRepo) CreateInvite(_ context.Context, inv repository.Invite) (repository.Invite, error) {
	return inv, nil // echo-back
}
func (r *WorkspaceRepo) ListPendingInvites(_ context.Context, _ string) ([]repository.Invite, error) {
	panic("memory: WorkspaceRepo.ListPendingInvites not implemented")
}
func (r *WorkspaceRepo) GetInviteByToken(_ context.Context, _ string) (repository.Invite, error) {
	panic("memory: WorkspaceRepo.GetInviteByToken not implemented")
}
func (r *WorkspaceRepo) DeleteInvite(_ context.Context, _, _ string) error { return nil }
func (r *WorkspaceRepo) AcceptInvite(_ context.Context, _ string) error    { return nil }
func (r *WorkspaceRepo) ListByUserID(_ context.Context, _ string) ([]repository.WorkspaceWithRole, error) {
	panic("memory: WorkspaceRepo.ListByUserID not implemented")
}
func (r *WorkspaceRepo) RunInTx(_ context.Context, fn func(repository.WorkspaceRepository) error) error {
	return fn(r)
}
func (r *WorkspaceRepo) RunRegistrationTx(
	_ context.Context,
	fn func(context.Context, repository.UserRepository, repository.WorkspaceRepository) error,
) error {
	return fn(context.Background(), &UserRepo{}, r)
}

// UserRepo -----------------------------------------------------------------
// Stub: not exercised by service tests. Promote to mapStore when needed.

type UserRepo struct{}

func NewUserRepo() *UserRepo { return &UserRepo{} }

func (r *UserRepo) GetByID(_ context.Context, _ string) (repository.User, error) {
	panic("memory: UserRepo.GetByID not implemented")
}
func (r *UserRepo) GetByEmail(_ context.Context, _ string) (repository.User, error) {
	panic("memory: UserRepo.GetByEmail not implemented")
}
func (r *UserRepo) Create(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil // echo-back
}
func (r *UserRepo) Update(_ context.Context, _ repository.User) (repository.User, error) {
	panic("memory: UserRepo.Update not implemented")
}
func (r *UserRepo) UpdateProfile(_ context.Context, _ string, _ string) (repository.User, error) {
	panic("memory: UserRepo.UpdateProfile not implemented")
}
func (r *UserRepo) UpdateAvatarKey(_ context.Context, _ string, _ string) (repository.User, error) {
	panic("memory: UserRepo.UpdateAvatarKey not implemented")
}
func (r *UserRepo) ClearAvatarKey(_ context.Context, _ string) (repository.User, error) {
	panic("memory: UserRepo.ClearAvatarKey not implemented")
}
func (r *UserRepo) SetPassword(_ context.Context, _, _ string) error { return nil }
func (r *UserRepo) SetAuthMethods(_ context.Context, _ string, _ string) (repository.User, error) {
	panic("memory: UserRepo.SetAuthMethods not implemented")
}
func (r *UserRepo) SetPendingEmail(_ context.Context, _ string, _ string) error { return nil }
func (r *UserRepo) ClearPendingEmail(_ context.Context, _ string) error         { return nil }
func (r *UserRepo) ConfirmEmailChange(_ context.Context, _ string, _ string) (repository.User, error) {
	panic("memory: UserRepo.ConfirmEmailChange not implemented")
}
func (r *UserRepo) SoftDelete(_ context.Context, _ string) error           { return nil }
func (r *UserRepo) AnonymizeDeletedUser(_ context.Context, _ string) error { return nil }
func (r *UserRepo) HardDelete(_ context.Context, _ string) error           { return nil }
func (r *UserRepo) GetByGoogleID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound // sentinel
}
func (r *UserRepo) GetByCanvaID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound // sentinel
}
func (r *UserRepo) GetByOIDC(_ context.Context, _, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound // sentinel
}
func (r *UserRepo) CreateWithGoogle(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil // echo-back
}
func (r *UserRepo) CreateWithOIDC(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil // echo-back
}
func (r *UserRepo) CreateWithCanva(_ context.Context, u repository.User) (repository.User, error) {
	return u, nil // echo-back
}
func (r *UserRepo) LinkGoogle(_ context.Context, _ repository.User) (repository.User, error) {
	panic("memory: UserRepo.LinkGoogle not implemented")
}
func (r *UserRepo) LinkOIDC(_ context.Context, _ repository.User) (repository.User, error) {
	panic("memory: UserRepo.LinkOIDC not implemented")
}
func (r *UserRepo) LinkCanva(_ context.Context, _ repository.User) (repository.User, error) {
	panic("memory: UserRepo.LinkCanva not implemented")
}
func (r *UserRepo) UnlinkGoogle(_ context.Context, _ repository.User) (repository.User, error) {
	panic("memory: UserRepo.UnlinkGoogle not implemented")
}
func (r *UserRepo) UnlinkOIDC(_ context.Context, _ repository.User) (repository.User, error) {
	panic("memory: UserRepo.UnlinkOIDC not implemented")
}
func (r *UserRepo) UnlinkCanva(_ context.Context, _ repository.User) (repository.User, error) {
	panic("memory: UserRepo.UnlinkCanva not implemented")
}
func (r *UserRepo) ListWorkspaceIDs(_ context.Context, _ string) ([]string, error) {
	panic("memory: UserRepo.ListWorkspaceIDs not implemented")
}
func (r *UserRepo) RunInTx(_ context.Context, fn func(repository.UserRepository) error) error {
	return fn(r)
}

// OAuthRepo ----------------------------------------------------------------
// Stub: not exercised by service tests. Promote to mapStore when needed.

type OAuthRepo struct{}

func NewOAuthRepo() *OAuthRepo { return &OAuthRepo{} }

func (r *OAuthRepo) List(_ context.Context, _ string) ([]repository.OAuthConnection, error) {
	panic("memory: OAuthRepo.List not implemented")
}
func (r *OAuthRepo) GetByID(_ context.Context, _, _ string) (repository.OAuthConnection, error) {
	return repository.OAuthConnection{}, apperr.ErrNotFound // sentinel
}
func (r *OAuthRepo) GetByProviderUserID(_ context.Context, _, _, _ string) (repository.OAuthConnection, error) {
	return repository.OAuthConnection{}, apperr.ErrNotFound // sentinel
}
func (r *OAuthRepo) Create(_ context.Context, _ repository.OAuthConnection) error { return nil }
func (r *OAuthRepo) UpdateTokens(_ context.Context, _, _ string, _ *string, _ *string) error {
	return nil
}
func (r *OAuthRepo) UpdateTokensAndScopes(_ context.Context, _, _ string, _ *string, _ *string, _ string) error {
	return nil
}
func (r *OAuthRepo) Delete(_ context.Context, _, _ string) error { return nil }
