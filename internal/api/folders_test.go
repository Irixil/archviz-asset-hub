//go:build integration

package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
	"damask/server/internal/testutil/fixtures"
)

// folderDTO builds a FolderDTO fixture with the given id, projectID and name.
func folderDTO(id, projectID, name string, parentID *string) *service.FolderDTO {
	return fixtures.Folder(func(f *service.FolderDTO) {
		f.ID = id
		f.ProjectID = projectID
		f.Name = name
		f.ParentID = parentID
	})
}

func TestCreateFolder_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Folders.CreateFn = func(_ context.Context, _, projectID string, p service.CreateFolderParams) (*service.FolderDTO, error) {
		return folderDTO("fld_1", projectID, p.Name, p.ParentID), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/projects/prj_1/folders",
		testutil.JSONBody(api.CreateFolderRequest{Name: "Assets"}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)

	var folder api.FolderResponse
	testutil.DecodeJSON(t, resp, &folder)
	if folder.Name != "Assets" {
		t.Errorf("name = %q, want Assets", folder.Name)
	}
	if folder.ProjectID != "prj_1" {
		t.Errorf("project_id = %q, want prj_1", folder.ProjectID)
	}
}

func TestCreateFolder_SubfolderSuccess(t *testing.T) {
	env := testutil.NewTestEnv(t)
	parentID := "fld_root"
	env.Folders.CreateFn = func(_ context.Context, _, projectID string, p service.CreateFolderParams) (*service.FolderDTO, error) {
		return folderDTO("fld_sub", projectID, p.Name, p.ParentID), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/projects/prj_1/folders",
		testutil.JSONBody(api.CreateFolderRequest{Name: "Sub", ParentID: &parentID}), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)
}

func TestCreateFolder_DuplicateName(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Folders.CreateFn = func(_ context.Context, _, _ string, _ service.CreateFolderParams) (*service.FolderDTO, error) {
		return nil, fmt.Errorf("duplicate name: %w", apperr.ErrConflict)
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodPost, "/api/v1/projects/prj_1/folders",
		testutil.JSONBody(api.CreateFolderRequest{Name: "Dupe"}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusConflict)
}

func TestGetFolders_Tree(t *testing.T) {
	env := testutil.NewTestEnv(t)
	childID := "fld_child"
	env.Folders.ListTreeFn = func(_ context.Context, _, _ string) ([]*service.FolderTreeDTO, error) {
		return []*service.FolderTreeDTO{
			{
				ID:        "fld_root",
				ProjectID: "prj_1",
				Name:      "Root",
				Children: []*service.FolderTreeDTO{
					{ID: childID, ProjectID: "prj_1", Name: "Child"},
				},
			},
		}, nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodGet, "/api/v1/projects/prj_1/folders", nil, cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var tree []api.FolderResponse
	testutil.DecodeJSON(t, resp, &tree)
	if len(tree) != 1 {
		t.Fatalf("expected 1 root folder, got %d", len(tree))
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(tree[0].Children))
	}
}

func TestUpdateFolder_Rename(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Folders.UpdateFn = func(_ context.Context, _, id string, p service.UpdateFolderParams) (*service.FolderDTO, error) {
		return folderDTO(id, "prj_1", *p.Name, nil), nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	newName := "NewName"
	req := testutil.AuthRequest(http.MethodPut, "/api/v1/folders/fld_1",
		testutil.JSONBody(api.UpdateFolderRequest{Name: &newName}), cookie)
	resp, _ := env.App.Test(req)
	testutil.AssertStatus(t, resp, http.StatusOK)

	var updated api.FolderResponse
	testutil.DecodeJSON(t, resp, &updated)
	if updated.Name != "NewName" {
		t.Errorf("name = %q, want NewName", updated.Name)
	}
}

func TestDeleteFolder_Success(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Folders.DeleteFn = func(_ context.Context, _, _ string) error { return nil }
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	req := testutil.AuthRequest(http.MethodDelete, "/api/v1/folders/fld_1", nil, cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusNoContent)
}
