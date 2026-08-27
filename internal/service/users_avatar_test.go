package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/storage"

	"github.com/HugoSmits86/nativewebp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type errStorage struct {
	putErr    error
	deleteErr error
	keys      map[string][]byte
}

func (s *errStorage) Put(_ context.Context, key string, r io.Reader) error {
	if s.putErr != nil {
		return s.putErr
	}
	if s.keys == nil {
		s.keys = make(map[string][]byte)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.keys[key] = data
	return nil
}

func (s *errStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if data, ok := s.keys[key]; ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, errors.New("not found")
}

func (s *errStorage) Delete(_ context.Context, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.keys, key)
	return nil
}

func (s *errStorage) List(_ context.Context, prefix string) ([]string, error) {
	var out []string
	for key := range s.keys {
		if len(prefix) == 0 || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, key)
		}
	}
	return out, nil
}

func (s *errStorage) Stat(_ context.Context, key string) (storage.Info, error) {
	if data, ok := s.keys[key]; ok {
		return storage.Info{Size: int64(len(data))}, nil
	}
	return storage.Info{}, fmt.Errorf("stat %s: %w", key, storage.ErrNotFound)
}

func makePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, color.NRGBA{R: uint8(x * 10), G: uint8(y * 10), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func installServiceSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return recorder
}

func findServiceSpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range recorder.Ended() {
		if span.Name() == name {
			return span
		}
	}
	t.Fatalf("span %q not found; ended=%d", name, len(recorder.Ended()))
	return nil
}

func assertServiceAttrString(t *testing.T, span sdktrace.ReadOnlySpan, key, want string) {
	t.Helper()
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			if attr.Value.AsString() != want {
				t.Fatalf("%s = %q, want %q", key, attr.Value.AsString(), want)
			}
			return
		}
	}
	t.Fatalf("attribute %s not found", key)
}

func assertServiceAttrBool(t *testing.T, span sdktrace.ReadOnlySpan, key string, want bool) {
	t.Helper()
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			if attr.Value.AsBool() != want {
				t.Fatalf("%s = %v, want %v", key, attr.Value.AsBool(), want)
			}
			return
		}
	}
	t.Fatalf("attribute %s not found", key)
}

func TestUserService_UploadAvatar_OK(t *testing.T) {
	recorder := installServiceSpanRecorder(t)
	svc, users, _, stor := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "avatar@example.com", Name: "Avatar"})

	dto, err := svc.UploadAvatar(context.Background(), "u_1", makePNG(t))
	if err != nil {
		t.Fatalf("UploadAvatar: %v", err)
	}
	if dto.AvatarStorageKey == nil || *dto.AvatarStorageKey != "avatars/u_1.webp" {
		t.Fatalf("AvatarStorageKey = %v", dto.AvatarStorageKey)
	}

	keys, err := stor.List(t.Context(), "avatars")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 || keys[0] != "avatars/u_1.webp" {
		t.Fatalf("stored keys = %v", keys)
	}
	stored, err := stor.Get(t.Context(), "avatars/u_1.webp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	t.Cleanup(func() { _ = stored.Close() })
	img, err := nativewebp.Decode(stored)
	if err != nil {
		t.Fatalf("nativewebp.Decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != 256 {
		t.Fatalf("decoded width = %d, want %d", got, 256)
	}
	if got := img.Bounds().Dy(); got != 256 {
		t.Fatalf("decoded height = %d, want %d", got, 256)
	}

	root := findServiceSpan(t, recorder, "service.users.upload_avatar")
	assertServiceAttrString(t, root, "damask.user_id", "u_1")
	assertServiceAttrString(t, root, "avatar.storage_key", "avatars/u_1.webp")
	_ = findServiceSpan(t, recorder, "service.users.avatar_storage_put")
}

func TestUserService_UploadAvatar_Unsupported(t *testing.T) {
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "avatar@example.com", Name: "Avatar"})

	_, err := svc.UploadAvatar(context.Background(), "u_1", []byte("not-an-image"))
	if !errors.Is(err, service.ErrUnsupportedAvatarType) {
		t.Fatalf("expected ErrUnsupportedAvatarType, got %v", err)
	}
}

func TestUserService_UploadAvatar_StorageFailure(t *testing.T) {
	users := memoryUserRepoWithSeed(t, repository.User{ID: "u_1", Email: "avatar@example.com", Name: "Avatar"})
	workspaces := memoryWorkspaceRepo(t, users)
	svc := service.NewUserService(users, workspaces, &errStorage{putErr: errors.New("boom")})

	_, err := svc.UploadAvatar(context.Background(), "u_1", makePNG(t))
	if !errors.Is(err, service.ErrAvatarStorage) {
		t.Fatalf("expected ErrAvatarStorage, got %v", err)
	}
}

func TestUserService_DeleteAvatar_OK(t *testing.T) {
	recorder := installServiceSpanRecorder(t)
	svc, users, _, stor := newUserSvc(t)
	key := "avatars/u_1.webp"
	users.Seed(repository.User{ID: "u_1", Email: "avatar@example.com", Name: "Avatar", AvatarStorageKey: &key})
	if err := stor.Put(t.Context(), key, bytes.NewReader([]byte("avatar"))); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := svc.DeleteAvatar(context.Background(), "u_1"); err != nil {
		t.Fatalf("DeleteAvatar: %v", err)
	}

	user, err := users.GetByID(context.Background(), "u_1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if user.AvatarStorageKey != nil {
		t.Fatalf("AvatarStorageKey = %v, want nil", *user.AvatarStorageKey)
	}

	keys, err := stor.List(t.Context(), "avatars")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("stored keys = %v, want empty", keys)
	}

	root := findServiceSpan(t, recorder, "service.users.delete_avatar")
	assertServiceAttrString(t, root, "avatar.storage_key", key)
	assertServiceAttrBool(t, root, "avatar.has_existing_storage_key", true)
	_ = findServiceSpan(t, recorder, "service.users.avatar_storage_delete")
}

func TestUserService_DeleteAvatar_NoAvatar(t *testing.T) {
	recorder := installServiceSpanRecorder(t)
	svc, users, _, _ := newUserSvc(t)
	users.Seed(repository.User{ID: "u_1", Email: "avatar@example.com", Name: "Avatar"})

	if err := svc.DeleteAvatar(context.Background(), "u_1"); err != nil {
		t.Fatalf("DeleteAvatar: %v", err)
	}

	root := findServiceSpan(t, recorder, "service.users.delete_avatar")
	assertServiceAttrBool(t, root, "avatar.has_existing_storage_key", false)
}

func TestUserService_DeleteAvatar_StorageFailure(t *testing.T) {
	key := "avatars/u_1.webp"
	users := memoryUserRepoWithSeed(
		t,
		repository.User{ID: "u_1", Email: "avatar@example.com", Name: "Avatar", AvatarStorageKey: &key},
	)
	workspaces := memoryWorkspaceRepo(t, users)
	svc := service.NewUserService(users, workspaces, &errStorage{deleteErr: errors.New("boom")})

	err := svc.DeleteAvatar(context.Background(), "u_1")
	if !errors.Is(err, service.ErrAvatarStorage) {
		t.Fatalf("expected ErrAvatarStorage, got %v", err)
	}

	user, getErr := users.GetByID(context.Background(), "u_1")
	if getErr != nil {
		t.Fatalf("GetByID: %v", getErr)
	}
	if user.AvatarStorageKey == nil || *user.AvatarStorageKey != key {
		t.Fatalf("AvatarStorageKey = %v, want %q", user.AvatarStorageKey, key)
	}
}

func memoryUserRepoWithSeed(t *testing.T, user repository.User) *memory.RealUserRepo {
	t.Helper()
	users := memory.NewRealUserRepo()
	users.Seed(user)
	return users
}

func memoryWorkspaceRepo(t *testing.T, users *memory.RealUserRepo) *memory.RealWorkspaceRepo {
	t.Helper()
	workspaces := memory.NewRealWorkspaceRepo()
	workspaces.SetUserRepo(users)
	return workspaces
}

var _ storage.Storage = (*errStorage)(nil)
