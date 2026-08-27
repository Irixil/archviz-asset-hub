package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"
)

// UserDTO is the output representation of a user.
type UserDTO struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

// OIDCUserDTO is returned by UpsertOIDCUser / UpsertCanvaUser / UnlinkProvider.
type OIDCUserDTO struct {
	ID               string
	Name             string
	DisplayName      string
	Email            string
	AvatarURL        *string
	AvatarStorageKey *string
	AuthMethods      string
	OIDCLinked       bool
	GoogleLinked     bool
	CanvaLinked      bool
	PendingEmail     *string
	WorkspaceID      string
}

// UpsertOIDCUserParams is the input for UserService.UpsertOIDCUser.
type UpsertOIDCUserParams struct {
	Issuer    string
	Sub       string
	Email     string
	Name      string
	AvatarURL string
	IsGoogle  bool
}

// UpsertCanvaUserParams is the input for UserService.UpsertCanvaUser.
type UpsertCanvaUserParams struct {
	CanvaID   string
	Email     string
	Name      string
	AvatarURL string
}

// RegisterUserParams is the input for UserService.Register.
type RegisterUserParams struct {
	UserID        string
	Name          string
	Email         string
	PasswordHash  string
	WorkspaceName string
}

// RegisterUserResult is returned by UserService.Register.
type RegisterUserResult struct {
	User        *UserDTO
	WorkspaceID string
}

// LoginUserParams is the input for UserService.Login.
type LoginUserParams struct {
	Email         string
	PlainPassword string
}

// LoginUserResult is returned by UserService.Login.
type LoginUserResult struct {
	User        *UserDTO
	WorkspaceID string
}

var ErrUnsupportedAvatarType = errors.New("unsupported avatar type")
var ErrAvatarStorage = errors.New("avatar storage failure")

type userService struct {
	users      repository.UserRepository
	workspaces repository.WorkspaceRepository
	stor       storage.Storage
}

// NewUserService returns a UserService.
func NewUserService(
	users repository.UserRepository,
	workspaces repository.WorkspaceRepository,
	stor storage.Storage,
) UserService {
	return &userService{users: users, workspaces: workspaces, stor: stor}
}

// Register creates a new user and a default workspace atomically.
// It uses WorkspaceRepository.RunRegistrationTx so that user row, workspace row, and
// member row all land in a single DB transaction without leaking *[sql.DB] into the service.
func (s *userService) Register(ctx context.Context, p RegisterUserParams) (*RegisterUserResult, error) {
	workspaceID := uuid.New().String()
	var result RegisterUserResult

	err := s.workspaces.RunRegistrationTx(
		ctx,
		func(ctx context.Context, txUsers repository.UserRepository, txWorkspaces repository.WorkspaceRepository) error {
			u, createErr := txUsers.Create(ctx, repository.User{
				ID:           p.UserID,
				Email:        p.Email,
				PasswordHash: p.PasswordHash,
				Name:         p.Name,
			})
			if createErr != nil {
				if strings.Contains(createErr.Error(), "UNIQUE constraint failed") {
					return fmt.Errorf("email already in use: %w", apperr.ErrConflict)
				}
				return fmt.Errorf("could not create user: %w", createErr)
			}

			ws, wsErr := txWorkspaces.Create(ctx, repository.Workspace{
				ID:   workspaceID,
				Name: p.WorkspaceName,
			})
			if wsErr != nil {
				return fmt.Errorf("could not create workspace: %w", wsErr)
			}

			if memberErr := txWorkspaces.CreateMember(ctx, repository.Member{
				WorkspaceID: ws.ID,
				UserID:      u.ID,
				Role:        string(auth.Owner),
			}); memberErr != nil {
				return fmt.Errorf("could not create membership: %w", memberErr)
			}

			result = RegisterUserResult{User: toUserDTO(u), WorkspaceID: ws.ID}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *userService) Login(ctx context.Context, p LoginUserParams) (*LoginUserResult, error) {
	user, err := s.users.GetByEmail(ctx, p.Email)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, fmt.Errorf("invalid credentials: %w", apperr.ErrForbidden)
		}
		return nil, err
	}

	if cmpErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(p.PlainPassword)); cmpErr != nil {
		return nil, fmt.Errorf("invalid credentials: %w", apperr.ErrForbidden)
	}

	workspaces, err := s.workspaces.ListByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if len(workspaces) == 0 {
		return nil, fmt.Errorf("user has no workspace: %w", apperr.ErrInvalidInput)
	}

	return &LoginUserResult{
		User:        toUserDTO(user),
		WorkspaceID: workspaces[0].ID,
	}, nil
}

func (s *userService) GetByID(ctx context.Context, userID string) (*UserDTO, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserDTO(user), nil
}

func (s *userService) GetProfile(ctx context.Context, userID string) (*OIDCUserDTO, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
}

func (s *userService) GetProfileByEmail(ctx context.Context, email string) (*OIDCUserDTO, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
}

func (s *userService) UpdateProfile(ctx context.Context, userID, displayName string) (*OIDCUserDTO, error) {
	user, err := s.users.UpdateProfile(ctx, userID, displayName)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
}

func (s *userService) UploadAvatar(ctx context.Context, userID string, data []byte) (dto *OIDCUserDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.users.upload_avatar",
		attribute.String("damask.user_id", userID),
		attribute.Int("avatar.bytes", len(data)),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "avatar upload failed", "user_id", userID, "error", err)
		}
	}()

	if s.stor == nil {
		return nil, errors.New("storage unavailable")
	}

	slog.DebugContext(ctx, "avatar upload started", "user_id", userID, "input_bytes", len(data))

	contentType := http.DetectContentType(data)
	switch contentType {
	case transform.MimeImageJPEG, transform.MimeImagePNG, transform.MimeImageWebP, transform.MimeImageGIF:
	default:
		err = fmt.Errorf("%w: %s", ErrUnsupportedAvatarType, contentType)
		return nil, err
	}

	_, processSpan := apptelemetry.StartSpan(ctx, "service.users.avatar_process")
	processed, processErr := transform.ProcessAvatar(data)
	apptelemetry.EndSpan(processSpan, processErr)
	if processErr != nil {
		err = fmt.Errorf("%w: %w", ErrUnsupportedAvatarType, processErr)
		return nil, err
	}

	storageKey := "avatars/" + userID + ".webp"
	span.SetAttributes(attribute.String("avatar.storage_key", storageKey))

	_, putSpan := apptelemetry.StartSpan(ctx, "service.users.avatar_storage_put",
		attribute.String("avatar.storage_key", storageKey),
	)
	putErr := s.stor.Put(ctx, storageKey, bytes.NewReader(processed))
	apptelemetry.EndSpan(putSpan, putErr)
	if putErr != nil {
		err = fmt.Errorf("%w: could not store avatar: %w", ErrAvatarStorage, putErr)
		return nil, err
	}

	repoCtx, repoSpan := apptelemetry.StartSpan(ctx, "service.users.avatar_repo_update",
		attribute.String("avatar.storage_key", storageKey),
	)
	user, repoErr := s.users.UpdateAvatarKey(repoCtx, userID, storageKey)
	apptelemetry.EndSpan(repoSpan, repoErr)
	if repoErr != nil {
		err = repoErr
		return nil, err
	}

	dto, err = s.toOIDCUserDTO(ctx, user)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(
		ctx,
		"avatar upload completed",
		"user_id",
		userID,
		"storage_key",
		storageKey,
		"output_bytes",
		len(processed),
	)
	return dto, nil
}

func (s *userService) DeleteAvatar(ctx context.Context, userID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.users.delete_avatar",
		attribute.String("damask.user_id", userID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "avatar delete failed", "user_id", userID, "error", err)
		}
	}()

	if s.stor == nil {
		return errors.New("storage unavailable")
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	hasAvatar := user.AvatarStorageKey != nil && *user.AvatarStorageKey != ""
	span.SetAttributes(attribute.Bool("avatar.has_existing_storage_key", hasAvatar))
	if !hasAvatar {
		slog.DebugContext(ctx, "avatar delete skipped", "user_id", userID)
		return nil
	}

	storageKey := *user.AvatarStorageKey
	span.SetAttributes(attribute.String("avatar.storage_key", storageKey))

	_, deleteSpan := apptelemetry.StartSpan(ctx, "service.users.avatar_storage_delete",
		attribute.String("avatar.storage_key", storageKey),
	)
	deleteErr := s.stor.Delete(ctx, storageKey)
	apptelemetry.EndSpan(deleteSpan, deleteErr)
	if deleteErr != nil {
		err = fmt.Errorf("%w: could not delete avatar: %w", ErrAvatarStorage, deleteErr)
		return err
	}

	_, repoSpan := apptelemetry.StartSpan(ctx, "service.users.avatar_repo_update",
		attribute.String("avatar.storage_key", storageKey),
	)
	_, repoErr := s.users.ClearAvatarKey(ctx, userID)
	apptelemetry.EndSpan(repoSpan, repoErr)
	if repoErr != nil {
		err = repoErr
		return err
	}

	slog.DebugContext(ctx, "avatar delete completed", "user_id", userID, "storage_key", storageKey)
	return nil
}

func (s *userService) UpdateAvatarKey(ctx context.Context, userID, storageKey string) (*OIDCUserDTO, error) {
	user, err := s.users.UpdateAvatarKey(ctx, userID, storageKey)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
}

func (s *userService) ClearAvatar(ctx context.Context, userID string) (*OIDCUserDTO, error) {
	user, err := s.users.ClearAvatarKey(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
}

func (s *userService) ResetPassword(ctx context.Context, userID, passwordHash string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", apperr.ErrInvalidInput)
	}
	if setErr := s.users.SetPassword(ctx, userID, passwordHash); setErr != nil {
		return setErr
	}
	if hasPasswordMethod(user.AuthMethods, "password") {
		return nil
	}
	_, err = s.users.SetAuthMethods(ctx, userID, addAuthMethod(user.AuthMethods, "password"))
	return err
}

func (s *userService) ChangePassword(ctx context.Context, userID, currentPassword, newPasswordHash string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if hasPasswordMethod(user.AuthMethods, "password") {
		if cmpErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); cmpErr != nil {
			return fmt.Errorf("current_password_incorrect: %w", apperr.ErrInvalidInput)
		}
	}
	if setErr := s.users.SetPassword(ctx, userID, newPasswordHash); setErr != nil {
		return setErr
	}
	if hasPasswordMethod(user.AuthMethods, "password") {
		return nil
	}
	_, err = s.users.SetAuthMethods(ctx, userID, addAuthMethod(user.AuthMethods, "password"))
	return err
}

func (s *userService) RequestEmailChange(ctx context.Context, userID, email string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if strings.EqualFold(user.Email, email) {
		return fmt.Errorf("email unchanged: %w", apperr.ErrInvalidInput)
	}
	if _, checkErr := s.users.GetByEmail(ctx, email); checkErr == nil {
		return fmt.Errorf("email already in use: %w", apperr.ErrConflict)
	} else if !errors.Is(checkErr, apperr.ErrNotFound) {
		return checkErr
	}
	return s.users.SetPendingEmail(ctx, userID, email)
}

func (s *userService) CancelEmailChange(ctx context.Context, userID string) error {
	return s.users.ClearPendingEmail(ctx, userID)
}

func (s *userService) ConfirmEmailChange(ctx context.Context, userID, email string) (*OIDCUserDTO, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", apperr.ErrInvalidInput)
	}
	if user.PendingEmail == nil || *user.PendingEmail != email {
		return nil, fmt.Errorf("stale token: %w", apperr.ErrInvalidInput)
	}
	if existing, checkErr := s.users.GetByEmail(ctx, email); checkErr == nil && existing.ID != userID {
		return nil, fmt.Errorf("email already in use: %w", apperr.ErrConflict)
	} else if checkErr != nil && !errors.Is(checkErr, apperr.ErrNotFound) {
		return nil, checkErr
	}
	updated, err := s.users.ConfirmEmailChange(ctx, userID, email)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, updated)
}

func (s *userService) DeleteAccount(ctx context.Context, userID, password string, hardDelete bool) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if hasPasswordMethod(user.AuthMethods, "password") {
		if cmpErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); cmpErr != nil {
			return fmt.Errorf("password_incorrect: %w", apperr.ErrInvalidInput)
		}
	}

	workspaces, err := s.workspaces.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if err = s.checkSoleOwnerWorkspaces(ctx, workspaces); err != nil {
		return err
	}

	return s.workspaces.RunRegistrationTx(
		ctx,
		func(ctx context.Context, txUsers repository.UserRepository, txWorkspaces repository.WorkspaceRepository) error {
			if !hardDelete {
				if softErr := txUsers.SoftDelete(ctx, userID); softErr != nil {
					return softErr
				}
				if anonErr := txUsers.AnonymizeDeletedUser(ctx, userID); anonErr != nil {
					return anonErr
				}
			}
			for _, ws := range workspaces {
				if delErr := txWorkspaces.DeleteMember(ctx, ws.ID, userID); delErr != nil {
					return delErr
				}
			}
			if hardDelete {
				if hardErr := txUsers.HardDelete(ctx, userID); hardErr != nil {
					return hardErr
				}
			}
			return nil
		},
	)
}

func (s *userService) checkSoleOwnerWorkspaces(ctx context.Context, workspaces []repository.WorkspaceWithRole) error {
	for _, ws := range workspaces {
		members, err := s.workspaces.ListMembers(ctx, ws.ID)
		if err != nil {
			return err
		}
		ownerCount := 0
		for _, m := range members {
			if m.Role == string(auth.Owner) {
				ownerCount++
			}
		}
		if ownerCount == 1 {
			return fmt.Errorf("sole workspace owner: %w", apperr.ErrInvalidInput)
		}
	}
	return nil
}

// CreateWorkspace creates a new workspace owned by userID in a transaction.
func (s *userService) CreateWorkspace(ctx context.Context, userID, name string) (*WorkspaceDTO, error) {
	workspaceID := uuid.New().String()

	err := s.workspaces.RunInTx(ctx, func(txWS repository.WorkspaceRepository) error {
		ws, err := txWS.Create(ctx, repository.Workspace{
			ID:   workspaceID,
			Name: name,
		})
		if err != nil {
			return err
		}
		return txWS.CreateMember(ctx, repository.Member{
			WorkspaceID: ws.ID,
			UserID:      userID,
			Role:        string(auth.Owner),
		})
	})
	if err != nil {
		return nil, err
	}

	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return toWorkspaceDTO(ws), nil
}

// oauthProvider captures the per-provider behavior for the three-phase OAuth upsert.
type oauthProvider struct {
	method     string
	lookupByID func(ctx context.Context) (repository.User, error)
	applyID    func(u *repository.User)
	linkUser   func(ctx context.Context, u repository.User) (repository.User, error)
	createUser func(ctx context.Context, txUsers repository.UserRepository, u repository.User) (repository.User, error)
}

func (s *userService) UpsertOIDCUser(ctx context.Context, p UpsertOIDCUserParams) (*OIDCUserDTO, error) {
	issuer := p.Issuer
	sub := p.Sub
	prov := oauthProvider{
		method: methodName(p.IsGoogle),
		lookupByID: func(ctx context.Context) (repository.User, error) {
			if p.IsGoogle {
				return s.users.GetByGoogleID(ctx, sub)
			}
			return s.users.GetByOIDC(ctx, issuer, sub)
		},
		applyID: func(u *repository.User) {
			if p.IsGoogle {
				u.GoogleUserID = &sub
			} else {
				u.OidcIssuer = &issuer
				u.OidcSub = &sub
			}
		},
		linkUser: func(ctx context.Context, u repository.User) (repository.User, error) {
			if p.IsGoogle {
				return s.users.LinkGoogle(ctx, u)
			}
			return s.users.LinkOIDC(ctx, u)
		},
		createUser: func(ctx context.Context, txUsers repository.UserRepository, u repository.User) (repository.User, error) {
			if p.IsGoogle {
				return txUsers.CreateWithGoogle(ctx, u)
			}
			return txUsers.CreateWithOIDC(ctx, u)
		},
	}
	return s.oauthUpsert(ctx, prov, p.Email, p.Name, p.AvatarURL)
}

func (s *userService) UpsertCanvaUser(ctx context.Context, p UpsertCanvaUserParams) (*OIDCUserDTO, error) {
	canvaID := p.CanvaID
	name := p.Name
	if name == "" {
		name = "Canva User"
	}
	email := p.Email
	if email == "" {
		email = "canva+" + canvaID + "@canva.local"
	}
	prov := oauthProvider{
		method: "canva",
		lookupByID: func(ctx context.Context) (repository.User, error) {
			return s.users.GetByCanvaID(ctx, canvaID)
		},
		applyID: func(u *repository.User) {
			u.CanvaUserID = &canvaID
		},
		linkUser: func(ctx context.Context, u repository.User) (repository.User, error) {
			return s.users.LinkCanva(ctx, u)
		},
		createUser: func(ctx context.Context, txUsers repository.UserRepository, u repository.User) (repository.User, error) {
			return txUsers.CreateWithCanva(ctx, u)
		},
	}
	return s.oauthUpsert(ctx, prov, email, name, p.AvatarURL)
}

// oauthUpsert implements the three-phase pattern shared by all OAuth providers:
// look up by provider ID → link by email → create new user+workspace.
func (s *userService) oauthUpsert(
	ctx context.Context,
	prov oauthProvider,
	email, name, avatarURL string,
) (*OIDCUserDTO, error) {
	// Phase 1: existing provider identity.
	user, err := prov.lookupByID(ctx)
	if err != nil && !errors.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		return s.refreshAndLink(ctx, user, prov, avatarURL)
	}

	// Phase 2: link by email.
	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		return s.refreshAndLink(ctx, existing, prov, avatarURL)
	}

	// Phase 3: new user + workspace in one transaction.
	return s.createOAuthUser(ctx, prov, email, name, avatarURL)
}

func (s *userService) refreshAndLink(
	ctx context.Context,
	user repository.User,
	prov oauthProvider,
	avatarURL string,
) (*OIDCUserDTO, error) {
	user.AuthMethods = addAuthMethod(user.AuthMethods, prov.method)
	if user.AvatarStorageKey == nil {
		user.AvatarURL = nilIfEmpty(avatarURL)
	}
	prov.applyID(&user)
	linked, err := prov.linkUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, linked)
}

func (s *userService) createOAuthUser(
	ctx context.Context,
	prov oauthProvider,
	email, name, avatarURL string,
) (*OIDCUserDTO, error) {
	userID := uuid.New().String()
	workspaceID := uuid.New().String()
	newUser := repository.User{
		ID:          userID,
		Email:       email,
		Name:        name,
		AvatarURL:   nilIfEmpty(avatarURL),
		AuthMethods: `["` + prov.method + `"]`,
	}
	prov.applyID(&newUser)

	var created repository.User
	err := s.workspaces.RunRegistrationTx(
		ctx,
		func(ctx context.Context, txUsers repository.UserRepository, txWorkspaces repository.WorkspaceRepository) error {
			var uErr error
			created, uErr = prov.createUser(ctx, txUsers, newUser)
			if uErr != nil {
				return uErr
			}
			ws, wErr := txWorkspaces.Create(ctx, repository.Workspace{ID: workspaceID, Name: name + "'s Workspace"})
			if wErr != nil {
				return wErr
			}
			return txWorkspaces.CreateMember(ctx, repository.Member{
				WorkspaceID: ws.ID,
				UserID:      created.ID,
				Role:        string(auth.Owner),
			})
		},
	)
	if err != nil {
		return nil, err
	}
	dto, err := s.toOIDCUserDTO(ctx, created)
	if err != nil {
		return nil, err
	}
	dto.WorkspaceID = workspaceID
	return dto, nil
}

func (s *userService) UnlinkProvider(ctx context.Context, userID, provider string) (*OIDCUserDTO, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if hasOnlyMethodStr(user.AuthMethods, provider) {
		return nil, fmt.Errorf("set a password before removing your last sign-in method: %w", apperr.ErrInvalidInput)
	}
	user.AuthMethods = removeAuthMethodStr(user.AuthMethods, provider)
	switch provider {
	case "google":
		user, err = s.users.UnlinkGoogle(ctx, user)
	case "oidc":
		user, err = s.users.UnlinkOIDC(ctx, user)
	case "canva":
		user, err = s.users.UnlinkCanva(ctx, user)
	}
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
}

func (s *userService) toOIDCUserDTO(ctx context.Context, user repository.User) (*OIDCUserDTO, error) {
	ids, err := s.users.ListWorkspaceIDs(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	wsID := ""
	if len(ids) > 0 {
		wsID = ids[0]
	}
	return &OIDCUserDTO{
		ID:               user.ID,
		Name:             user.Name,
		DisplayName:      displayNameForUser(user),
		Email:            user.Email,
		AvatarURL:        user.AvatarURL,
		AvatarStorageKey: user.AvatarStorageKey,
		AuthMethods:      user.AuthMethods,
		OIDCLinked:       user.OidcSub != nil,
		GoogleLinked:     user.GoogleUserID != nil,
		CanvaLinked:      user.CanvaUserID != nil,
		PendingEmail:     user.PendingEmail,
		WorkspaceID:      wsID,
	}, nil
}

func methodName(isGoogle bool) string {
	if isGoogle {
		return "google"
	}
	return "oidc"
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func addAuthMethod(current, method string) string {
	var methods []string
	_ = json.Unmarshal([]byte(current), &methods)
	if slices.Contains(methods, method) {
		return current
	}
	methods = append(methods, method)
	b, _ := json.Marshal(methods)
	return string(b)
}

func removeAuthMethodStr(current, method string) string {
	var methods []string
	_ = json.Unmarshal([]byte(current), &methods)
	out := methods[:0]
	for _, m := range methods {
		if m != method {
			out = append(out, m)
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func hasOnlyMethodStr(authMethods, method string) bool {
	var methods []string
	_ = json.Unmarshal([]byte(authMethods), &methods)
	return len(methods) == 1 && methods[0] == method
}

func hasPasswordMethod(authMethods, method string) bool { //nolint:unparam // readability
	var methods []string
	_ = json.Unmarshal([]byte(authMethods), &methods)
	return slices.Contains(methods, method)
}

func displayNameForUser(user repository.User) string {
	if user.DisplayName != nil && strings.TrimSpace(*user.DisplayName) != "" {
		return strings.TrimSpace(*user.DisplayName)
	}
	if strings.TrimSpace(user.Name) != "" {
		return strings.TrimSpace(user.Name)
	}
	if idx := strings.Index(user.Email, "@"); idx > 0 {
		return user.Email[:idx]
	}
	return user.Email
}

func toUserDTO(u repository.User) *UserDTO {
	return &UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
	}
}
