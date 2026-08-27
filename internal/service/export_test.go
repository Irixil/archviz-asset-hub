package service_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"damask/server/internal/repository"
	"damask/server/internal/service"
)

func TestStampWorkspaceID(t *testing.T) {
	t.Parallel()

	// valid JSON — workspace_id is injected
	raw := json.RawMessage(`{"host":"sftp.example.com"}`)
	out, err := service.StampWorkspaceID(raw, "ws_42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]any
	if err = json.Unmarshal(out, &m); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if m["workspace_id"] != "ws_42" {
		t.Errorf("workspace_id not stamped: %v", m)
	}
	if m["host"] != "sftp.example.com" {
		t.Errorf("existing key lost: %v", m)
	}

	// invalid JSON — returns error
	_, err = service.StampWorkspaceID(json.RawMessage(`not-json`), "ws_1")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	// existing workspace_id key is overwritten
	raw2 := json.RawMessage(`{"workspace_id":"old"}`)
	out2, err := service.StampWorkspaceID(raw2, "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m2 map[string]any
	_ = json.Unmarshal(out2, &m2)
	if m2["workspace_id"] != "new" {
		t.Errorf("workspace_id not overwritten: %v", m2)
	}

	_ = errors.New("") // keep import
}

func TestExportConfigToDTO(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	status := "ok"
	errStr := "some error"
	mins := 30

	in := repository.ExportConfig{
		ID:              "cfg_1",
		WorkspaceID:     "ws_1",
		ProjectID:       "proj_1",
		Label:           "My Export",
		DestType:        "sftp",
		Versions:        "current",
		IncludeVariants: true,
		ScheduleType:    "after_quiet",
		QuietMinutes:    &mins,
		Enabled:         true,
		LastRunAt:       &now,
		LastRunStatus:   &status,
		LastError:       &errStr,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	got := service.ExportConfigToDTO(in)
	if got.ID != in.ID || got.WorkspaceID != in.WorkspaceID || got.ProjectID != in.ProjectID {
		t.Errorf("ID/WorkspaceID/ProjectID mismatch: %+v", got)
	}
	if got.Label != in.Label || got.DestType != in.DestType || got.Versions != in.Versions {
		t.Errorf("Label/DestType/Versions mismatch: %+v", got)
	}
	if got.IncludeVariants != in.IncludeVariants || got.ScheduleType != in.ScheduleType {
		t.Errorf("IncludeVariants/ScheduleType mismatch: %+v", got)
	}
	if got.QuietMinutes != in.QuietMinutes || got.Enabled != in.Enabled {
		t.Errorf("QuietMinutes/Enabled mismatch: %+v", got)
	}
	if got.LastRunAt != in.LastRunAt || got.LastRunStatus != in.LastRunStatus || got.LastError != in.LastError {
		t.Errorf("LastRunAt/LastRunStatus/LastError mismatch: %+v", got)
	}
}

func TestExportRunToDTO(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	triggeredBy := "user_1"
	errStr := "something failed"

	in := repository.ExportRun{
		ID:             "run_1",
		ExportConfigID: "cfg_1",
		TriggeredBy:    &triggeredBy,
		Status:         "completed",
		AssetsTotal:    10,
		AssetsExported: 8,
		AssetsSkipped:  2,
		BytesWritten:   1024 * 1024,
		Error:          &errStr,
		StartedAt:      &now,
		CompletedAt:    &now,
		CreatedAt:      now,
	}

	got := service.ExportRunToDTO(in)
	if got.ID != in.ID || got.ExportConfigID != in.ExportConfigID {
		t.Errorf("ID/ExportConfigID mismatch: %+v", got)
	}
	if got.TriggeredBy != in.TriggeredBy || got.Status != in.Status {
		t.Errorf("TriggeredBy/Status mismatch: %+v", got)
	}
	if got.AssetsTotal != in.AssetsTotal || got.AssetsExported != in.AssetsExported ||
		got.AssetsSkipped != in.AssetsSkipped {
		t.Errorf("asset counts mismatch: %+v", got)
	}
	if got.BytesWritten != in.BytesWritten || got.Error != in.Error {
		t.Errorf("BytesWritten/Error mismatch: %+v", got)
	}
	if got.StartedAt != in.StartedAt || got.CompletedAt != in.CompletedAt || got.CreatedAt != in.CreatedAt {
		t.Errorf("timestamps mismatch: %+v", got)
	}
}
