package api

import (
	"testing"
)

func TestParseFolderFilter(t *testing.T) {
	ptr := func(s string) *string { return &s }

	tests := []struct {
		name         string
		folderParam  string
		projectParam string
		wantFolder   *string
		wantProject  *string
		wantIsRoot   bool
		wantErr      bool
	}{
		{
			name:        "no params",
			wantFolder:  nil,
			wantProject: nil,
			wantIsRoot:  false,
		},
		{
			name:         "project only",
			projectParam: "proj1",
			wantFolder:   nil,
			wantProject:  ptr("proj1"),
			wantIsRoot:   false,
		},
		{
			name:        "explicit folder id",
			folderParam: "folder42",
			wantFolder:  ptr("folder42"),
			wantProject: nil,
			wantIsRoot:  false,
		},
		{
			name:        "root requires project_id — missing",
			folderParam: "root",
			wantErr:     true,
		},
		{
			name:         "root with project_id",
			folderParam:  "root",
			projectParam: "proj1",
			wantFolder:   nil,
			wantProject:  ptr("proj1"),
			wantIsRoot:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			folderID, projectID, isRoot, err := parseFolderFilter(tc.folderParam, tc.projectParam)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			deref := func(p *string) string {
				if p == nil {
					return "<nil>"
				}
				return *p
			}
			if deref(folderID) != deref(tc.wantFolder) {
				t.Errorf("folderID: got %q, want %q", deref(folderID), deref(tc.wantFolder))
			}
			if deref(projectID) != deref(tc.wantProject) {
				t.Errorf("projectID: got %q, want %q", deref(projectID), deref(tc.wantProject))
			}
			if isRoot != tc.wantIsRoot {
				t.Errorf("isRoot: got %v, want %v", isRoot, tc.wantIsRoot)
			}
		})
	}
}
