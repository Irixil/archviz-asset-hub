package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func TestHandleUploadAvatar_DelegatesToService(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "user_1", "ws_1")

	var gotUserID string
	var gotBytes []byte
	env.Users.UploadAvatarFn = func(_ context.Context, userID string, data []byte) (*service.OIDCUserDTO, error) {
		gotUserID = userID
		gotBytes = append([]byte(nil), data...)
		key := "avatars/user_1.webp"
		return &service.OIDCUserDTO{
			ID:               userID,
			Name:             "User",
			DisplayName:      "User",
			Email:            "user@example.com",
			AvatarStorageKey: &key,
			AuthMethods:      `[]`,
		}, nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	payload := []byte("avatar-bytes")
	if _, writeErr := part.Write(payload); writeErr != nil {
		t.Fatalf("part.Write: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("writer.Close: %v", closeErr)
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/users/me/avatar", body, token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	if gotUserID != "user_1" {
		t.Fatalf("userID = %q, want user_1", gotUserID)
	}
	if !bytes.Equal(gotBytes, payload) {
		t.Fatalf("payload = %q, want %q", gotBytes, payload)
	}
}

func TestHandleUploadAvatar_TooLarge(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "user_1", "ws_1")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, writeErr := part.Write(bytes.Repeat([]byte("a"), 5<<20+1)); writeErr != nil {
		t.Fatalf("part.Write: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("writer.Close: %v", closeErr)
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/users/me/avatar", body, token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusRequestEntityTooLarge)

	var out struct {
		Error string `json:"error"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&out); decodeErr != nil {
		t.Fatalf("decode: %v", decodeErr)
	}
	if out.Error != "avatar_too_large" {
		t.Fatalf("error = %q, want avatar_too_large", out.Error)
	}
}

func TestHandleUploadAvatar_MissingField(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "user_1", "ws_1")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/users/me/avatar", body, token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestHandleDeleteAvatar_DelegatesToService(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "user_1", "ws_1")

	var gotUserID string
	env.Users.DeleteAvatarFn = func(_ context.Context, userID string) error {
		gotUserID = userID
		return nil
	}

	resp, err := env.App.Test(testutil.BearerRequest(http.MethodDelete, "/api/v1/users/me/avatar", nil, token))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusNoContent)

	if gotUserID != "user_1" {
		t.Fatalf("userID = %q, want user_1", gotUserID)
	}
}

func TestHandleUploadAvatar_UnsupportedType(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "user_1", "ws_1")
	env.Users.UploadAvatarFn = func(_ context.Context, _ string, _ []byte) (*service.OIDCUserDTO, error) {
		return nil, service.ErrUnsupportedAvatarType
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("avatar", "avatar.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, writeErr := part.Write([]byte("payload")); writeErr != nil {
		t.Fatalf("part.Write: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("writer.Close: %v", closeErr)
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/users/me/avatar", body, token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnsupportedMediaType)
}

func TestHandleDeleteAvatar_ServiceError(t *testing.T) {
	env := testutil.NewTestEnv(t)
	token := env.MintToken(t, "user_1", "ws_1")
	env.Users.DeleteAvatarFn = func(_ context.Context, _ string) error {
		return apperr.ErrNotFound
	}

	resp, err := env.App.Test(testutil.BearerRequest(http.MethodDelete, "/api/v1/users/me/avatar", nil, token))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusNotFound)
}
