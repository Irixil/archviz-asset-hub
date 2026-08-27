package jobspec

import (
	"encoding/json"
	"testing"
)

// TestPayloadWireCompatibility snapshots the JSON encoding of each payload
// moved from internal/jobs into this package. The exact key set and order
// must not change: these payloads are already persisted (as job.payload
// rows) and any in-flight job at deploy time is decoded by the new binary.
func TestPayloadWireCompatibility(t *testing.T) {
	title := "t"
	cases := []struct {
		name string
		v    any
		want string
	}{
		{
			"VersionThumbnailJobPayload",
			VersionThumbnailJobPayload{
				AssetID:     "asset-1",
				VersionID:   "version-1",
				WorkspaceID: "ws-1",
				StorageKey:  "key-1",
				MimeType:    "image/jpeg",
			},
			`{"asset_id":"asset-1","version_id":"version-1","workspace_id":"ws-1","storage_key":"key-1","mime_type":"image/jpeg"}`,
		},
		{
			"ExtractExifPayload",
			ExtractExifPayload{AssetID: "asset-1", WorkspaceID: "ws-1", UserID: "user-1"},
			`{"asset_id":"asset-1","workspace_id":"ws-1","user_id":"user-1"}`,
		},
		{
			"ExtractMediaTagsPayload",
			ExtractMediaTagsPayload{AssetID: "asset-1", WorkspaceID: "ws-1"},
			`{"asset_id":"asset-1","workspace_id":"ws-1"}`,
		},
		{
			"ExtractTextPayload",
			ExtractTextPayload{
				WorkspaceID: "ws-1",
				AssetID:     "asset-1",
				StorageKey:  "key-1",
				MimeType:    "application/pdf",
			},
			`{"workspace_id":"ws-1","asset_id":"asset-1","storage_key":"key-1","mime_type":"application/pdf"}`,
		},
		{
			"ExtractTextPayload_omitempty_mime_type",
			ExtractTextPayload{WorkspaceID: "ws-1", AssetID: "asset-1", StorageKey: "key-1"},
			`{"workspace_id":"ws-1","asset_id":"asset-1","storage_key":"key-1"}`,
		},
		{
			"VariantJobPayload",
			VariantJobPayload{
				AssetID:     "asset-1",
				WorkspaceID: "ws-1",
				VersionID:   "version-1",
				VersionNum:  2,
				VariantID:   "variant-1",
				StorageKey:  "key-1",
				MimeType:    "image/jpeg",
				Type:        "image_resize",
				Params:      json.RawMessage(`{"width":100}`),
				Title:       &title,
				IsShared:    true,
				Continuation: &NodeContinuation{
					RunID:       "run-1",
					NodeID:      "node-1",
					WorkflowID:  "workflow-1",
					WorkspaceID: "ws-1",
					ContextJSON: "{}",
				},
			},
			`{"asset_id":"asset-1","workspace_id":"ws-1","version_id":"version-1","version_num":2,` +
				`"variant_id":"variant-1","storage_key":"key-1","mime_type":"image/jpeg","type":"image_resize",` +
				`"params":{"width":100},"title":"t","is_shared":true,` +
				`"continuation":{"run_id":"run-1","node_id":"node-1","workflow_id":"workflow-1","workspace_id":"ws-1","context_json":"{}"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("wire format changed:\ngot:  %s\nwant: %s", got, tc.want)
			}
		})
	}
}
