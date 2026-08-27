// Package fixtures provides canned DTO values for use in handler tests.
// Each constructor accepts functional override options so tests can customise
// individual fields without repeating the full struct literal.
package fixtures

import (
	"time"

	"damask/server/internal/service"
	"damask/server/internal/transform"
)

var fixedTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

const (
	fixtureWorkspaceID = "ws_fixture_1"
	fixtureProjectID   = "prj_fixture_1"
)

// Asset returns a canned AssetDTO. Apply option funcs to override specific fields.
func Asset(overrides ...func(*service.AssetDTO)) *service.AssetDTO {
	a := &service.AssetDTO{
		ID:               "ast_fixture_1",
		WorkspaceID:      fixtureWorkspaceID,
		OriginalFilename: "fixture.jpg",
		StorageKey:       fixtureWorkspaceID + "/ast_fixture_1/original.jpg",
		MimeType:         transform.MimeImageJPEG,
		Size:             12345,
		CreatedAt:        fixedTime,
		UpdatedAt:        fixedTime,
	}
	for _, o := range overrides {
		o(a)
	}
	return a
}

// Project returns a canned ProjectDTO.
func Project(overrides ...func(*service.ProjectDTO)) *service.ProjectDTO {
	p := &service.ProjectDTO{
		ID:          fixtureProjectID,
		WorkspaceID: fixtureWorkspaceID,
		Name:        "Fixture Project",
		CreatedAt:   fixedTime,
		UpdatedAt:   fixedTime,
	}
	for _, o := range overrides {
		o(p)
	}
	return p
}

// Folder returns a canned FolderDTO.
func Folder(overrides ...func(*service.FolderDTO)) *service.FolderDTO {
	f := &service.FolderDTO{
		ID:          "fld_fixture_1",
		WorkspaceID: fixtureWorkspaceID,
		ProjectID:   fixtureProjectID,
		Name:        "Fixture Folder",
		CreatedAt:   fixedTime,
	}
	for _, o := range overrides {
		o(f)
	}
	return f
}

// Tag returns a canned TagDTO.
func Tag(overrides ...func(*service.TagDTO)) *service.TagDTO {
	t := &service.TagDTO{
		Name:        "fixture-tag",
		WorkspaceID: fixtureWorkspaceID,
		AssetCount:  1,
	}
	for _, o := range overrides {
		o(t)
	}
	return t
}

// Collection returns a canned CollectionDTO.
func Collection(overrides ...func(*service.CollectionDTO)) *service.CollectionDTO {
	c := &service.CollectionDTO{
		ID:          "col_fixture_1",
		WorkspaceID: fixtureWorkspaceID,
		Name:        "Fixture Collection",
		CreatedAt:   fixedTime,
		UpdatedAt:   fixedTime,
	}
	for _, o := range overrides {
		o(c)
	}
	return c
}

// Share returns a canned ShareDTO.
func Share(overrides ...func(*service.ShareDTO)) *service.ShareDTO {
	targetType := "project"
	targetID := fixtureProjectID
	s := &service.ShareDTO{
		ID:          "shr_fixture_1",
		WorkspaceID: fixtureWorkspaceID,
		TargetType:  targetType,
		TargetID:    targetID,
		CreatedAt:   fixedTime,
	}
	for _, o := range overrides {
		o(s)
	}
	return s
}

// Version returns a canned VersionDTO.
func Version(overrides ...func(*service.VersionDTO)) *service.VersionDTO {
	v := &service.VersionDTO{
		ID:          "ver_fixture_1",
		AssetID:     "ast_fixture_1",
		WorkspaceID: fixtureWorkspaceID,
		VersionNum:  1,
		StorageKey:  "ws_fixture_1/ast_fixture_1/v1/original.jpg",
		MimeType:    transform.MimeImageJPEG,
		Size:        12345,
		IsCurrent:   true,
		CreatedAt:   fixedTime,
	}
	for _, o := range overrides {
		o(v)
	}
	return v
}
