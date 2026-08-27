package events

import (
	"encoding/json"
	"testing"
)

// TestConstructorsProduceExpectedType verifies each constructor sets the
// stable type string and encodes only its own fields — no leakage of empty
// strings for the other unrelated payload shapes.
func TestConstructorsProduceExpectedType(t *testing.T) {
	cases := []struct {
		name    string
		ev      Event
		typ     string
		payload map[string]any
	}{
		{
			name:    "ThumbnailReady",
			ev:      ThumbnailReady("asset_1", "thumb_1"),
			typ:     TypeThumbnailReady,
			payload: map[string]any{"asset_id": "asset_1", "thumbnail_key": "thumb_1"},
		},
		{
			name:    "ThumbnailReady omits empty key",
			ev:      ThumbnailReady("asset_1", ""),
			typ:     TypeThumbnailReady,
			payload: map[string]any{"asset_id": "asset_1"},
		},
		{
			name:    "VariantReady",
			ev:      VariantReady("asset_1", "variant_1"),
			typ:     TypeVariantReady,
			payload: map[string]any{"asset_id": "asset_1", "variant_id": "variant_1"},
		},
		{
			name:    "VariantFailed",
			ev:      VariantFailed("asset_1", "job_1", "boom"),
			typ:     TypeVariantFailed,
			payload: map[string]any{"asset_id": "asset_1", "job_id": "job_1", "error": "boom"},
		},
		{
			name:    "StackMergeDone",
			ev:      StackMergeDone("asset_1", "job_1"),
			typ:     TypeStackMergeDone,
			payload: map[string]any{"asset_id": "asset_1", "job_id": "job_1"},
		},
		{
			name: "VariantDraftReady",
			ev:   VariantDraftReady("nonce_1", "asset_1", "/preview", "2026-01-01T00:00:00Z"),
			typ:  TypeVariantDraftReady,
			payload: map[string]any{
				"nonce": "nonce_1", "asset_id": "asset_1",
				"preview_url": "/preview", "expires_at": "2026-01-01T00:00:00Z",
			},
		},
		{
			name:    "VariantDraftError",
			ev:      VariantDraftError("nonce_1", "boom"),
			typ:     TypeVariantDraftError,
			payload: map[string]any{"nonce": "nonce_1", "error": "boom"},
		},
		{
			name:    "WorkflowRunFailed",
			ev:      WorkflowRunFailed("asset_1", "wf_1", "run_1", "boom"),
			typ:     TypeWorkflowRunFailed,
			payload: map[string]any{"asset_id": "asset_1", "workflow_id": "wf_1", "run_id": "run_1", "error": "boom"},
		},
		{
			name: "WorkflowRunStepUpdated",
			ev:   WorkflowRunStepUpdated("asset_1", "wf_1", "run_1", "node_1", "completed", ""),
			typ:  TypeWorkflowRunStepUpdate,
			payload: map[string]any{
				"asset_id": "asset_1", "workflow_id": "wf_1",
				"run_id": "run_1", "node_id": "node_1", "status": "completed",
			},
		},
		{
			name:    "Resync",
			ev:      Resync("buffer overrun"),
			typ:     TypeResync,
			payload: map[string]any{"reason": "buffer overrun"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ev.Type != tc.typ {
				t.Fatalf("Type = %q, want %q", tc.ev.Type, tc.typ)
			}
			var got map[string]any
			if err := json.Unmarshal(tc.ev.Payload, &got); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			if len(got) != len(tc.payload) {
				t.Fatalf("payload = %#v, want %#v (field count mismatch — check omitempty/leakage)", got, tc.payload)
			}
			for k, v := range tc.payload {
				if got[k] != v {
					t.Errorf("payload[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

// TestEventEnvelopeEncoding checks the top-level envelope shape: id and type
// serialize, workspace_id never does, and an unset payload is omitted.
func TestEventEnvelopeEncoding(t *testing.T) {
	ev := ThumbnailReady("asset_1", "thumb_1")
	ev.ID = "42"
	ev.WorkspaceID = "ws_1"

	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if unmarshalErr := json.Unmarshal(b, &got); unmarshalErr != nil {
		t.Fatalf("unmarshal: %v", unmarshalErr)
	}
	if got["id"] != "42" {
		t.Errorf("id = %v, want 42", got["id"])
	}
	if got["type"] != TypeThumbnailReady {
		t.Errorf("type = %v, want %v", got["type"], TypeThumbnailReady)
	}
	if _, ok := got["workspace_id"]; ok {
		t.Error("workspace_id leaked into JSON output")
	}
	if _, ok := got["payload"]; !ok {
		t.Error("expected payload field present")
	}

	var empty Event
	b, err = json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	got = nil
	if unmarshalErr := json.Unmarshal(b, &got); unmarshalErr != nil {
		t.Fatalf("unmarshal empty: %v", unmarshalErr)
	}
	if _, ok := got["payload"]; ok {
		t.Error("expected payload omitted for zero-value Event")
	}
}
