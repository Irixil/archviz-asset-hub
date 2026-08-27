//go:build integration

package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func TestStackExport_EmptyAssetIDs(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		testutil.JSONBody(map[string]any{"asset_ids": []string{}}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestStackExport_ValidZip(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Assets.CountByIDsFn = func(_ context.Context, _ string, ids []string) (int64, error) {
		return int64(len(ids)), nil
	}
	env.Stack.ExportZipFn = func(_ context.Context, _ string, p service.ExportZipParams, w io.Writer) error {
		zw := zip.NewWriter(w)
		f, _ := zw.Create("fixture.jpg")
		f.Write([]byte("fake-jpeg-bytes")) //nolint:errcheck
		return zw.Close()
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		testutil.JSONBody(map[string]any{"asset_ids": []string{"ast_1"}, "filename": "my-export"}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type: got %q, want application/zip", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "my-export.zip") {
		t.Errorf("Content-Disposition: got %q, want my-export.zip", cd)
	}

	data, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	if len(zr.File) == 0 {
		t.Error("expected at least one file in zip")
	}
}

func TestStackExport_FilenameSanitised(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Assets.CountByIDsFn = func(_ context.Context, _ string, ids []string) (int64, error) {
		return int64(len(ids)), nil
	}
	env.Stack.ExportZipFn = func(_ context.Context, _ string, _ service.ExportZipParams, w io.Writer) error {
		zw := zip.NewWriter(w)
		return zw.Close()
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		testutil.JSONBody(map[string]any{"asset_ids": []string{"ast_1"}, "filename": "../../../etc/passwd"}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	cd := resp.Header.Get("Content-Disposition")
	if strings.Contains(cd, "/") || strings.Contains(cd, "\\") {
		t.Errorf("Content-Disposition contains path separators: %s", cd)
	}
}

func TestStackExport_Unauthenticated(t *testing.T) {
	env := testutil.NewTestEnv(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/stack/export",
		testutil.JSONBody(map[string]any{"asset_ids": []string{"x"}}))
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}
