package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"damask/server/internal/ai"
	"damask/server/internal/repository"
)

// --- helpers ---

func rc(kv ...any) *RunContext {
	data := map[string]any{}
	for i := 0; i < len(kv)-1; i += 2 {
		data[kv[i].(string)] = kv[i+1]
	}
	return NewRunContext(data)
}

func cfg(raw string) json.RawMessage { return json.RawMessage(raw) }

// --- passThroughNode (triggers + control.fan_out) ---

func TestTriggerNodes_SchemaAndExecute(t *testing.T) {
	t.Parallel()
	types := []string{
		"trigger.manual",
		"trigger.asset_created",
		"trigger.version_uploaded",
		"trigger.tag_added",
		"trigger.schedule",
		"trigger.webhook",
		"control.fan_out",
	}
	for _, nodeType := range types {
		t.Run(nodeType, func(t *testing.T) {
			t.Parallel()
			node, err := Build(Deps{}, nodeType)
			if err != nil {
				t.Fatalf("Build(%q): %v", nodeType, err)
			}
			schema := node.Schema()
			if schema.Type != nodeType {
				t.Errorf("Schema.Type: got %q, want %q", schema.Type, nodeType)
			}
			if len(schema.Outputs) < 2 {
				t.Errorf("expected at least 2 outputs, got %d", len(schema.Outputs))
			}
			port, updates, err := node.Execute(context.Background(), rc(), cfg(`{}`))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if port != portOut {
				t.Errorf("port: got %q, want %q", port, portOut)
			}
			if updates != nil {
				t.Errorf("expected nil updates, got %v", updates)
			}
		})
	}
}

// --- filterNode ---

func buildFilter(t *testing.T, nodeType string) Node {
	t.Helper()
	node, err := Build(Deps{}, nodeType)
	if err != nil {
		t.Fatalf("Build(%q): %v", nodeType, err)
	}
	return node
}

func TestFilterMime_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.mime")
	port, _, err := node.Execute(context.Background(),
		rc("mime_type", "image/jpeg"),
		cfg(`{"prefix":"image/"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected %q, got %q", portMatch, port)
	}
}

func TestFilterMime_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.mime")
	port, _, err := node.Execute(context.Background(),
		rc("mime_type", "video/mp4"),
		cfg(`{"prefix":"image/"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterFilename_Contains_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.filename")
	port, _, err := node.Execute(context.Background(),
		rc("filename", "product_hero.jpg"),
		cfg(`{"contains":"hero"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterFilename_Extension_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.filename")
	port, _, err := node.Execute(context.Background(),
		rc("filename", "document.pdf"),
		cfg(`{"extension":".jpg"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterSize_InRange(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.size")
	port, _, err := node.Execute(context.Background(),
		rc("size", float64(5000)),
		cfg(`{"min":1000,"max":10000}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterSize_OutOfRange(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.size")
	port, _, err := node.Execute(context.Background(),
		rc("size", float64(500)),
		cfg(`{"min":1000}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterTag_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.tag")
	port, _, err := node.Execute(context.Background(),
		rc("tag_name", "Hero"),
		cfg(`{"name":"hero"}`)) // case-insensitive
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterTag_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.tag")
	port, _, err := node.Execute(context.Background(),
		rc("tag_name", "draft"),
		cfg(`{"name":"hero"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

func TestFilterFolder_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.folder")
	port, _, err := node.Execute(context.Background(),
		rc("folder_id", "fld_1"),
		cfg(`{"folder_id":"fld_1"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterExpression_Match(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.expression")
	port, _, err := node.Execute(context.Background(),
		rc("status", "ready"),
		cfg(`{"key":"status","value":"ready"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portMatch {
		t.Errorf("expected match, got %q", port)
	}
}

func TestFilterExpression_NoMatch(t *testing.T) {
	t.Parallel()
	node := buildFilter(t, "filter.expression")
	port, _, err := node.Execute(context.Background(),
		rc("status", "pending"),
		cfg(`{"key":"status","value":"ready"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != "no_match" {
		t.Errorf("expected no_match, got %q", port)
	}
}

// --- rcGetString / rcRequireString ---

func TestRcGetString_String(t *testing.T) {
	t.Parallel()
	r := rc("key", "value")
	v, ok := rcGetString(r, "key")
	if !ok || v != "value" {
		t.Fatalf("expected (value, true), got (%q, %v)", v, ok)
	}
}

func TestRcGetString_Float(t *testing.T) {
	t.Parallel()
	r := rc("size", float64(42))
	v, ok := rcGetString(r, "size")
	if !ok || v == "" {
		t.Fatalf("expected non-empty string, got (%q, %v)", v, ok)
	}
}

func TestRcGetString_Missing(t *testing.T) {
	t.Parallel()
	r := rc()
	v, ok := rcGetString(r, "missing")
	if ok || v != "" {
		t.Fatalf("expected (\"\", false), got (%q, %v)", v, ok)
	}
}

func TestRcRequireString_Missing(t *testing.T) {
	t.Parallel()
	r := rc()
	_, err := rcRequireString(r, "required_key")
	if err == nil {
		t.Fatal("expected error for missing required key")
	}
}

// --- stub managers for action node tests ---

type stubTagManager struct {
	addedTag string
}

func (s *stubTagManager) AddToAsset(_ context.Context, _, _, tagName string) (string, error) {
	s.addedTag = tagName
	return tagName, nil
}

type stubAssetManager struct {
	movedFolderID  *string
	movedProjectID *string
}

func (s *stubAssetManager) Get(_ context.Context, _, _ string) (*Asset, error) {
	return &Asset{ID: "ast_1", WorkspaceID: "ws_1", MimeType: "image/jpeg"}, nil
}

func (s *stubAssetManager) Move(_ context.Context, _, _ string, p AssetMoveParams) (*Asset, error) {
	s.movedFolderID = p.FolderID
	s.movedProjectID = p.ProjectID
	return &Asset{ID: "ast_1", WorkspaceID: "ws_1", FolderID: p.FolderID, ProjectID: p.ProjectID}, nil
}

type stubAssetManagerWithMime struct {
	mimeType string
}

func (s *stubAssetManagerWithMime) Get(_ context.Context, _, _ string) (*Asset, error) {
	versionID := "ver_1"
	return &Asset{ID: "ast_1", WorkspaceID: "ws_1", MimeType: s.mimeType, CurrentVersionID: &versionID}, nil
}

func (s *stubAssetManagerWithMime) Move(_ context.Context, _, _ string, p AssetMoveParams) (*Asset, error) {
	return &Asset{ID: "ast_1", WorkspaceID: "ws_1", FolderID: p.FolderID, ProjectID: p.ProjectID}, nil
}

type stubShareManager struct {
	lastShareID string
}

func (s *stubShareManager) Create(_ context.Context, _ string, _ ShareCreateParams) (string, error) {
	s.lastShareID = "sh_new"
	return s.lastShareID, nil
}

// --- action node tests ---

func TestActionTag_Execute_OK(t *testing.T) {
	t.Parallel()
	tags := &stubTagManager{}
	node := tagAssetNode{
		deps: Deps{Tags: tags},
		schema: actionSchema("action.tag", "Tag Asset", ".",
			mustConfigSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`)),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1", "workflow_created_by", "usr_1")
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{"name":"hero"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if updates["tag_name"] != "hero" {
		t.Errorf("expected tag_name=hero in updates, got %v", updates)
	}
	if tags.addedTag != "hero" {
		t.Errorf("expected stub to record tag hero, got %q", tags.addedTag)
	}
}

func TestActionMoveFolder_Execute_OK(t *testing.T) {
	t.Parallel()
	assets := &stubAssetManager{}
	folder := "fld_42"
	node := moveAssetNode{
		deps: Deps{Assets: assets},
		schema: actionSchema("action.move_folder", "Move Asset", ".",
			mustConfigSchema(`{"type":"object","properties":{}}`)),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{"folder_id":"`+folder+`"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if assets.movedFolderID == nil || *assets.movedFolderID != folder {
		t.Errorf("expected folder_id=%q to be moved, got %v", folder, assets.movedFolderID)
	}
	_ = updates
}

// --- stubs for metadata.ai_image_description ---

type stubVersionManager struct {
	version repository.AssetVersion
}

func (s *stubVersionManager) GetByID(_ context.Context, _ string) (repository.AssetVersion, error) {
	return s.version, nil
}

func (s *stubVersionManager) NextVersionNum(_ context.Context, _ string) (int64, error) {
	return 1, nil
}

func (s *stubVersionManager) Create(
	_ context.Context,
	v repository.AssetVersion,
) (repository.AssetVersion, error) {
	return v, nil
}

func (s *stubVersionManager) SetCurrent(_ context.Context, _, _ string) error { return nil }

func (s *stubVersionManager) SetAssetThumbnail(_ context.Context, _ string, _ *string) error {
	return nil
}

type stubWorkspaceManager struct {
	providers []AIProviderStatus
	err       error
}

func (s *stubWorkspaceManager) ListAIProviders(
	_ context.Context, _ string, _ ai.Capability,
) ([]AIProviderStatus, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.providers, nil
}

type stubTextTrackManager struct {
	trackID  string
	lastCall TextTrackCreateParams
	lastOCR  TextTrackCreateOCRParams
	err      error
}

func (s *stubTextTrackManager) CreateAIImageDescription(
	_ context.Context,
	_ string,
	p TextTrackCreateParams,
) (string, error) {
	s.lastCall = p
	if s.err != nil {
		return "", s.err
	}
	return s.trackID, nil
}

func (s *stubTextTrackManager) CreateOCR(
	_ context.Context,
	_ string,
	p TextTrackCreateOCRParams,
) (string, error) {
	s.lastOCR = p
	if s.err != nil {
		return "", s.err
	}
	return s.trackID, nil
}

// --- metadata.ai_image_description node tests ---

func aiImageDescDeps(workspace *stubWorkspaceManager, tracks *stubTextTrackManager) Deps {
	return Deps{
		Assets:     &stubAssetManagerWithMime{mimeType: "image/jpeg"},
		Versions:   &stubVersionManager{version: repository.AssetVersion{StorageKey: "sk1", MimeType: "image/jpeg"}},
		Workspace:  workspace,
		TextTracks: tracks,
	}
}

func TestActionAIImageDescription_Execute_OK(t *testing.T) {
	t.Parallel()
	tracks := &stubTextTrackManager{trackID: "trk_1"}
	node := aiImageDescriptionNode{
		deps: aiImageDescDeps(&stubWorkspaceManager{
			providers: []AIProviderStatus{{ID: ai.ProviderOpenRouter, Configured: true}},
		}, tracks),
		schema: aiImageDescriptionSchemaFn(),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if updates["track_id"] != "trk_1" {
		t.Errorf("expected track_id=trk_1 in updates, got %v", updates)
	}
	if tracks.lastCall.Model != "google/gemini-2.5-flash" {
		t.Errorf("expected default model, got %q", tracks.lastCall.Model)
	}
}

func TestActionAIImageDescription_Execute_NotConfigured(t *testing.T) {
	t.Parallel()
	tracks := &stubTextTrackManager{trackID: "trk_1"}
	node := aiImageDescriptionNode{
		deps:   aiImageDescDeps(&stubWorkspaceManager{}, tracks),
		schema: aiImageDescriptionSchemaFn(),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	_, _, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err == nil {
		t.Fatal("expected error when OpenRouter is not configured")
	}
	if tracks.lastCall.AssetID != "" {
		t.Errorf("expected CreateAIImageDescription to not be called, got %v", tracks.lastCall)
	}
}

func TestActionAIImageDescription_Execute_NonImageMime(t *testing.T) {
	t.Parallel()
	tracks := &stubTextTrackManager{trackID: "trk_1"}
	deps := aiImageDescDeps(&stubWorkspaceManager{
		providers: []AIProviderStatus{{ID: ai.ProviderOpenRouter, Configured: true}},
	}, tracks)
	deps.Assets = &stubAssetManagerWithMime{mimeType: "video/mp4"}
	node := aiImageDescriptionNode{deps: deps, schema: aiImageDescriptionSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	_, _, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err == nil {
		t.Fatal("expected error for non-image asset")
	}
}

func TestActionAIImageDescription_Execute_WithContinuation(t *testing.T) {
	t.Parallel()
	tracks := &stubTextTrackManager{trackID: "trk_1"}
	node := aiImageDescriptionNode{
		deps: aiImageDescDeps(&stubWorkspaceManager{
			providers: []AIProviderStatus{{ID: ai.ProviderOpenRouter, Configured: true}},
		}, tracks),
		schema: aiImageDescriptionSchemaFn(),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	triggerRC.Set(rcKeyContinuation, NodeContinuation{
		RunID: "run_1", NodeID: "n_tag", WorkflowID: "wf_1", WorkspaceID: "ws_1",
	})
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portContinued {
		t.Errorf("expected portContinued, got %q", port)
	}
	if updates["track_id"] != "trk_1" {
		t.Errorf("expected track_id=trk_1 in updates, got %v", updates)
	}
	if tracks.lastCall.Continuation == nil {
		t.Fatal("expected continuation to be passed to TextTracks.CreateAIImageDescription")
	}
}

func TestActionAIImageDescription_InjectContinuation_AnySuccessor(t *testing.T) {
	t.Parallel()
	node := aiImageDescriptionNode{schema: aiImageDescriptionSchemaFn()}
	g := &Graph{
		Nodes: []GraphNode{
			{ID: "n_desc", Type: nodeTypeAIImageDesc},
			{ID: "n_tag", Type: "action.tag"},
		},
		Edges: []GraphEdge{
			{FromNode: "n_desc", FromPort: portOut, ToNode: "n_tag", ToPort: "in"},
		},
	}
	runRC := rc()
	node.InjectContinuation(runRC, g, "n_desc", "run_1", "wf_1", "ws_1")
	val, ok := runRC.Get(rcKeyContinuation)
	if !ok {
		t.Fatal("expected continuation to be seeded")
	}
	cont, ok := val.(NodeContinuation)
	if !ok || cont.NodeID != "n_tag" {
		t.Errorf("expected continuation targeting n_tag, got %v", val)
	}
}

// --- action.ocr node tests ---

func ocrDeps(tracks *stubTextTrackManager) Deps {
	return Deps{
		Assets:     &stubAssetManagerWithMime{mimeType: "image/png"},
		Versions:   &stubVersionManager{version: repository.AssetVersion{StorageKey: "sk1", MimeType: "image/png"}},
		TextTracks: tracks,
	}
}

func TestActionOCR_Execute_OK(t *testing.T) {
	t.Parallel()
	tracks := &stubTextTrackManager{trackID: "trk_1"}
	node := ocrNode{deps: ocrDeps(tracks), schema: ocrSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if updates["track_id"] != "trk_1" {
		t.Errorf("expected track_id=trk_1 in updates, got %v", updates)
	}
	if tracks.lastOCR.Lang != "eng" {
		t.Errorf("expected default lang=eng, got %q", tracks.lastOCR.Lang)
	}
	if tracks.lastOCR.OutputFormat != ocrOutputFormatTxt {
		t.Errorf("expected default output_format=txt, got %q", tracks.lastOCR.OutputFormat)
	}
}

func TestActionOCR_Execute_UnsupportedMime(t *testing.T) {
	t.Parallel()
	tracks := &stubTextTrackManager{trackID: "trk_1"}
	deps := ocrDeps(tracks)
	deps.Assets = &stubAssetManagerWithMime{mimeType: "video/mp4"}
	node := ocrNode{deps: deps, schema: ocrSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	_, _, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err == nil {
		t.Fatal("expected error for unsupported mime type")
	}
}

func TestActionOCR_Execute_WithContinuation(t *testing.T) {
	t.Parallel()
	tracks := &stubTextTrackManager{trackID: "trk_1"}
	node := ocrNode{deps: ocrDeps(tracks), schema: ocrSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	triggerRC.Set(rcKeyContinuation, NodeContinuation{
		RunID: "run_1", NodeID: "n_tag", WorkflowID: "wf_1", WorkspaceID: "ws_1",
	})
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portContinued {
		t.Errorf("expected portContinued, got %q", port)
	}
	if updates["track_id"] != "trk_1" {
		t.Errorf("expected track_id=trk_1 in updates, got %v", updates)
	}
	if tracks.lastOCR.Continuation == nil {
		t.Fatal("expected continuation to be passed to TextTracks.CreateOCR")
	}
}

func TestActionOCR_InjectContinuation_AnySuccessor(t *testing.T) {
	t.Parallel()
	node := ocrNode{schema: ocrSchemaFn()}
	g := &Graph{
		Nodes: []GraphNode{
			{ID: "n_ocr", Type: nodeTypeOCR},
			{ID: "n_tag", Type: "action.tag"},
		},
		Edges: []GraphEdge{
			{FromNode: "n_ocr", FromPort: portOut, ToNode: "n_tag", ToPort: "in"},
		},
	}
	runRC := rc()
	node.InjectContinuation(runRC, g, "n_ocr", "run_1", "wf_1", "ws_1")
	val, ok := runRC.Get(rcKeyContinuation)
	if !ok {
		t.Fatal("expected continuation to be seeded")
	}
	cont, ok := val.(NodeContinuation)
	if !ok || cont.NodeID != "n_tag" {
		t.Errorf("expected continuation targeting n_tag, got %v", val)
	}
}

// --- action.auto_tag node tests ---

type stubAutoTagManager struct {
	lastWorkspaceID, lastAssetID, lastMode string
	lastContinuation                       *NodeContinuation
	err                                    error
}

func (s *stubAutoTagManager) Enqueue(
	_ context.Context,
	workspaceID, assetID, mode string,
	continuation *NodeContinuation,
) error {
	s.lastWorkspaceID = workspaceID
	s.lastAssetID = assetID
	s.lastMode = mode
	s.lastContinuation = continuation
	return s.err
}

func autoTagDeps(autoTag *stubAutoTagManager) Deps {
	return Deps{
		Assets:  &stubAssetManagerWithMime{mimeType: "image/jpeg"},
		AutoTag: autoTag,
	}
}

func TestActionAutoTag_Execute_OK(t *testing.T) {
	t.Parallel()
	autoTag := &stubAutoTagManager{}
	node := autoTagNode{deps: autoTagDeps(autoTag), schema: autoTagSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	port, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if updates["mode"] != autoTagModePending {
		t.Errorf("expected default mode=pending, got %v", updates)
	}
	if autoTag.lastAssetID != "ast_1" || autoTag.lastWorkspaceID != "ws_1" {
		t.Errorf("expected Enqueue called with ws_1/ast_1, got %s/%s", autoTag.lastWorkspaceID, autoTag.lastAssetID)
	}
}

func TestActionAutoTag_Execute_SilentMode(t *testing.T) {
	t.Parallel()
	autoTag := &stubAutoTagManager{}
	node := autoTagNode{deps: autoTagDeps(autoTag), schema: autoTagSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	_, updates, err := node.Execute(context.Background(), triggerRC, cfg(`{"mode":"silent"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updates["mode"] != autoTagModeSilent {
		t.Errorf("expected mode=silent, got %v", updates)
	}
	if autoTag.lastMode != autoTagModeSilent {
		t.Errorf("expected Enqueue called with silent mode, got %q", autoTag.lastMode)
	}
}

func TestActionAutoTag_Execute_InvalidMode(t *testing.T) {
	t.Parallel()
	autoTag := &stubAutoTagManager{}
	node := autoTagNode{deps: autoTagDeps(autoTag), schema: autoTagSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	_, _, err := node.Execute(context.Background(), triggerRC, cfg(`{"mode":"bogus"}`))
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestActionAutoTag_Execute_NonTaggableMime(t *testing.T) {
	t.Parallel()
	autoTag := &stubAutoTagManager{}
	deps := autoTagDeps(autoTag)
	deps.Assets = &stubAssetManagerWithMime{mimeType: "application/zip"}
	node := autoTagNode{deps: deps, schema: autoTagSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	_, _, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err == nil {
		t.Fatal("expected error for non-taggable mime type")
	}
	if autoTag.lastAssetID != "" {
		t.Errorf("expected Enqueue to not be called, got %v", autoTag)
	}
}

func TestActionAutoTag_Execute_WithContinuation(t *testing.T) {
	t.Parallel()
	autoTag := &stubAutoTagManager{}
	node := autoTagNode{deps: autoTagDeps(autoTag), schema: autoTagSchemaFn()}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1")
	triggerRC.Set(rcKeyContinuation, NodeContinuation{
		RunID: "run_1", NodeID: "n_next", WorkflowID: "wf_1", WorkspaceID: "ws_1",
	})
	port, _, err := node.Execute(context.Background(), triggerRC, cfg(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portContinued {
		t.Errorf("expected portContinued, got %q", port)
	}
	if autoTag.lastContinuation == nil {
		t.Fatal("expected continuation to be passed to AutoTag.Enqueue")
	}
}

func TestActionAutoTag_InjectContinuation_AnySuccessor(t *testing.T) {
	t.Parallel()
	node := autoTagNode{schema: autoTagSchemaFn()}
	g := &Graph{
		Nodes: []GraphNode{
			{ID: "n_auto_tag", Type: nodeTypeAutoTag},
			{ID: "n_next", Type: "action.tag"},
		},
		Edges: []GraphEdge{
			{FromNode: "n_auto_tag", FromPort: portOut, ToNode: "n_next", ToPort: "in"},
		},
	}
	runRC := rc()
	node.InjectContinuation(runRC, g, "n_auto_tag", "run_1", "wf_1", "ws_1")
	val, ok := runRC.Get(rcKeyContinuation)
	if !ok {
		t.Fatal("expected continuation to be seeded")
	}
	cont, ok := val.(NodeContinuation)
	if !ok || cont.NodeID != "n_next" {
		t.Errorf("expected continuation targeting n_next, got %v", val)
	}
}

func TestActionShare_Execute_OK(t *testing.T) {
	t.Parallel()
	shares := &stubShareManager{}
	node := createShareNode{
		deps: Deps{Shares: shares},
		schema: actionSchema("action.share", "Create Share", ".",
			mustConfigSchema(`{"type":"object","properties":{}}`)),
	}
	triggerRC := rc("workspace_id", "ws_1", "asset_id", "ast_1", "workflow_created_by", "usr_1")
	port, updates, err := node.Execute(
		context.Background(),
		triggerRC,
		cfg(`{"label":"My Share","allow_download":true}`),
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if port != portOut {
		t.Errorf("expected portOut, got %q", port)
	}
	if updates["share_id"] == "" {
		t.Errorf("expected non-empty share_id in updates, got %v", updates)
	}
}
