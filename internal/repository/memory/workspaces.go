package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

const providerImageRouter = "imagerouter"

// RealWorkspaceRepo is a map-backed WorkspaceRepository for unit tests.
type RealWorkspaceRepo struct {
	mu         sync.RWMutex
	workspaces map[string]repository.Workspace
	irKeys     map[string]string
	orKeys     map[string]string
	members    map[string]repository.Member // key: workspaceID+":"+userID
	invites    map[string]repository.Invite // key: invite ID
	userWS     map[string][]string          // userID -> []workspaceID
	userRepo   repository.UserRepository    // injected for RunRegistrationTx
}

func NewRealWorkspaceRepo() *RealWorkspaceRepo {
	return &RealWorkspaceRepo{
		workspaces: make(map[string]repository.Workspace),
		irKeys:     make(map[string]string),
		orKeys:     make(map[string]string),
		members:    make(map[string]repository.Member),
		invites:    make(map[string]repository.Invite),
		userWS:     make(map[string][]string),
	}
}

// SetUserRepo wires the UserRepository used by RunRegistrationTx.
func (r *RealWorkspaceRepo) SetUserRepo(u repository.UserRepository) {
	r.userRepo = u
}

func (r *RealWorkspaceRepo) Seed(workspaces ...repository.Workspace) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ws := range workspaces {
		r.workspaces[ws.ID] = ws
	}
}

func (r *RealWorkspaceRepo) GetByID(_ context.Context, id string) (repository.Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return repository.Workspace{}, fmt.Errorf("workspace %q: %w", id, apperr.ErrNotFound)
	}
	return ws, nil
}

func (r *RealWorkspaceRepo) Create(_ context.Context, ws repository.Workspace) (repository.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaces[ws.ID] = ws
	return ws, nil
}

func (r *RealWorkspaceRepo) Update(_ context.Context, ws repository.Workspace) (repository.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[ws.ID]; !ok {
		return repository.Workspace{}, fmt.Errorf("workspace %q: %w", ws.ID, apperr.ErrNotFound)
	}
	r.workspaces[ws.ID] = ws
	return ws, nil
}

func (r *RealWorkspaceRepo) UpdateLockedTaxonomy(
	_ context.Context,
	workspaceID string,
	locked bool,
) (repository.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[workspaceID]
	if !ok {
		return repository.Workspace{}, fmt.Errorf("workspace %q: %w", workspaceID, apperr.ErrNotFound)
	}
	ws.LockedTaxonomy = locked
	r.workspaces[workspaceID] = ws
	return ws, nil
}

func (r *RealWorkspaceRepo) UpdateAutoTagSettings(
	_ context.Context,
	workspaceID string,
	enabled bool,
	mode string,
) (repository.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[workspaceID]
	if !ok {
		return repository.Workspace{}, fmt.Errorf("workspace %q: %w", workspaceID, apperr.ErrNotFound)
	}
	ws.AutoTagEnabled = enabled
	ws.AutoTagMode = mode
	r.workspaces[workspaceID] = ws
	return ws, nil
}

func (r *RealWorkspaceRepo) UpdateDuplicateDetectionMode(
	_ context.Context,
	workspaceID string,
	mode string,
) (repository.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[workspaceID]
	if !ok {
		return repository.Workspace{}, fmt.Errorf("workspace %q: %w", workspaceID, apperr.ErrNotFound)
	}
	ws.DuplicateDetectionMode = mode
	r.workspaces[workspaceID] = ws
	return ws, nil
}

func (r *RealWorkspaceRepo) CountAssets(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (r *RealWorkspaceRepo) GetAIProviderKey(_ context.Context, workspaceID, providerName string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.workspaces[workspaceID]; !ok {
		return "", fmt.Errorf("workspace %q: %w", workspaceID, apperr.ErrNotFound)
	}
	if providerName == providerImageRouter {
		return r.irKeys[workspaceID], nil
	}
	return r.orKeys[workspaceID], nil
}

func (r *RealWorkspaceRepo) SetAIProviderKey(_ context.Context, workspaceID, providerName, encKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[workspaceID]; !ok {
		return fmt.Errorf("workspace %q: %w", workspaceID, apperr.ErrNotFound)
	}
	if providerName == providerImageRouter {
		r.irKeys[workspaceID] = encKey
	} else {
		r.orKeys[workspaceID] = encKey
	}
	return nil
}

func (r *RealWorkspaceRepo) ClearAIProviderKey(_ context.Context, workspaceID, providerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[workspaceID]; !ok {
		return fmt.Errorf("workspace %q: %w", workspaceID, apperr.ErrNotFound)
	}
	if providerName == providerImageRouter {
		delete(r.irKeys, workspaceID)
	} else {
		delete(r.orKeys, workspaceID)
	}
	return nil
}

func (r *RealWorkspaceRepo) GetMember(_ context.Context, workspaceID, userID string) (repository.Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.members[workspaceID+":"+userID]
	if !ok {
		return repository.Member{}, fmt.Errorf("member not found: %w", apperr.ErrNotFound)
	}
	return m, nil
}

func (r *RealWorkspaceRepo) ListMembers(_ context.Context, workspaceID string) ([]repository.Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Member
	for _, m := range r.members {
		if m.WorkspaceID == workspaceID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *RealWorkspaceRepo) CountMembers(ctx context.Context, workspaceID string) (int64, error) {
	members, err := r.ListMembers(ctx, workspaceID)
	return int64(len(members)), err
}

func (r *RealWorkspaceRepo) CreateMember(_ context.Context, m repository.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m.JoinedAt.IsZero() {
		m.JoinedAt = time.Now()
	}
	r.members[m.WorkspaceID+":"+m.UserID] = m
	r.userWS[m.UserID] = append(r.userWS[m.UserID], m.WorkspaceID)
	return nil
}

func (r *RealWorkspaceRepo) DeleteMember(_ context.Context, workspaceID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.members, workspaceID+":"+userID)
	current := r.userWS[userID]
	next := current[:0]
	for _, id := range current {
		if id != workspaceID {
			next = append(next, id)
		}
	}
	r.userWS[userID] = append([]string(nil), next...)
	return nil
}

func (r *RealWorkspaceRepo) UpdateMemberRole(_ context.Context, workspaceID, userID, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := workspaceID + ":" + userID
	m, ok := r.members[key]
	if !ok {
		return fmt.Errorf("member not found: %w", apperr.ErrNotFound)
	}
	m.Role = role
	r.members[key] = m
	return nil
}

func (r *RealWorkspaceRepo) CreateInvite(_ context.Context, inv repository.Invite) (repository.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}
	if inv.Token == "" {
		inv.Token = uuid.New().String()
	}
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	}
	inv.CreatedAt = time.Now()
	r.invites[inv.ID] = inv
	return inv, nil
}

func (r *RealWorkspaceRepo) ListPendingInvites(_ context.Context, workspaceID string) ([]repository.Invite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Invite
	now := time.Now()
	for _, inv := range r.invites {
		if inv.WorkspaceID == workspaceID && inv.AcceptedAt == nil && inv.ExpiresAt.After(now) {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (r *RealWorkspaceRepo) GetInviteByToken(_ context.Context, token string) (repository.Invite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	for _, inv := range r.invites {
		if inv.Token == token && inv.AcceptedAt == nil && inv.ExpiresAt.After(now) {
			return inv, nil
		}
	}
	return repository.Invite{}, fmt.Errorf("invite not found: %w", apperr.ErrNotFound)
}

func (r *RealWorkspaceRepo) DeleteInvite(_ context.Context, workspaceID, inviteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.invites, inviteID)
	_ = workspaceID
	return nil
}

func (r *RealWorkspaceRepo) AcceptInvite(_ context.Context, inviteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inv, ok := r.invites[inviteID]
	if !ok {
		return fmt.Errorf("invite not found: %w", apperr.ErrNotFound)
	}
	now := time.Now()
	inv.AcceptedAt = &now
	r.invites[inviteID] = inv
	return nil
}

func (r *RealWorkspaceRepo) ListByUserID(_ context.Context, userID string) ([]repository.WorkspaceWithRole, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.WorkspaceWithRole
	for _, m := range r.members {
		if m.UserID == userID {
			if ws, ok := r.workspaces[m.WorkspaceID]; ok {
				out = append(out, repository.WorkspaceWithRole{Workspace: ws, Role: m.Role})
			}
		}
	}
	return out, nil
}

// RunInTx runs fn with the same repo (memory is already serialised; no real tx needed).
func (r *RealWorkspaceRepo) RunInTx(_ context.Context, fn func(repository.WorkspaceRepository) error) error {
	return fn(r)
}

// RunRegistrationTx runs fn with the same repo instances (memory is already atomic).
func (r *RealWorkspaceRepo) RunRegistrationTx(
	_ context.Context,
	fn func(context.Context, repository.UserRepository, repository.WorkspaceRepository) error,
) error {
	// The caller must supply the user repo; here we use an inline no-op stub that
	// delegates back to whatever real user repo is in use. Since memory tests inject
	// a RealUserRepo, we need access to it. We expose a SetUserRepo method for that.
	return fn(context.Background(), r.userRepo, r)
}
