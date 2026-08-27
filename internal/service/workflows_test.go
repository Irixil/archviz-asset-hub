package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/workflow"
)

type workflowQueueStub struct {
	enqueued int
	types    []string
}

func (q *workflowQueueStub) Register(string, queue.HandlerFunc) {}
func (q *workflowQueueStub) Start(context.Context)              {}
func (q *workflowQueueStub) Stop()                              {}

func (q *workflowQueueStub) Enqueue(_ context.Context, _, jobType, _ string) (dbgen.Job, error) {
	q.enqueued++
	q.types = append(q.types, jobType)
	return dbgen.Job{ID: "job_1"}, nil
}

func newWorkflowSvc(
	t *testing.T,
) (service.WorkflowService, *memory.WorkflowRepo, *memory.WorkflowRunRepo, *memory.WorkflowWebhookRepo, *workflowQueueStub) {
	t.Helper()
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	webhooks := memory.NewWorkflowWebhookRepo()
	queue := &workflowQueueStub{}
	svc := service.NewWorkflowService(
		workflows,
		runs,
		webhooks,
		queue,
		service.WorkflowServiceDeps{},
	)
	return svc, workflows, runs, webhooks, queue
}

func TestWorkflowServiceGetWrongWorkspace(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_a"})
	_, err := svc.Get(context.Background(), "ws_b", "wf_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkflowServiceCreateOK(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", "usr_1", service.CreateWorkflowParams{
		Name:                 "My Workflow",
		Graph:                `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
		NotifyOnFailureEmail: "ops@example.com",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if !dto.Enabled {
		t.Fatal("expected new workflow to be enabled")
	}
	if dto.NotifyOnFailureEmail != "ops@example.com" {
		t.Fatalf("expected notify email to be persisted, got %q", dto.NotifyOnFailureEmail)
	}
}

func TestWorkflowServiceTriggerManualEnqueuesRun(t *testing.T) {
	svc, repo, _, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	runID, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "")
	if err != nil {
		t.Fatalf("TriggerManual() unexpected error: %v", err)
	}
	if runID == "" || queue.enqueued != 1 {
		t.Fatalf("expected run to be created and enqueued, run_id=%q enqueued=%d", runID, queue.enqueued)
	}
}

func TestWorkflowServiceTriggerManual_NoAsset_TriggerDataIsManualOnly(t *testing.T) {
	svc, repo, runs, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	runID, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "")
	if err != nil {
		t.Fatalf("TriggerManual() unexpected error: %v", err)
	}
	run, err := runs.GetByID(context.Background(), runID)
	if err != nil {
		t.Fatalf("run not found: %v", err)
	}
	var td map[string]any
	if decodeErr := json.Unmarshal([]byte(run.TriggerData), &td); decodeErr != nil {
		t.Fatalf("invalid trigger_data JSON: %v", decodeErr)
	}
	if td["trigger"] != "manual" {
		t.Fatalf("trigger: got %v, want manual", td["trigger"])
	}
	if _, ok := td["asset_id"]; ok {
		t.Fatal("expected no asset_id in trigger_data when assetID is empty")
	}
}

func TestWorkflowServiceTriggerManual_WithAsset_TriggerDataHasAssetContext(t *testing.T) {
	svc, repo, runs, assets, versions, _ := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	assets.Seed(repository.Asset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "photo.jpg",
		MimeType:         "image/jpeg",
		Size:             512,
	})
	versions.Seed(repository.AssetVersion{
		ID:          "ver_1",
		AssetID:     "ast_1",
		WorkspaceID: "ws_1",
		VersionNum:  1,
		StorageKey:  "ws_1/ast_1/v1.jpg",
		IsCurrent:   true,
	})

	runID, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "ast_1")
	if err != nil {
		t.Fatalf("TriggerManual() unexpected error: %v", err)
	}
	run, err := runs.GetByID(context.Background(), runID)
	if err != nil {
		t.Fatalf("run not found: %v", err)
	}
	var td map[string]any
	if decodeErr := json.Unmarshal([]byte(run.TriggerData), &td); decodeErr != nil {
		t.Fatalf("invalid trigger_data JSON: %v", decodeErr)
	}
	checks := map[string]any{
		"asset_id":   "ast_1",
		"version_id": "ver_1",
		"mime_type":  "image/jpeg",
		"project_id": "",
		"folder_id":  "",
	}
	for key, want := range checks {
		got, ok := td[key]
		if !ok {
			t.Errorf("trigger_data missing key %q", key)
			continue
		}
		if got != want {
			t.Errorf("trigger_data[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestWorkflowServiceTriggerManual_UnknownAsset_ReturnsNotFound(t *testing.T) {
	svc, repo, _, assets, _, _ := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	// asset exists in a different workspace
	assets.Seed(repository.Asset{ID: "ast_other", WorkspaceID: "ws_other", OriginalFilename: "x.jpg"})
	_, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "ast_other")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkflowServiceListFilterManualEnabled(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(
		repository.Workflow{
			ID:          "wf_1",
			WorkspaceID: "ws_1",
			Name:        "Manual",
			Enabled:     true,
			TriggerType: "trigger.manual",
		},
		repository.Workflow{
			ID:          "wf_2",
			WorkspaceID: "ws_1",
			Name:        "Disabled",
			Enabled:     false,
			TriggerType: "trigger.manual",
		},
		repository.Workflow{
			ID:          "wf_3",
			WorkspaceID: "ws_1",
			Name:        "Asset",
			Enabled:     true,
			TriggerType: "trigger.asset_created",
		},
		repository.Workflow{
			ID:          "wf_4",
			WorkspaceID: "ws_2",
			Name:        "Other workspace",
			Enabled:     true,
			TriggerType: "trigger.manual",
		},
	)
	triggerType := "trigger.manual"
	got, err := svc.List(context.Background(), "ws_1", service.ListWorkflowsParams{
		TriggerType: &triggerType,
		EnabledOnly: true,
	})
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "wf_1" {
		t.Fatalf("unexpected workflows: %#v", got)
	}
}

func newWorkflowSvcWithAssets(t *testing.T) (
	service.WorkflowService,
	*memory.WorkflowRepo,
	*memory.WorkflowRunRepo,
	*memory.AssetRepo,
	*memory.RealVersionRepo,
	*workflowQueueStub,
) {
	t.Helper()
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	assets := memory.NewAssetRepo()
	versions := memory.NewRealVersionRepo()
	q := &workflowQueueStub{}
	svc := service.NewWorkflowService(
		workflows, runs, memory.NewWorkflowWebhookRepo(), q,
		service.WorkflowServiceDeps{Assets: assets, Versions: versions},
	)
	return svc, workflows, runs, assets, versions, q
}

func TestWorkflowServiceTriggerManualBulkOK(t *testing.T) {
	svc, repo, runs, assets, versions, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(
		repository.Asset{
			ID:               "ast_1",
			WorkspaceID:      "ws_1",
			OriginalFilename: "foo.jpg",
			MimeType:         "image/jpeg",
			Size:             1024,
		},
		repository.Asset{
			ID:               "ast_2",
			WorkspaceID:      "ws_1",
			OriginalFilename: "bar.png",
			MimeType:         "image/png",
			Size:             2048,
		},
		repository.Asset{
			ID:               "ast_3",
			WorkspaceID:      "ws_1",
			OriginalFilename: "baz.mp4",
			MimeType:         "video/mp4",
			Size:             4096,
		},
	)
	versions.Seed(
		repository.AssetVersion{
			ID:          "ver_1",
			AssetID:     "ast_1",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "ws_1/ast_1/v1.jpg",
			IsCurrent:   true,
		},
		repository.AssetVersion{
			ID:          "ver_2",
			AssetID:     "ast_2",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "ws_1/ast_2/v1.png",
			IsCurrent:   true,
		},
		repository.AssetVersion{
			ID:          "ver_3",
			AssetID:     "ast_3",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "ws_1/ast_3/v1.mp4",
			IsCurrent:   true,
		},
	)
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_1", "ast_2", "ast_3"})
	if err != nil {
		t.Fatalf("TriggerManualBulk() unexpected error: %v", err)
	}
	if len(runIDs) != 3 || q.enqueued != 3 {
		t.Fatalf("expected 3 runs and jobs, runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
	for _, typ := range q.types {
		if typ != queue.JobTypeRunWorkflow {
			t.Fatalf("unexpected job type %q", typ)
		}
	}
	type expectation struct {
		assetID    string
		mimeType   string
		filename   string
		size       float64
		versionID  string
		versionNum float64
		storageKey string
	}
	expectations := []expectation{
		{"ast_1", "image/jpeg", "foo.jpg", 1024, "ver_1", 1, "ws_1/ast_1/v1.jpg"},
		{"ast_2", "image/png", "bar.png", 2048, "ver_2", 1, "ws_1/ast_2/v1.png"},
		{"ast_3", "video/mp4", "baz.mp4", 4096, "ver_3", 1, "ws_1/ast_3/v1.mp4"},
	}
	for i, runID := range runIDs {
		run, runErr := runs.GetByID(context.Background(), runID)
		if runErr != nil {
			t.Fatalf("run %q not persisted: %v", runID, runErr)
		}
		var td map[string]any
		if decodeErr := json.Unmarshal([]byte(run.TriggerData), &td); decodeErr != nil {
			t.Fatalf("run %d trigger_data invalid JSON: %v", i, decodeErr)
		}
		ex := expectations[i]
		// strings.Contains still used to keep "strings" import valid
		if !strings.Contains(run.TriggerData, ex.assetID) {
			t.Fatalf("run %d trigger_data missing asset_id %q: %s", i, ex.assetID, run.TriggerData)
		}
		checks := map[string]any{
			"asset_id":          ex.assetID,
			"workspace_id":      "ws_1",
			"mime_type":         ex.mimeType,
			"original_filename": ex.filename,
			"filename":          ex.filename,
			"size":              ex.size,
			"version_id":        ex.versionID,
			"version_num":       ex.versionNum,
			"storage_key":       ex.storageKey,
		}
		for key, want := range checks {
			got, ok := td[key]
			if !ok {
				t.Errorf("run %d: trigger_data missing key %q", i, key)
				continue
			}
			if got != want {
				t.Errorf("run %d: trigger_data[%q] = %v (%T), want %v (%T)", i, key, got, got, want, want)
			}
		}
	}
}

func TestWorkflowServiceTriggerManualBulkAssetNotFound(t *testing.T) {
	svc, repo, _, assets, _, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(repository.Asset{ID: "ast_alien", WorkspaceID: "ws_other", OriginalFilename: "x.jpg"})
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_alien"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if len(runIDs) != 0 || q.enqueued != 0 {
		t.Fatalf("expected no runs, got runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkNoCurrentVersion(t *testing.T) {
	svc, repo, _, assets, _, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(
		repository.Asset{
			ID:               "ast_noversion",
			WorkspaceID:      "ws_1",
			OriginalFilename: "ghost.jpg",
			MimeType:         "image/jpeg",
		},
	)
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_noversion"})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if len(runIDs) != 0 || q.enqueued != 0 {
		t.Fatalf("expected no runs, got runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkPartialSuccess(t *testing.T) {
	svc, repo, _, assets, versions, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(
		repository.Asset{ID: "ast_ok", WorkspaceID: "ws_1", OriginalFilename: "ok.jpg", MimeType: "image/jpeg"},
		repository.Asset{ID: "ast_bad", WorkspaceID: "ws_1", OriginalFilename: "bad.jpg", MimeType: "image/jpeg"},
	)
	versions.Seed(
		repository.AssetVersion{
			ID:          "ver_ok",
			AssetID:     "ast_ok",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "k/ok.jpg",
			IsCurrent:   true,
		},
	)
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_ok", "ast_bad"})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if len(runIDs) != 1 || q.enqueued != 1 {
		t.Fatalf("expected 1 partial run, got runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkDisabledWorkflow(t *testing.T) {
	svc, repo, _, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: false, TriggerType: "trigger.manual"})
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_1"})
	if !errors.Is(err, apperr.ErrConflict) || queue.enqueued != 0 {
		t.Fatalf("expected ErrConflict and empty queue, err=%v enqueued=%d", err, queue.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkWrongTriggerType(t *testing.T) {
	svc, repo, _, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.asset_created"})
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_1"})
	if !errors.Is(err, apperr.ErrConflict) || queue.enqueued != 0 {
		t.Fatalf("expected ErrConflict and empty queue, err=%v enqueued=%d", err, queue.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkEmptyAssetIDs(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", nil)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestWorkflowServiceTriggerManualBulkTooManyAssets(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	assetIDs := make([]string, 501)
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", assetIDs)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestWorkflowServiceTriggerManualBulkWorkspaceIsolation(t *testing.T) {
	svc, repo, _, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_a", Enabled: true, TriggerType: "trigger.manual"})
	_, err := svc.TriggerManualBulk(context.Background(), "ws_b", "wf_1", []string{"ast_1"})
	if !errors.Is(err, apperr.ErrNotFound) || queue.enqueued != 0 {
		t.Fatalf("expected ErrNotFound and empty queue, err=%v enqueued=%d", err, queue.enqueued)
	}
}

func TestWorkflowServiceTemplates(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	templates := svc.Templates()
	if len(templates) < 5 {
		t.Fatalf("expected embedded templates, got %d", len(templates))
	}
	if templates[0].Graph == "" {
		t.Fatal("expected template graph payload")
	}
	for _, tpl := range templates {
		if tpl.Icon == "" {
			t.Fatalf("expected non-empty Icon for template %q", tpl.ID)
		}
	}
}

func TestWorkflowServiceTemplatesIncludesBlankManual(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	templates := svc.Templates()
	var blank *service.WorkflowTemplateDTO
	for i := range templates {
		if templates[i].ID == "blank-manual" {
			blank = &templates[i]
		}
	}
	if blank == nil {
		t.Fatal("expected blank-manual template")
	}
	if blank.TriggerType != "trigger.manual" {
		t.Fatalf("expected trigger.manual trigger type, got %q", blank.TriggerType)
	}
}

func TestWorkflowServiceFindCoveringWorkflowScope(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:            "wf_1",
		WorkspaceID:   "ws_1",
		Name:          "Folder automation",
		Enabled:       true,
		TriggerType:   "trigger.version_uploaded",
		TriggerConfig: `{"folder_id":"fld_1"}`,
	})
	got, err := svc.FindCoveringWorkflow(context.Background(), "ws_1", "", "prj_1", "fld_1")
	if err != nil {
		t.Fatalf("FindCoveringWorkflow() unexpected error: %v", err)
	}
	if got == nil || got.ID != "wf_1" || got.Scope != "folder" {
		t.Fatalf("unexpected covering workflow: %#v", got)
	}
}

func TestWorkflowServiceCreateFromVariantsCreatesDisabledWorkflow(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	assets := memory.NewAssetRepo()
	variants := memory.NewRealVariantRepo()
	queue := &workflowQueueStub{}
	projectID := "prj_1"
	assets.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", ProjectID: &projectID, MimeType: "image/jpeg"})
	params := `{"width":1200}`
	variants.Seed(
		repository.Variant{ID: "var_1", WorkspaceID: "ws_1", Type: "manual"},
		repository.Variant{ID: "var_2", WorkspaceID: "ws_1", Type: "image_resize", TransformParams: &params},
	)
	svc := service.NewWorkflowService(
		workflows,
		runs,
		memory.NewWorkflowWebhookRepo(),
		queue,
		service.WorkflowServiceDeps{Assets: assets, Variants: variants},
	)
	got, err := svc.CreateFromVariants(context.Background(), "ws_1", service.CreateVariantAutomationParams{
		AssetID:   "ast_1",
		CreatedBy: "usr_1",
		Scope:     service.AutomationScopeProject,
	})
	if err != nil {
		t.Fatalf("CreateFromVariants() unexpected error: %v", err)
	}
	if got.Enabled || got.TriggerType != "trigger.version_uploaded" {
		t.Fatalf("unexpected workflow: enabled=%v trigger=%q", got.Enabled, got.TriggerType)
	}
	row, err := workflows.GetByID(context.Background(), "ws_1", got.ID)
	if err != nil {
		t.Fatalf("created workflow not found: %v", err)
	}
	if row.TriggerConfig != `{"project_id":"prj_1"}` {
		t.Fatalf("trigger_config = %s", row.TriggerConfig)
	}
	if got.Graph == "" || !strings.Contains(got.Graph, `"filter.mime"`) || strings.Contains(got.Graph, `"manual"`) {
		t.Fatalf("unexpected graph: %s", got.Graph)
	}
}

func TestWorkflowServiceCreateFromVariantsAssetScopeTriggerConfig(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	assets := memory.NewAssetRepo()
	variants := memory.NewRealVariantRepo()
	projectID := "prj_1"
	assets.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", ProjectID: &projectID, MimeType: "image/jpeg"})
	variants.Seed(repository.Variant{ID: "var_1", WorkspaceID: "ws_1", Type: "image_resize"})
	svc := service.NewWorkflowService(
		workflows,
		memory.NewWorkflowRunRepo(),
		memory.NewWorkflowWebhookRepo(),
		&workflowQueueStub{},
		service.WorkflowServiceDeps{Assets: assets, Variants: variants},
	)
	got, err := svc.CreateFromVariants(context.Background(), "ws_1", service.CreateVariantAutomationParams{
		AssetID:   "ast_1",
		CreatedBy: "usr_1",
		Scope:     service.AutomationScopeAsset,
	})
	if err != nil {
		t.Fatalf("CreateFromVariants() unexpected error: %v", err)
	}
	row, err := workflows.GetByID(context.Background(), "ws_1", got.ID)
	if err != nil {
		t.Fatalf("created workflow not found: %v", err)
	}
	if row.TriggerConfig != `{"asset_id":"ast_1"}` {
		t.Fatalf("trigger_config = %s, want {\"asset_id\":\"ast_1\"}", row.TriggerConfig)
	}
}

func TestWorkflowServiceFindCoveringWorkflowAssetScope(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:            "wf_asset",
		WorkspaceID:   "ws_1",
		Name:          "Asset automation",
		Enabled:       true,
		TriggerType:   "trigger.version_uploaded",
		TriggerConfig: `{"asset_id":"ast_1"}`,
	})
	got, err := svc.FindCoveringWorkflow(context.Background(), "ws_1", "ast_1", "prj_1", "fld_1")
	if err != nil {
		t.Fatalf("FindCoveringWorkflow() unexpected error: %v", err)
	}
	if got == nil || got.ID != "wf_asset" || got.Scope != "asset" {
		t.Fatalf("unexpected covering workflow: %#v", got)
	}
	// must not match a different asset
	none, err := svc.FindCoveringWorkflow(context.Background(), "ws_1", "ast_2", "prj_1", "fld_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("FindCoveringWorkflow() expected ErrNotFound, got %v", err)
	}
	if none != nil {
		t.Fatalf("expected no workflow for different asset, got %#v", none)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestWorkflowService_Update_OK(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Name: "Old", TriggerType: "trigger.manual"})
	newName := "New Name"
	newEmail := "ops@example.com"
	dto, err := svc.Update(context.Background(), "ws_1", "wf_1", service.UpdateWorkflowParams{
		Name:                 &newName,
		NotifyOnFailureEmail: &newEmail,
	})
	if err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if dto.Name != "New Name" {
		t.Fatalf("Name = %q, want %q", dto.Name, "New Name")
	}
	if dto.NotifyOnFailureEmail != "ops@example.com" {
		t.Fatalf("NotifyOnFailureEmail = %q, want %q", dto.NotifyOnFailureEmail, "ops@example.com")
	}
}

func TestWorkflowService_Update_InvalidParams(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	emptyName := ""
	_, err := svc.Update(context.Background(), "ws_1", "wf_1", service.UpdateWorkflowParams{Name: &emptyName})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestWorkflowService_Update_ExtractsTriggerType(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", TriggerType: "trigger.manual"})
	webhookGraph := `{"nodes":[{"id":"n1","type":"trigger.webhook","config":{},"position":{"x":0,"y":0}}],"edges":[]}`
	dto, err := svc.Update(context.Background(), "ws_1", "wf_1", service.UpdateWorkflowParams{Graph: &webhookGraph})
	if err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if dto.TriggerType != "trigger.webhook" {
		t.Fatalf("TriggerType = %q, want trigger.webhook", dto.TriggerType)
	}
}

// ---------------------------------------------------------------------------
// SetEnabled
// ---------------------------------------------------------------------------

func TestWorkflowService_SetEnabled_OK(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true})

	if err := svc.SetEnabled(context.Background(), "ws_1", "wf_1", false); err != nil {
		t.Fatalf("SetEnabled(false) unexpected error: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), "ws_1", "wf_1")
	if got.Enabled {
		t.Fatal("expected workflow to be disabled")
	}

	if err := svc.SetEnabled(context.Background(), "ws_1", "wf_1", true); err != nil {
		t.Fatalf("SetEnabled(true) unexpected error: %v", err)
	}
	got, _ = repo.GetByID(context.Background(), "ws_1", "wf_1")
	if !got.Enabled {
		t.Fatal("expected workflow to be enabled")
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestWorkflowService_Delete_OK(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1"})

	if err := svc.Delete(context.Background(), "ws_1", "wf_1"); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}
	_, err := repo.GetByID(context.Background(), "ws_1", "wf_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TriggerWebhook
// ---------------------------------------------------------------------------

const webhookGraph = `{"nodes":[{"id":"n1","type":"trigger.webhook","config":{},"position":{"x":0,"y":0}}],"edges":[]}`

func TestWorkflowService_TriggerWebhook_OK(t *testing.T) {
	svc, repo, _, webhooks, q := newWorkflowSvc(t)
	repo.Seed(
		repository.Workflow{
			ID:          "wf_1",
			WorkspaceID: "ws_1",
			Enabled:     true,
			TriggerType: "trigger.webhook",
			Graph:       webhookGraph,
		},
	)
	plaintext := "mysecrettoken"
	_ = webhooks.Upsert(context.Background(), "wf_1", workflow.Sha256Hex(plaintext))

	runID, err := svc.TriggerWebhook(context.Background(), "wf_1", plaintext, []byte(`{}`))
	if err != nil {
		t.Fatalf("TriggerWebhook() unexpected error: %v", err)
	}
	if runID == "" {
		t.Fatal("expected non-empty run ID")
	}
	if q.enqueued != 1 {
		t.Fatalf("expected 1 enqueued job, got %d", q.enqueued)
	}
}

func TestWorkflowService_TriggerWebhook_NotFound(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	_, err := svc.TriggerWebhook(context.Background(), "wf_unknown", "token", []byte(`{}`))
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkflowService_TriggerWebhook_BadToken(t *testing.T) {
	svc, repo, _, webhooks, _ := newWorkflowSvc(t)
	repo.Seed(
		repository.Workflow{
			ID:          "wf_1",
			WorkspaceID: "ws_1",
			Enabled:     true,
			TriggerType: "trigger.webhook",
			Graph:       webhookGraph,
		},
	)
	_ = webhooks.Upsert(context.Background(), "wf_1", workflow.Sha256Hex("correcttoken"))

	_, err := svc.TriggerWebhook(context.Background(), "wf_1", "wrongtoken", []byte(`{}`))
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestWorkflowService_TriggerWebhook_JSONBody(t *testing.T) {
	svc, repo, runs, webhooks, _ := newWorkflowSvc(t)
	repo.Seed(
		repository.Workflow{
			ID:          "wf_1",
			WorkspaceID: "ws_1",
			Enabled:     true,
			TriggerType: "trigger.webhook",
			Graph:       webhookGraph,
		},
	)
	plaintext := "tok"
	_ = webhooks.Upsert(context.Background(), "wf_1", workflow.Sha256Hex(plaintext))

	runID, err := svc.TriggerWebhook(context.Background(), "wf_1", plaintext, []byte(`{"foo":"bar"}`))
	if err != nil {
		t.Fatalf("TriggerWebhook() unexpected error: %v", err)
	}
	run, _ := runs.GetByID(context.Background(), runID)
	var td map[string]any
	_ = json.Unmarshal([]byte(run.TriggerData), &td)
	body, ok := td["body"]
	if !ok {
		t.Fatal("expected trigger_data to contain 'body' key for JSON body")
	}
	bodyMap, ok := body.(map[string]any)
	if !ok || bodyMap["foo"] != "bar" {
		t.Fatalf("unexpected body value: %#v", body)
	}
}

func TestWorkflowService_TriggerWebhook_NonJSONBody(t *testing.T) {
	svc, repo, runs, webhooks, _ := newWorkflowSvc(t)
	repo.Seed(
		repository.Workflow{
			ID:          "wf_1",
			WorkspaceID: "ws_1",
			Enabled:     true,
			TriggerType: "trigger.webhook",
			Graph:       webhookGraph,
		},
	)
	plaintext := "tok"
	_ = webhooks.Upsert(context.Background(), "wf_1", workflow.Sha256Hex(plaintext))

	runID, err := svc.TriggerWebhook(context.Background(), "wf_1", plaintext, []byte(`not-json`))
	if err != nil {
		t.Fatalf("TriggerWebhook() unexpected error: %v", err)
	}
	run, _ := runs.GetByID(context.Background(), runID)
	var td map[string]any
	_ = json.Unmarshal([]byte(run.TriggerData), &td)
	if td["raw_body"] != "not-json" {
		t.Fatalf("expected raw_body=not-json, got %v", td["raw_body"])
	}
	if _, ok := td["body"]; ok {
		t.Fatal("expected no 'body' key for non-JSON body")
	}
}

// ---------------------------------------------------------------------------
// GetRun
// ---------------------------------------------------------------------------

func TestWorkflowService_GetRun_OK(t *testing.T) {
	svc, _, runs, _, _ := newWorkflowSvc(t)
	ctx := context.Background()
	now := time.Now().UTC()
	run, _ := runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          "run_1",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		Status:      service.WorkflowRunStatusPending,
		TriggerData: `{"trigger":"manual"}`,
	})
	_ = run
	_, _ = runs.CreateStep(ctx, repository.CreateWorkflowRunStepParams{
		ID:        "step_1",
		RunID:     "run_1",
		NodeID:    "n1",
		NodeType:  "trigger.manual",
		Status:    service.WorkflowRunStatusPending,
		InputCtx:  `{"k":"v"}`,
		StartedAt: &now,
	})

	dto, err := svc.GetRun(ctx, "ws_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun() unexpected error: %v", err)
	}
	if dto.ID != "run_1" || dto.WorkflowID != "wf_1" {
		t.Fatalf("unexpected run DTO: %+v", dto)
	}
	if len(dto.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(dto.Steps))
	}
	if dto.Steps[0].NodeID != "n1" {
		t.Fatalf("unexpected step node_id: %q", dto.Steps[0].NodeID)
	}
	if dto.TriggerData["trigger"] != "manual" {
		t.Fatalf("unexpected trigger_data: %v", dto.TriggerData)
	}
}

func TestWorkflowService_GetRun_WrongWorkspace(t *testing.T) {
	svc, _, runs, _, _ := newWorkflowSvc(t)
	ctx := context.Background()
	_, _ = runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          "run_1",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_a",
		Status:      service.WorkflowRunStatusPending,
		TriggerData: `{}`,
	})

	_, err := svc.GetRun(ctx, "ws_b", "run_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListRuns
// ---------------------------------------------------------------------------

func TestWorkflowService_ListRuns_OK(t *testing.T) {
	svc, _, runs, _, _ := newWorkflowSvc(t)
	ctx := context.Background()
	for _, id := range []string{"run_1", "run_2", "run_3"} {
		_, _ = runs.Create(ctx, repository.CreateWorkflowRunParams{
			ID:          id,
			WorkflowID:  "wf_1",
			WorkspaceID: "ws_1",
			Status:      service.WorkflowRunStatusPending,
			TriggerData: `{}`,
		})
	}

	dtos, err := svc.ListRuns(ctx, "ws_1", "wf_1", 2, "")
	if err != nil {
		t.Fatalf("ListRuns() unexpected error: %v", err)
	}
	if len(dtos) != 2 {
		t.Fatalf("expected 2 runs (limit), got %d", len(dtos))
	}
	for _, dto := range dtos {
		if len(dto.Steps) != 0 {
			t.Fatal("expected no steps in ListRuns result")
		}
	}
}

// ---------------------------------------------------------------------------
// ListAllRuns
// ---------------------------------------------------------------------------

func TestWorkflowService_ListAllRuns_OK(t *testing.T) {
	svc, _, runs, _, _ := newWorkflowSvc(t)
	ctx := context.Background()
	_, _ = runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          "run_1",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		Status:      service.WorkflowRunStatusPending,
		TriggerData: `{}`,
	})
	_, _ = runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          "run_2",
		WorkflowID:  "wf_2",
		WorkspaceID: "ws_1",
		Status:      service.WorkflowRunStatusPending,
		TriggerData: `{}`,
	})
	_, _ = runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          "run_3",
		WorkflowID:  "wf_other",
		WorkspaceID: "ws_2",
		Status:      service.WorkflowRunStatusPending,
		TriggerData: `{}`,
	})

	got, err := svc.ListAllRuns(ctx, "ws_1", 10, "")
	if err != nil {
		t.Fatalf("ListAllRuns() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 runs for ws_1, got %d: %v", len(got), got)
	}
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids["run_1"] || !ids["run_2"] {
		t.Fatalf("expected run_1 and run_2, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetWebhookToken
// ---------------------------------------------------------------------------

func TestWorkflowService_GetWebhookToken_CreatesWhenMissing(t *testing.T) {
	svc, repo, _, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1"})

	token, err := svc.GetWebhookToken(context.Background(), "ws_1", "wf_1")
	if err != nil {
		t.Fatalf("GetWebhookToken() unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token when none existed")
	}
}

func TestWorkflowService_GetWebhookToken_ReturnsEmptyWhenExists(t *testing.T) {
	svc, repo, _, webhooks, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1"})
	_ = webhooks.Upsert(context.Background(), "wf_1", workflow.Sha256Hex("existing"))

	token, err := svc.GetWebhookToken(context.Background(), "ws_1", "wf_1")
	if err != nil {
		t.Fatalf("GetWebhookToken() unexpected error: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty token when hash already exists, got %q", token)
	}
}

func TestWorkflowService_GetWebhookToken_NotFound(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	_, err := svc.GetWebhookToken(context.Background(), "ws_1", "wf_unknown")
	if err == nil {
		t.Fatal("expected error for unknown workflow")
	}
}

// ---------------------------------------------------------------------------
// RegenerateWebhookToken
// ---------------------------------------------------------------------------

func TestWorkflowService_RegenerateWebhookToken_OK(t *testing.T) {
	svc, repo, _, webhooks, _ := newWorkflowSvc(t)
	repo.Seed(
		repository.Workflow{
			ID:          "wf_1",
			WorkspaceID: "ws_1",
			Enabled:     true,
			TriggerType: "trigger.webhook",
			Graph:       webhookGraph,
		},
	)
	ctx := context.Background()

	tok1, err := svc.RegenerateWebhookToken(ctx, "ws_1", "wf_1")
	if err != nil {
		t.Fatalf("RegenerateWebhookToken() first call unexpected error: %v", err)
	}
	if tok1 == "" {
		t.Fatal("expected non-empty token")
	}

	tok2, err := svc.RegenerateWebhookToken(ctx, "ws_1", "wf_1")
	if err != nil {
		t.Fatalf("RegenerateWebhookToken() second call unexpected error: %v", err)
	}
	if tok2 == "" || tok2 == tok1 {
		t.Fatalf("expected different non-empty token on regeneration, tok1=%q tok2=%q", tok1, tok2)
	}

	// tok1 must no longer work
	_, err = svc.TriggerWebhook(ctx, "wf_1", tok1, []byte(`{}`))
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden with old token after regeneration, got %v", err)
	}

	// tok2 must still work
	_ = webhooks // already verified via TriggerWebhook
	_, err = svc.TriggerWebhook(ctx, "wf_1", tok2, []byte(`{}`))
	if err != nil {
		t.Fatalf("TriggerWebhook with new token unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NodeSchemas
// ---------------------------------------------------------------------------

func TestWorkflowService_NodeSchemas_OK(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	schemas := svc.NodeSchemas()
	if len(schemas) == 0 {
		t.Fatal("expected non-empty node schemas")
	}
	for _, s := range schemas {
		if s.Type == "" || s.Label == "" {
			t.Errorf("schema missing Type or Label: %+v", s)
		}
	}
}

// ---------------------------------------------------------------------------
// parseMap / parseMapPtr / toWorkflowNodePorts via public API
// ---------------------------------------------------------------------------

func TestWorkflowService_GetRun_ParseMapFallbacks(t *testing.T) {
	svc, _, runs, _, _ := newWorkflowSvc(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// invalid JSON trigger_data → should produce empty map, not panic
	_, _ = runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          "run_bad",
		WorkflowID:  "wf_1",
		WorkspaceID: "ws_1",
		Status:      service.WorkflowRunStatusPending,
		TriggerData: `not-json`,
	})
	_ = now
	dto, err := svc.GetRun(ctx, "ws_1", "run_bad")
	if err != nil {
		t.Fatalf("GetRun() unexpected error: %v", err)
	}
	if dto.TriggerData == nil || len(dto.TriggerData) != 0 {
		t.Fatalf("expected empty map for invalid trigger_data, got %v", dto.TriggerData)
	}

	// step with nil OutputCtx → parseMapPtr(nil) should produce empty map
	_, _ = runs.CreateStep(ctx, repository.CreateWorkflowRunStepParams{
		ID:        "step_nil",
		RunID:     "run_bad",
		NodeID:    "n1",
		NodeType:  "trigger.manual",
		Status:    service.WorkflowRunStatusPending,
		InputCtx:  `{}`,
		OutputCtx: nil,
		StartedAt: &now,
	})
	dto2, err := svc.GetRun(ctx, "ws_1", "run_bad")
	if err != nil {
		t.Fatalf("GetRun() unexpected error: %v", err)
	}
	if len(dto2.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(dto2.Steps))
	}
	if dto2.Steps[0].OutputCtx == nil {
		t.Fatal("expected non-nil OutputCtx map even for nil pointer")
	}
	if len(dto2.Steps[0].OutputCtx) != 0 {
		t.Fatalf("expected empty OutputCtx map, got %v", dto2.Steps[0].OutputCtx)
	}
}

func TestWorkflowService_NodeSchemas_PortsAreMapped(t *testing.T) {
	svc, _, _, _, _ := newWorkflowSvc(t)
	schemas := svc.NodeSchemas()
	// find any schema that has inputs or outputs and verify mapping
	for _, s := range schemas {
		for _, p := range s.Inputs {
			if p.ID == "" {
				t.Errorf("schema %q: input port missing ID", s.Type)
			}
		}
		for _, p := range s.Outputs {
			if p.ID == "" {
				t.Errorf("schema %q: output port missing ID", s.Type)
			}
		}
	}
}
