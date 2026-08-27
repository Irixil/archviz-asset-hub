package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealUserRepo is a map-backed UserRepository for unit tests.
type RealUserRepo struct {
	mu      sync.RWMutex
	byID    map[string]repository.User
	byEmail map[string]string // email -> id
}

func NewRealUserRepo() *RealUserRepo {
	return &RealUserRepo{
		byID:    make(map[string]repository.User),
		byEmail: make(map[string]string),
	}
}

func (r *RealUserRepo) Seed(users ...repository.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range users {
		r.byID[u.ID] = u
		r.byEmail[u.Email] = u.ID
	}
}

func (r *RealUserRepo) GetByID(_ context.Context, id string) (repository.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok || u.DeletedAt != nil {
		return repository.User{}, fmt.Errorf("user %q: %w", id, apperr.ErrNotFound)
	}
	return u, nil
}

func (r *RealUserRepo) GetByEmail(_ context.Context, email string) (repository.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byEmail[email]
	if !ok {
		return repository.User{}, fmt.Errorf("user %q: %w", email, apperr.ErrNotFound)
	}
	u := r.byID[id]
	if u.DeletedAt != nil {
		return repository.User{}, fmt.Errorf("user %q: %w", email, apperr.ErrNotFound)
	}
	return u, nil
}

func (r *RealUserRepo) Create(_ context.Context, u repository.User) (repository.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[u.Email]; exists {
		return repository.User{}, fmt.Errorf("email already in use: %w", apperr.ErrConflict)
	}
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u.ID
	return u, nil
}

func (r *RealUserRepo) Update(_ context.Context, u repository.User) (repository.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev, ok := r.byID[u.ID]
	if !ok {
		return repository.User{}, fmt.Errorf("user %q: %w", u.ID, apperr.ErrNotFound)
	}
	if prev.Email != u.Email {
		delete(r.byEmail, prev.Email)
		r.byEmail[u.Email] = u.ID
	}
	r.byID[u.ID] = u
	return u, nil
}

func (r *RealUserRepo) UpdateProfile(_ context.Context, id, displayName string) (repository.User, error) {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return repository.User{}, err
	}
	u.DisplayName = &displayName
	return r.Update(context.Background(), u)
}

func (r *RealUserRepo) UpdateAvatarKey(_ context.Context, id, storageKey string) (repository.User, error) {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return repository.User{}, err
	}
	u.AvatarStorageKey = &storageKey
	return r.Update(context.Background(), u)
}

func (r *RealUserRepo) ClearAvatarKey(_ context.Context, id string) (repository.User, error) {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return repository.User{}, err
	}
	u.AvatarStorageKey = nil
	return r.Update(context.Background(), u)
}

func (r *RealUserRepo) SetPassword(_ context.Context, id, passwordHash string) error {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	u.PasswordHash = passwordHash
	_, err = r.Update(context.Background(), u)
	return err
}

func (r *RealUserRepo) SetAuthMethods(_ context.Context, id, authMethods string) (repository.User, error) {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return repository.User{}, err
	}
	u.AuthMethods = authMethods
	return r.Update(context.Background(), u)
}

func (r *RealUserRepo) SetPendingEmail(_ context.Context, id, email string) error {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	u.PendingEmail = &email
	_, err = r.Update(context.Background(), u)
	return err
}

func (r *RealUserRepo) ClearPendingEmail(_ context.Context, id string) error {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	u.PendingEmail = nil
	_, err = r.Update(context.Background(), u)
	return err
}

func (r *RealUserRepo) ConfirmEmailChange(_ context.Context, id, pendingEmail string) (repository.User, error) {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return repository.User{}, err
	}
	if u.PendingEmail == nil || *u.PendingEmail != pendingEmail {
		return repository.User{}, apperr.ErrInvalidInput
	}
	u.Email = pendingEmail
	u.PendingEmail = nil
	return r.Update(context.Background(), u)
}

func (r *RealUserRepo) SoftDelete(_ context.Context, id string) error {
	u, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	deleted := "deleted"
	u.DeletedAt = &deleted
	_, err = r.Update(context.Background(), u)
	return err
}

func (r *RealUserRepo) AnonymizeDeletedUser(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return fmt.Errorf("user %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.byEmail, u.Email)
	u.Email = "deleted_" + u.ID + "@deleted.invalid"
	u.DisplayName = ptrString("Deleted user")
	u.PasswordHash = ""
	u.AvatarStorageKey = nil
	u.AvatarURL = nil
	u.PendingEmail = nil
	u.AuthMethods = "[]"
	r.byID[id] = u
	r.byEmail[u.Email] = u.ID
	return nil
}

func (r *RealUserRepo) HardDelete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return fmt.Errorf("user %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.byID, id)
	delete(r.byEmail, u.Email)
	return nil
}

func (r *RealUserRepo) GetByGoogleID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound
}
func (r *RealUserRepo) GetByCanvaID(_ context.Context, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound
}
func (r *RealUserRepo) GetByOIDC(_ context.Context, _, _ string) (repository.User, error) {
	return repository.User{}, apperr.ErrNotFound
}
func (r *RealUserRepo) CreateWithGoogle(_ context.Context, u repository.User) (repository.User, error) {
	return r.Create(context.Background(), u)
}
func (r *RealUserRepo) CreateWithOIDC(_ context.Context, u repository.User) (repository.User, error) {
	return r.Create(context.Background(), u)
}
func (r *RealUserRepo) CreateWithCanva(_ context.Context, u repository.User) (repository.User, error) {
	return r.Create(context.Background(), u)
}
func (r *RealUserRepo) LinkGoogle(_ context.Context, u repository.User) (repository.User, error) {
	return r.Update(context.Background(), u)
}
func (r *RealUserRepo) LinkOIDC(_ context.Context, u repository.User) (repository.User, error) {
	return r.Update(context.Background(), u)
}
func (r *RealUserRepo) LinkCanva(_ context.Context, u repository.User) (repository.User, error) {
	return r.Update(context.Background(), u)
}
func (r *RealUserRepo) UnlinkGoogle(_ context.Context, u repository.User) (repository.User, error) {
	return r.Update(context.Background(), u)
}
func (r *RealUserRepo) UnlinkOIDC(_ context.Context, u repository.User) (repository.User, error) {
	return r.Update(context.Background(), u)
}
func (r *RealUserRepo) UnlinkCanva(_ context.Context, u repository.User) (repository.User, error) {
	return r.Update(context.Background(), u)
}
func (r *RealUserRepo) ListWorkspaceIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (r *RealUserRepo) RunInTx(_ context.Context, fn func(repository.UserRepository) error) error {
	return fn(r)
}

func ptrString(s string) *string {
	return &s
}
