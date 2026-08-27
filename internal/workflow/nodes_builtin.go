package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"damask/server/internal/ai"
	"damask/server/internal/apperr"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

const (
	nodeCategoryTrigger   = "trigger"
	nodeCategoryFilter    = "filter"
	nodeCategoryAction    = "action"
	portError             = "error"
	portOut               = "out"
	portMatch             = "match"
	portNoMatch           = "no_match"
	labelError            = "Error"
	labelOut              = "Out"
	labelContinued        = "Continued"
	nodeTypeCreateVariant = "action.create_variant"
	nodeTypeSetNewVersion = "action.set_new_version"
	nodeTypeAIImageDesc   = "action.ai_image_description"
	nodeTypeOCR           = "action.ocr"
	nodeTypeAutoTag       = "action.auto_tag"
	autoTagModePending    = "pending"
	autoTagModeSilent     = "silent"
	rcKeyContinuation     = "__workflow_continuation"
	portContinued         = "continued"
	outputKeyTrackID      = "track_id"
	ocrOutputFormatTxt    = "txt"
	ocrOutputFormatHocr   = "hocr"
)

// ContinuationInjector is an optional interface nodes can implement to seed
// the RunContext with a NodeContinuation before Execute is called. This lets
// async nodes (e.g. create_variant) embed the resume hint in their job payload
// without the executor needing to know which node types are involved.
type ContinuationInjector interface {
	InjectContinuation(rc *RunContext, g *Graph, nodeID, runID, workflowID, workspaceID string)
}

func mustConfigSchema(raw string) json.RawMessage {
	return json.RawMessage(raw)
}

func triggerSchema(nodeType, label, desc string, configSchema json.RawMessage) NodeSchema {
	return NodeSchema{
		Type:         nodeType,
		Label:        label,
		Category:     nodeCategoryTrigger,
		Description:  desc,
		Outputs:      []Port{{ID: portOut, Label: labelOut}, {ID: portError, Label: labelError}},
		ConfigSchema: configSchema,
	}
}

func filterSchema(nodeType, label, desc string, configSchema json.RawMessage) NodeSchema {
	return NodeSchema{
		Type:        nodeType,
		Label:       label,
		Category:    nodeCategoryFilter,
		Description: desc,
		Inputs:      []Port{{ID: "in", Label: "In"}},
		Outputs: []Port{
			{ID: portMatch, Label: "Match"},
			{ID: portNoMatch, Label: "No match"},
			{ID: portError, Label: labelError},
		},
		ConfigSchema: configSchema,
	}
}

func actionSchema(nodeType, label, desc string, configSchema json.RawMessage) NodeSchema {
	return NodeSchema{
		Type:         nodeType,
		Label:        label,
		Category:     nodeCategoryAction,
		Description:  desc,
		Inputs:       []Port{{ID: "in", Label: "In"}},
		Outputs:      []Port{{ID: portOut, Label: labelOut}, {ID: portError, Label: labelError}},
		ConfigSchema: configSchema,
	}
}

func setNewVersionSchemaFn() NodeSchema {
	return actionSchema(
		nodeTypeSetNewVersion,
		"Set New Version",
		"Promotes the current variant as a new asset version.",
		mustConfigSchema(
			`{"type":"object","properties":{"comment":{"type":"string","title":"Version Comment"}},"additionalProperties":false}`,
		),
	)
}

func createVariantSchemaFn() NodeSchema {
	return NodeSchema{
		Type:        nodeTypeCreateVariant,
		Label:       "Create Variant",
		Category:    "action",
		Description: "Queues a new variant job.",
		Inputs:      []Port{{ID: "in", Label: "In"}},
		Outputs: []Port{
			{ID: portOut, Label: labelOut},
			{ID: portContinued, Label: labelContinued},
			{ID: portError, Label: labelError},
		},
		ConfigSchema: mustConfigSchema(
			`{"type":"object","properties":{"type":{"type":"string","title":"Variant Type","format":"variant"},"params":{"type":"object","title":"Params","format":"json"},"title":{"type":"string","title":"Title"},"is_shared":{"type":"boolean","title":"Shared"}},"required":["type"],"additionalProperties":false}`,
		),
	}
}

func aiImageDescriptionSchemaFn() NodeSchema {
	return NodeSchema{
		Type:        nodeTypeAIImageDesc,
		Label:       "AI Image Description",
		Category:    nodeCategoryAction,
		Description: "Describes an image using an AI vision model and stores it as a text track.",
		Inputs:      []Port{{ID: "in", Label: "In"}},
		Outputs: []Port{
			{ID: portOut, Label: labelOut},
			{ID: portContinued, Label: labelContinued},
			{ID: portError, Label: labelError},
		},
		ConfigSchema: mustConfigSchema(
			`{"type":"object","properties":{"model":{"type":"string","title":"Model","format":"vision_model"},"lang":{"type":"string","title":"Language","format":"language"},"prompt":{"type":"string","title":"Prompt","format":"ai_description_prompt"}},"additionalProperties":false}`,
		),
	}
}

func ocrSchemaFn() NodeSchema {
	return NodeSchema{
		Type:        nodeTypeOCR,
		Label:       "OCR",
		Category:    nodeCategoryAction,
		Description: "Extracts text from an image using Tesseract OCR and stores it as a text track.",
		Inputs:      []Port{{ID: "in", Label: "In"}},
		Outputs: []Port{
			{ID: portOut, Label: labelOut},
			{ID: portContinued, Label: labelContinued},
			{ID: portError, Label: labelError},
		},
		ConfigSchema: mustConfigSchema(
			`{"type":"object","properties":{"lang":{"type":"string","title":"Language","format":"ocr_language","default":"eng"},"output_format":{"type":"string","title":"Output Format","enum":["txt","hocr"],"default":"txt"}},"additionalProperties":false}`,
		),
	}
}

func autoTagSchemaFn() NodeSchema {
	return NodeSchema{
		Type:        nodeTypeAutoTag,
		Label:       "AI Auto-Tag",
		Category:    nodeCategoryAction,
		Description: "Suggests or applies AI-generated tags for an asset.",
		Inputs:      []Port{{ID: "in", Label: "In"}},
		Outputs: []Port{
			{ID: portOut, Label: labelOut},
			{ID: portContinued, Label: labelContinued},
			{ID: portError, Label: labelError},
		},
		ConfigSchema: mustConfigSchema(
			`{"type":"object","properties":{"mode":{"type":"string","title":"Mode","enum":["pending","silent"],"default":"pending","description":"Pending creates suggestions to review; silent applies tags immediately."}},"additionalProperties":false}`,
		),
	}
}

// initializes built-in nodes (triggers, filters, and actions that don't require external dependencies)
//
//nolint:funlen // this is just a long list of node registrations
func init() {
	registerPassThroughTrigger := func(nodeType, label, desc string, configSchema json.RawMessage) {
		schema := triggerSchema(nodeType, label, desc, configSchema)
		Register(schema, func(Deps) Node { return passThroughNode{schema: schema} })
	}
	registerPassThroughTrigger(
		"trigger.manual",
		"Manual Trigger",
		"Starts a workflow manually.",
		mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`),
	)
	registerPassThroughTrigger(
		"trigger.asset_created",
		"Asset Created",
		"Starts when an asset is uploaded.",
		mustConfigSchema(
			`{"type":"object","properties":{"project_id":{"type":"string","title":"Project ID"},"folder_id":{"type":"string","title":"Folder ID","format":"folder"}},"additionalProperties":false}`,
		),
	)
	registerPassThroughTrigger(
		"trigger.version_uploaded",
		"Version Uploaded",
		"Starts when a new asset version is uploaded.",
		mustConfigSchema(
			`{"type":"object","properties":{"asset_id":{"type":"string","title":"Asset ID"}},"additionalProperties":false}`,
		),
	)
	registerPassThroughTrigger(
		"trigger.tag_added",
		"Tag Added",
		"Starts when a tag is added to an asset.",
		mustConfigSchema(
			`{"type":"object","properties":{"tag":{"type":"string","title":"Tag Name","format":"tag"}},"required":["tag"],"additionalProperties":false}`,
		),
	)
	registerPassThroughTrigger(
		"trigger.schedule",
		"Schedule Trigger",
		"Starts on a scheduler tick.",
		mustConfigSchema(
			`{"type":"object","properties":{"cron":{"type":"string","title":"Cron","format":"cron"}},"required":["cron"],"additionalProperties":false}`,
		),
	)
	registerPassThroughTrigger(
		"trigger.webhook",
		"Webhook Trigger",
		"Starts from an inbound webhook.",
		mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`),
	)

	Register(
		filterSchema(
			"filter.mime",
			"Filter MIME Type",
			"Routes based on MIME type.",
			mustConfigSchema(
				`{"type":"object","properties":{"prefix":{"type":"string","title":"MIME Prefix","placeholder":"image/"}},"required":["prefix"],"additionalProperties":false}`,
			),
		),
		func(Deps) Node {
			return filterNode{
				schema: filterSchema(
					"filter.mime",
					"Filter MIME Type",
					"Routes based on MIME type.",
					mustConfigSchema(
						`{"type":"object","properties":{"prefix":{"type":"string","title":"MIME Prefix","placeholder":"image/"}},"required":["prefix"],"additionalProperties":false}`,
					),
				),
				matchFn: matchMime,
			}
		},
	)
	Register(
		filterSchema(
			"filter.filename",
			"Filter Filename",
			"Routes based on filename.",
			mustConfigSchema(
				`{"type":"object","properties":{"contains":{"type":"string","title":"Contains"},"extension":{"type":"string","title":"Extension","placeholder":".pdf"}},"additionalProperties":false}`,
			),
		),
		func(Deps) Node {
			return filterNode{
				schema: filterSchema(
					"filter.filename",
					"Filter Filename",
					"Routes based on filename.",
					mustConfigSchema(
						`{"type":"object","properties":{"contains":{"type":"string","title":"Contains"},"extension":{"type":"string","title":"Extension","placeholder":".pdf"}},"additionalProperties":false}`,
					),
				),
				matchFn: matchFilename,
			}
		},
	)
	Register(
		filterSchema(
			"filter.size",
			"Filter Size",
			"Routes based on file size.",
			mustConfigSchema(
				`{"type":"object","properties":{"min":{"type":"number","title":"Min Bytes"},"max":{"type":"number","title":"Max Bytes"}},"additionalProperties":false}`,
			),
		),
		func(Deps) Node {
			return filterNode{
				schema: filterSchema(
					"filter.size",
					"Filter Size",
					"Routes based on file size.",
					mustConfigSchema(
						`{"type":"object","properties":{"min":{"type":"number","title":"Min Bytes"},"max":{"type":"number","title":"Max Bytes"}},"additionalProperties":false}`,
					),
				),
				matchFn: matchSize,
			}
		},
	)
	Register(
		filterSchema(
			"filter.tag",
			"Filter Tag",
			"Routes based on tag name.",
			mustConfigSchema(
				`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`,
			),
		),
		func(Deps) Node {
			return filterNode{
				schema: filterSchema(
					"filter.tag",
					"Filter Tag",
					"Routes based on tag name.",
					mustConfigSchema(
						`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`,
					),
				),
				matchFn: matchTag,
			}
		},
	)
	Register(
		filterSchema(
			"filter.folder",
			"Filter Folder",
			"Routes based on folder id.",
			mustConfigSchema(
				`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"}},"required":["folder_id"],"additionalProperties":false}`,
			),
		),
		func(Deps) Node {
			return filterNode{
				schema: filterSchema(
					"filter.folder",
					"Filter Folder",
					"Routes based on folder id.",
					mustConfigSchema(
						`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"}},"required":["folder_id"],"additionalProperties":false}`,
					),
				),
				matchFn: matchFolder,
			}
		},
	)
	Register(
		filterSchema(
			"filter.expression",
			"Filter Expression",
			"Routes based on a key/value comparison.",
			mustConfigSchema(
				`{"type":"object","properties":{"key":{"type":"string","title":"Context Key"},"value":{"type":"string","title":"Expected Value"}},"required":["key","value"],"additionalProperties":false}`,
			),
		),
		func(Deps) Node {
			return filterNode{
				schema: filterSchema(
					"filter.expression",
					"Filter Expression",
					"Routes based on a key/value comparison.",
					mustConfigSchema(
						`{"type":"object","properties":{"key":{"type":"string","title":"Context Key"},"value":{"type":"string","title":"Expected Value"}},"required":["key","value"],"additionalProperties":false}`,
					),
				),
				matchFn: matchExpression,
			}
		},
	)

	Register(
		createVariantSchemaFn(),
		func(deps Deps) Node {
			return createVariantNode{deps: deps, schema: createVariantSchemaFn()}
		},
	)
	Register(
		actionSchema(
			"action.share",
			"Create Share",
			"Creates a share for the asset.",
			mustConfigSchema(
				`{"type":"object","properties":{"label":{"type":"string","title":"Label"},"allow_comments":{"type":"boolean","title":"Allow Comments"},"allow_download":{"type":"boolean","title":"Allow Download"},"expires_in_days":{"type":"number","title":"Expires In Days"}},"additionalProperties":false}`,
			),
		),
		func(deps Deps) Node {
			return createShareNode{
				deps: deps,
				schema: actionSchema(
					"action.share",
					"Create Share",
					"Creates a share for the asset.",
					mustConfigSchema(
						`{"type":"object","properties":{"label":{"type":"string","title":"Label"},"allow_comments":{"type":"boolean","title":"Allow Comments"},"allow_download":{"type":"boolean","title":"Allow Download"},"expires_in_days":{"type":"number","title":"Expires In Days"}},"additionalProperties":false}`,
					),
				),
			}
		},
	)
	Register(
		actionSchema(
			"action.tag",
			"Tag Asset",
			"Adds a tag to the asset.",
			mustConfigSchema(
				`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`,
			),
		),
		func(deps Deps) Node {
			return tagAssetNode{
				deps: deps,
				schema: actionSchema(
					"action.tag",
					"Tag Asset",
					"Adds a tag to the asset.",
					mustConfigSchema(
						`{"type":"object","properties":{"name":{"type":"string","title":"Tag Name","format":"tag"}},"required":["name"],"additionalProperties":false}`,
					),
				),
			}
		},
	)
	Register(
		actionSchema(
			"action.move_folder",
			"Move Asset",
			"Moves the asset to a folder or project.",
			mustConfigSchema(
				`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"},"project_id":{"type":"string","title":"Project ID"}},"additionalProperties":false}`,
			),
		),
		func(deps Deps) Node {
			return moveAssetNode{
				deps: deps,
				schema: actionSchema(
					"action.move_folder",
					"Move Asset",
					"Moves the asset to a folder or project.",
					mustConfigSchema(
						`{"type":"object","properties":{"folder_id":{"type":"string","title":"Folder ID","format":"folder"},"project_id":{"type":"string","title":"Project ID"}},"additionalProperties":false}`,
					),
				),
			}
		},
	)
	Register(
		actionSchema(
			"action.set_field",
			"Set Asset Field",
			"Sets a custom field value on the asset.",
			mustConfigSchema(
				`{"type":"object","properties":{"field_id":{"type":"string","title":"Field ID"},"value":{"title":"Value","format":"json"}},"required":["field_id"],"additionalProperties":false}`,
			),
		),
		func(deps Deps) Node {
			return setFieldNode{
				deps: deps,
				schema: actionSchema(
					"action.set_field",
					"Set Asset Field",
					"Sets a custom field value on the asset.",
					mustConfigSchema(
						`{"type":"object","properties":{"field_id":{"type":"string","title":"Field ID"},"value":{"title":"Value","format":"json"}},"required":["field_id"],"additionalProperties":false}`,
					),
				),
			}
		},
	)
	Register(
		actionSchema(
			"control.fan_out",
			"Fan Out",
			"Forwards execution to every connected branch.",
			mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`),
		),
		func(Deps) Node {
			return passThroughNode{
				schema: actionSchema(
					"control.fan_out",
					"Fan Out",
					"Forwards execution to every connected branch.",
					mustConfigSchema(`{"type":"object","properties":{},"additionalProperties":false}`),
				),
			}
		},
	)
	Register(
		setNewVersionSchemaFn(),
		func(deps Deps) Node {
			return setNewVersionNode{deps: deps, schema: setNewVersionSchemaFn()}
		},
	)
	Register(
		aiImageDescriptionSchemaFn(),
		func(deps Deps) Node {
			return aiImageDescriptionNode{deps: deps, schema: aiImageDescriptionSchemaFn()}
		},
	)
	Register(
		ocrSchemaFn(),
		func(deps Deps) Node {
			return ocrNode{deps: deps, schema: ocrSchemaFn()}
		},
	)
	Register(
		autoTagSchemaFn(),
		func(deps Deps) Node {
			return autoTagNode{deps: deps, schema: autoTagSchemaFn()}
		},
	)
}

type setNewVersionNode struct {
	deps   Deps
	schema NodeSchema
}

func (n setNewVersionNode) Schema() NodeSchema { return n.schema }

func (n setNewVersionNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	variantID, err := rcRequireString(rc, "variant_id")
	if err != nil {
		return "", nil, err
	}
	if n.deps.Versions == nil {
		return "", nil, errors.New("workflow set_new_version dependencies not configured")
	}

	variant, err := n.deps.Variants.GetVariantByID(ctx, workspaceID, variantID)
	if err != nil {
		return "", nil, fmt.Errorf("get variant: %w", err)
	}
	if variant.Status != "ready" {
		return "", nil, fmt.Errorf(
			"variant %s is not ready (status: %s): %w",
			variantID,
			variant.Status,
			apperr.ErrVariantNotReady,
		)
	}

	var nodeCfg struct {
		Comment *string `json:"comment,omitempty"`
	}
	_ = json.Unmarshal(cfg, &nodeCfg)

	nextNum, err := n.deps.Versions.NextVersionNum(ctx, assetID)
	if err != nil {
		return "", nil, fmt.Errorf("next version num: %w", err)
	}

	createdBy := actorUserID(ctx, rc)
	size := int64(0)
	if variant.Size != nil {
		size = *variant.Size
	}
	// Use the content type injected by the job worker; fall back to the asset's
	// original mime type when running outside the continuation path.
	mimeType, _ := rcGetString(rc, "variant_content_type")
	if mimeType == "" {
		mimeType, _ = rcGetString(rc, "mime_type")
	}
	newVersionID := uuid.NewString()
	created, err := n.deps.Versions.Create(ctx, repository.AssetVersion{
		ID:          newVersionID,
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		VersionNum:  nextNum,
		StorageKey:  variant.StorageKey,
		ContentHash: variant.ContentHash,
		MimeType:    mimeType,
		Size:        size,
		Comment:     nodeCfg.Comment,
		CreatedBy:   &createdBy,
	})
	if err != nil {
		return "", nil, fmt.Errorf("create version: %w", err)
	}

	if setErr := n.deps.Versions.SetCurrent(ctx, assetID, created.ID); setErr != nil {
		return "", nil, fmt.Errorf("set current version: %w", setErr)
	}

	// Clear the old thumbnail so the asset doesn't show a stale image, then
	// enqueue a fresh thumbnail job for the new version.
	if thumbErr := n.deps.Versions.SetAssetThumbnail(ctx, assetID, nil); thumbErr != nil {
		slog.ErrorContext(ctx, "set_new_version: clear asset thumbnail failed", "asset_id", assetID, "err", thumbErr)
	}
	if n.deps.Queue == nil {
		return "", nil, errors.New("set_new_version: queue dependency is nil")
	}
	payload, err := json.Marshal(map[string]string{
		"asset_id":     assetID,
		"version_id":   created.ID,
		"workspace_id": workspaceID, //nolint:goconst // queue job payloads are opaque to the executor
		"storage_key":  created.StorageKey,
		"mime_type":    created.MimeType,
	})
	if err != nil {
		return "", nil, fmt.Errorf("set_new_version: marshal thumbnail payload: %w", err)
	}
	_, _ = n.deps.Queue.Enqueue(ctx, workspaceID, queue.JobTypeVersionThumbnail, string(payload))

	return portOut, map[string]any{
		"version_id":  created.ID,
		"version_num": created.VersionNum,
	}, nil
}

type passThroughNode struct{ schema NodeSchema }

func (n passThroughNode) Schema() NodeSchema { return n.schema }
func (n passThroughNode) Execute(_ context.Context, _ *RunContext, _ json.RawMessage) (string, map[string]any, error) {
	return portOut, nil, nil
}

type filterNode struct {
	schema  NodeSchema
	matchFn func(*RunContext, json.RawMessage) (bool, error)
}

func (n filterNode) Schema() NodeSchema { return n.schema }
func (n filterNode) Execute(_ context.Context, rc *RunContext, cfg json.RawMessage) (string, map[string]any, error) {
	match, err := n.matchFn(rc, cfg)
	if err != nil {
		return "", nil, err
	}
	if match {
		return portMatch, nil, nil
	}
	return portNoMatch, nil, nil
}

func matchMime(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Prefix string `json:"prefix"`
	}
	_ = json.Unmarshal(cfg, &c)
	mimeType, _ := rcGetString(rc, "mime_type")
	return strings.HasPrefix(mimeType, c.Prefix), nil
}

func matchFilename(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Contains  string `json:"contains"`
		Extension string `json:"extension"`
	}
	_ = json.Unmarshal(cfg, &c)
	name, _ := rcGetString(rc, "filename")
	if name == "" {
		name, _ = rcGetString(rc, "original_filename")
	}
	if c.Contains != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(c.Contains)) {
		return false, nil
	}
	if c.Extension != "" && !strings.EqualFold(filepath.Ext(name), c.Extension) {
		return false, nil
	}
	return true, nil
}

func matchSize(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Min *float64 `json:"min"`
		Max *float64 `json:"max"`
	}
	_ = json.Unmarshal(cfg, &c)
	val, ok := rc.Get("size")
	if !ok {
		return false, nil
	}
	size, ok := val.(float64)
	if !ok {
		return false, nil
	}
	if c.Min != nil && size < *c.Min {
		return false, nil
	}
	if c.Max != nil && size > *c.Max {
		return false, nil
	}
	return true, nil
}

func matchTag(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(cfg, &c)
	tagName, _ := rcGetString(rc, "tag_name")
	return strings.EqualFold(tagName, c.Name), nil
}

func matchFolder(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		FolderID string `json:"folder_id"`
	}
	_ = json.Unmarshal(cfg, &c)
	folderID, _ := rcGetString(rc, "folder_id")
	return folderID == c.FolderID, nil
}

func matchExpression(rc *RunContext, cfg json.RawMessage) (bool, error) {
	var c struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	_ = json.Unmarshal(cfg, &c)
	val, _ := rcGetString(rc, c.Key)
	return val == c.Value, nil
}

type createVariantNode struct {
	deps   Deps
	schema NodeSchema
}

func (n createVariantNode) Schema() NodeSchema { return n.schema }

func (n createVariantNode) InjectContinuation(rc *RunContext, g *Graph, nodeID, runID, workflowID, workspaceID string) {
	rc.Delete(rcKeyContinuation)
	for _, s := range g.Successors(nodeID, portOut) {
		if s.Type == nodeTypeSetNewVersion {
			rc.Set(rcKeyContinuation, NodeContinuation{
				RunID:       runID,
				NodeID:      s.ID,
				WorkflowID:  workflowID,
				WorkspaceID: workspaceID,
			})
			return
		}
	}
}

// resolveAssetForVariant validates node dependencies and resolves the asset
// (with its current version) that a variant will be created from.
func (n createVariantNode) resolveAssetForVariant(ctx context.Context, rc *RunContext) (string, *Asset, error) {
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	if n.deps.Assets == nil || n.deps.Variants == nil || n.deps.Workspace == nil || n.deps.Queue == nil ||
		n.deps.Config == nil {
		return "", nil, errors.New("workflow create_variant dependencies not configured")
	}
	asset, err := n.deps.Assets.Get(ctx, workspaceID, assetID)
	if err != nil {
		return "", nil, err
	}
	if asset.CurrentVersionID == nil {
		return "", nil, fmt.Errorf("asset has no current version: %w", apperr.ErrInvalidInput)
	}
	return workspaceID, asset, nil
}

// prepareVariant reads the node's config, checks ImageRouter availability,
// and delegates to the variant service to normalize/validate the requested
// variant params.
func (n createVariantNode) prepareVariant(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
	workspaceID string,
	asset *Asset,
) (VariantPrepareResult, error) {
	irProviders, err := n.deps.Workspace.ListAIProviders(ctx, workspaceID, ai.CapBgRemove|ai.CapImageToImage)
	if err != nil {
		return VariantPrepareResult{}, err
	}
	imageRouterConfigured := providerConfigured(irProviders, ai.ProviderImageRouter)

	var nodeCfg struct {
		Type     string          `json:"type"`
		Params   json.RawMessage `json:"params"`
		Title    *string         `json:"title,omitempty"`
		IsShared bool            `json:"is_shared"`
	}
	if unmarshalErr := json.Unmarshal(cfg, &nodeCfg); unmarshalErr != nil {
		return VariantPrepareResult{}, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}

	assetID, _ := rcGetString(rc, "asset_id")
	return n.deps.Variants.PrepareCreate(ctx, VariantPrepareRequest{
		WorkspaceID:           workspaceID,
		AssetID:               assetID,
		Type:                  nodeCfg.Type,
		Params:                nodeCfg.Params,
		AssetMimeType:         asset.MimeType,
		ImageRouterConfigured: imageRouterConfigured,
		DefaultImageModel:     n.deps.Config.ImageRouter.DefaultModel,
		DefaultBgRemoveModel:  n.deps.Config.ImageRouter.DefaultBgRemoveModel,
		Title:                 nodeCfg.Title,
		IsShared:              nodeCfg.IsShared,
	})
}

func (n createVariantNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	workspaceID, asset, err := n.resolveAssetForVariant(ctx, rc)
	if err != nil {
		return "", nil, err
	}

	prepared, err := n.prepareVariant(ctx, rc, cfg, workspaceID, asset)
	if err != nil {
		return "", nil, err
	}

	versionID, err := rcRequireString(rc, "version_id")
	if err != nil {
		return "", nil, err
	}
	versionNum := rcGetVersionNum(rc)
	storageKey, _ := rcGetString(rc, "storage_key")
	variantID := uuid.NewString()
	vjp := buildVariantJobPayload(asset, versionID, versionNum, variantID, storageKey, prepared)

	// If the executor pre-populated a continuation (meaning a set_new_version
	// node is wired as our successor), embed it in the job payload so the job
	// worker can resume the workflow run once the variant is ready.
	if contVal, ok := rc.Get(rcKeyContinuation); ok {
		if cont, isCont := contVal.(NodeContinuation); isCont {
			// Snapshot the current context (before variant outputs) for the resume.
			cont.ContextJSON = mustJSON(rc)
			// Embed variant_id so the resumed node can look up the variant row.
			seed := jsonToMap(cont.ContextJSON)
			seed["variant_id"] = variantID
			cont.ContextJSON = mustJSON(NewRunContext(seed))
			vjp.Continuation = &cont
			payload, _ := json.Marshal(vjp)
			if _, enqErr := n.deps.Queue.Enqueue(ctx, workspaceID, prepared.Type, string(payload)); enqErr != nil {
				return "", nil, enqErr
			}
			// Return portContinued (no edges) — the job worker will resume the run.
			return portContinued, map[string]any{
				"variant_id":   variantID,
				"variant_type": prepared.Type,
			}, nil
		}
	}

	payload, _ := json.Marshal(vjp)
	job, err := n.deps.Queue.Enqueue(ctx, workspaceID, prepared.Type, string(payload))
	if err != nil {
		return "", nil, err
	}
	return portOut, map[string]any{
		"variant_id":     variantID,
		"variant_job_id": job.ID,
		"variant_type":   prepared.Type,
	}, nil
}

func buildVariantJobPayload(
	asset *Asset,
	versionID string,
	versionNum int64,
	variantID, storageKey string,
	prepared VariantPrepareResult,
) VariantJobPayload {
	return VariantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		VersionID:   versionID,
		VersionNum:  versionNum,
		VariantID:   variantID,
		StorageKey:  storageKey,
		MimeType:    asset.MimeType,
		Type:        prepared.Type,
		Params:      prepared.Params,
		Title:       prepared.Title,
		IsShared:    prepared.IsShared,
	}
}

type createShareNode struct {
	deps   Deps
	schema NodeSchema
}

func (n createShareNode) Schema() NodeSchema { return n.schema }

func (n createShareNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	var nodeCfg struct {
		Label         string `json:"label"`
		AllowComments bool   `json:"allow_comments"`
		AllowDownload bool   `json:"allow_download"`
		ExpiresInDays *int   `json:"expires_in_days"`
	}
	if cfgErr := json.Unmarshal(cfg, &nodeCfg); cfgErr != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	createdBy := actorUserID(ctx, rc)
	if createdBy == "" {
		return "", nil, fmt.Errorf("workflow_created_by is required for share creation: %w", apperr.ErrInvalidInput)
	}
	shareID, err := n.deps.Shares.Create(ctx, workspaceID, ShareCreateParams{
		CreatedBy:     createdBy,
		Label:         nodeCfg.Label,
		TargetType:    "asset",
		TargetID:      assetID,
		ExpiresInDays: nodeCfg.ExpiresInDays,
		AllowComments: nodeCfg.AllowComments,
		AllowDownload: nodeCfg.AllowDownload,
	})
	if err != nil {
		return "", nil, err
	}
	return portOut, map[string]any{"share_id": shareID}, nil
}

type tagAssetNode struct {
	deps   Deps
	schema NodeSchema
}

func (n tagAssetNode) Schema() NodeSchema { return n.schema }

func (n tagAssetNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	var nodeCfg struct {
		Name string `json:"name"`
	}
	if cfgErr := json.Unmarshal(cfg, &nodeCfg); cfgErr != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	slog.DebugContext(ctx, "action.tag: applying tag",
		"workspace_id", workspaceID,
		"asset_id", assetID,
		"tag_name", nodeCfg.Name,
	)
	tagName, err := n.deps.Tags.AddToAsset(ctx, workspaceID, assetID, nodeCfg.Name)
	if err != nil {
		slog.ErrorContext(ctx, "action.tag: failed to apply tag", "err", err,
			"workspace_id", workspaceID, "asset_id", assetID, "tag_name", nodeCfg.Name)
		return "", nil, err
	}
	slog.DebugContext(ctx, "action.tag: tag applied", "tag_name", tagName)
	return portOut, map[string]any{"tag_name": tagName}, nil
}

type moveAssetNode struct {
	deps   Deps
	schema NodeSchema
}

func (n moveAssetNode) Schema() NodeSchema { return n.schema }

func (n moveAssetNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	var nodeCfg struct {
		FolderID  *string `json:"folder_id"`
		ProjectID *string `json:"project_id"`
	}
	if cfgErr := json.Unmarshal(cfg, &nodeCfg); cfgErr != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	asset, err := n.deps.Assets.Move(ctx, workspaceID, assetID, AssetMoveParams{
		FolderID:  nodeCfg.FolderID,
		ProjectID: nodeCfg.ProjectID,
	})
	if err != nil {
		return "", nil, err
	}
	return portOut, map[string]any{"folder_id": asset.FolderID, "project_id": asset.ProjectID}, nil
}

type setFieldNode struct {
	deps   Deps
	schema NodeSchema
}

func (n setFieldNode) Schema() NodeSchema { return n.schema }

func (n setFieldNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	if n.deps.AssetFields == nil {
		return "", nil, errors.New("asset field service unavailable")
	}
	var nodeCfg struct {
		FieldID string `json:"field_id"`
		Value   any    `json:"value"`
	}
	if cfgErr := json.Unmarshal(cfg, &nodeCfg); cfgErr != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}
	userID := actorUserID(ctx, rc)
	if userID == "" {
		return "", nil, fmt.Errorf("workflow_created_by is required for field updates: %w", apperr.ErrInvalidInput)
	}
	err = n.deps.AssetFields.SetValues(ctx, workspaceID, assetID, userID, []FieldValueInput{{
		FieldID: nodeCfg.FieldID,
		Value:   nodeCfg.Value,
	}})
	if err != nil {
		return "", nil, err
	}
	return portOut, map[string]any{"field_id": nodeCfg.FieldID, "field_value": nodeCfg.Value}, nil
}

type aiImageDescriptionNode struct {
	deps   Deps
	schema NodeSchema
}

func (n aiImageDescriptionNode) Schema() NodeSchema { return n.schema }

// injectContinuationOnAnySuccessor seeds a continuation whenever anything is
// wired to the given port. Unlike create_variant (which only resumes into
// the one dedicated set_new_version consumer), any downstream node can
// consume the output these "produce text track + maybe continue" nodes
// produce, so there's no single node type to special-case here.
func injectContinuationOnAnySuccessor(
	rc *RunContext,
	g *Graph,
	nodeID, runID, workflowID, workspaceID, port string,
) {
	rc.Delete(rcKeyContinuation)
	successors := g.Successors(nodeID, port)
	if len(successors) == 0 {
		return
	}
	rc.Set(rcKeyContinuation, NodeContinuation{
		RunID:       runID,
		NodeID:      successors[0].ID,
		WorkflowID:  workflowID,
		WorkspaceID: workspaceID,
	})
}

func (n aiImageDescriptionNode) InjectContinuation(
	rc *RunContext,
	g *Graph,
	nodeID, runID, workflowID, workspaceID string,
) {
	injectContinuationOnAnySuccessor(rc, g, nodeID, runID, workflowID, workspaceID, portOut)
}

// snapshotContinuation returns the continuation the executor pre-populated
// for this step (if any), with the current run context snapshotted into it
// so a job worker can later resume the run from this point. Returns nil if
// no continuation was seeded (nothing wired to this node's "out" port).
func snapshotContinuation(rc *RunContext) *NodeContinuation {
	contVal, ok := rc.Get(rcKeyContinuation)
	if !ok {
		return nil
	}
	cont, isCont := contVal.(NodeContinuation)
	if !isCont {
		return nil
	}
	cont.ContextJSON = mustJSON(rc)
	return &cont
}

func isCuratedVisionModel(model string) bool {
	for _, m := range ai.CuratedVisionModels {
		if m.ID == model {
			return true
		}
	}
	return false
}

// resolveImageVersionForDescription validates node dependencies, resolves
// the asset (must be an image with a current version and a configured
// description provider), and returns its current version.
func (n aiImageDescriptionNode) resolveImageVersionForDescription(
	ctx context.Context,
	rc *RunContext,
) (assetID, workspaceID string, version repository.AssetVersion, err error) {
	assetID, err = rcRequireString(rc, "asset_id")
	if err != nil {
		return "", "", repository.AssetVersion{}, err
	}
	workspaceID, err = rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", "", repository.AssetVersion{}, err
	}
	if n.deps.Assets == nil || n.deps.Versions == nil || n.deps.Workspace == nil || n.deps.TextTracks == nil {
		return "", "", repository.AssetVersion{}, errors.New(
			"workflow ai_image_description dependencies not configured",
		)
	}

	asset, err := n.deps.Assets.Get(ctx, workspaceID, assetID)
	if err != nil {
		return "", "", repository.AssetVersion{}, err
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return "", "", repository.AssetVersion{}, fmt.Errorf("asset is not an image: %w", apperr.ErrInvalidInput)
	}
	if asset.CurrentVersionID == nil {
		return "", "", repository.AssetVersion{}, fmt.Errorf("asset has no current version: %w", apperr.ErrInvalidInput)
	}

	descProviders, err := n.deps.Workspace.ListAIProviders(ctx, workspaceID, ai.CapImageDescription)
	if err != nil {
		return "", "", repository.AssetVersion{}, err
	}
	if !anyProviderConfigured(descProviders) {
		return "", "", repository.AssetVersion{}, fmt.Errorf(
			"no AI provider configured for image description: %w", apperr.ErrInvalidInput,
		)
	}

	version, err = n.deps.Versions.GetByID(ctx, *asset.CurrentVersionID)
	if err != nil {
		return "", "", repository.AssetVersion{}, fmt.Errorf("get current version: %w", err)
	}
	return assetID, workspaceID, version, nil
}

// parseImageDescriptionConfig validates and defaults the node config,
// returning the resolved model, prompt, and language to use.
func parseImageDescriptionConfig(cfg json.RawMessage) (model, prompt, lang string, err error) {
	var nodeCfg struct {
		Model  string `json:"model"`
		Lang   string `json:"lang"`
		Prompt string `json:"prompt"`
	}
	if unmarshalErr := json.Unmarshal(cfg, &nodeCfg); unmarshalErr != nil {
		return "", "", "", fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}

	model = strings.TrimSpace(nodeCfg.Model)
	switch {
	case model == "":
		model = ai.DefaultVisionModel
	case !isCuratedVisionModel(model):
		return "", "", "", fmt.Errorf("unsupported model %q: %w", model, apperr.ErrInvalidInput)
	}

	prompt = nodeCfg.Prompt
	if strings.TrimSpace(prompt) == "" {
		prompt = ai.DefaultImageDescriptionPrompt
	} else if len(prompt) > ai.MaxImageDescriptionPromptLen {
		return "", "", "", fmt.Errorf("prompt too long: %w", apperr.ErrInvalidInput)
	}

	lang = strings.TrimSpace(nodeCfg.Lang)
	if lang == "" {
		lang = "en"
	}
	return model, prompt, lang, nil
}

func (n aiImageDescriptionNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	assetID, workspaceID, version, err := n.resolveImageVersionForDescription(ctx, rc)
	if err != nil {
		return "", nil, err
	}

	model, prompt, lang, err := parseImageDescriptionConfig(cfg)
	if err != nil {
		return "", nil, err
	}

	// If the executor pre-populated a continuation (meaning at least one node
	// is wired to our "out" port), snapshot the current context so the job
	// worker can resume the run once the description is ready.
	cont := snapshotContinuation(rc)

	trackID, err := n.deps.TextTracks.CreateAIImageDescription(ctx, workspaceID, TextTrackCreateParams{
		AssetID:      assetID,
		StorageKey:   version.StorageKey,
		MimeType:     version.MimeType,
		Model:        model,
		Prompt:       prompt,
		Lang:         lang,
		Continuation: cont,
	})
	if err != nil {
		return "", nil, err
	}

	if cont != nil {
		return portContinued, map[string]any{outputKeyTrackID: trackID}, nil
	}
	return portOut, map[string]any{outputKeyTrackID: trackID}, nil
}

type ocrNode struct {
	deps   Deps
	schema NodeSchema
}

func (n ocrNode) Schema() NodeSchema { return n.schema }

// InjectContinuation seeds a continuation whenever anything is wired to the
// "out" port, same as aiImageDescriptionNode — any downstream node can
// consume the text/track_id this node produces.
func (n ocrNode) InjectContinuation(
	rc *RunContext,
	g *Graph,
	nodeID, runID, workflowID, workspaceID string,
) {
	injectContinuationOnAnySuccessor(rc, g, nodeID, runID, workflowID, workspaceID, portOut)
}

func (n ocrNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	if n.deps.Assets == nil || n.deps.Versions == nil || n.deps.TextTracks == nil {
		return "", nil, errors.New("workflow ocr dependencies not configured")
	}

	asset, err := n.deps.Assets.Get(ctx, workspaceID, assetID)
	if err != nil {
		return "", nil, err
	}
	if !transform.SupportedOCRMIMEs[asset.MimeType] {
		return "", nil, fmt.Errorf(
			"asset mime type %q is not supported for OCR: %w",
			asset.MimeType,
			apperr.ErrInvalidInput,
		)
	}
	if asset.CurrentVersionID == nil {
		return "", nil, fmt.Errorf("asset has no current version: %w", apperr.ErrInvalidInput)
	}

	version, err := n.deps.Versions.GetByID(ctx, *asset.CurrentVersionID)
	if err != nil {
		return "", nil, fmt.Errorf("get current version: %w", err)
	}

	var nodeCfg struct {
		Lang         string `json:"lang"`
		OutputFormat string `json:"output_format"`
	}
	if unmarshalErr := json.Unmarshal(cfg, &nodeCfg); unmarshalErr != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}

	lang := strings.TrimSpace(nodeCfg.Lang)
	if lang == "" {
		lang = "eng"
	}

	outputFormat := strings.TrimSpace(nodeCfg.OutputFormat)
	if outputFormat == "" {
		outputFormat = ocrOutputFormatTxt
	} else if outputFormat != ocrOutputFormatTxt && outputFormat != ocrOutputFormatHocr {
		return "", nil, fmt.Errorf("unsupported output format %q: %w", outputFormat, apperr.ErrInvalidInput)
	}

	// If the executor pre-populated a continuation (meaning at least one node
	// is wired to our "out" port), snapshot the current context so the job
	// worker can resume the run once the OCR text is ready.
	cont := snapshotContinuation(rc)

	trackID, err := n.deps.TextTracks.CreateOCR(ctx, workspaceID, TextTrackCreateOCRParams{
		AssetID:      assetID,
		StorageKey:   version.StorageKey,
		MimeType:     version.MimeType,
		Lang:         lang,
		OutputFormat: outputFormat,
		Continuation: cont,
	})
	if err != nil {
		return "", nil, err
	}

	if cont != nil {
		return portContinued, map[string]any{outputKeyTrackID: trackID}, nil
	}
	return portOut, map[string]any{outputKeyTrackID: trackID}, nil
}

type autoTagNode struct {
	deps   Deps
	schema NodeSchema
}

func (n autoTagNode) Schema() NodeSchema { return n.schema }

// InjectContinuation seeds a continuation whenever anything is wired to the
// "out" port, same as ocrNode — any downstream node can consume the
// tags/count this node produces once tagging finishes.
func (n autoTagNode) InjectContinuation(
	rc *RunContext,
	g *Graph,
	nodeID, runID, workflowID, workspaceID string,
) {
	injectContinuationOnAnySuccessor(rc, g, nodeID, runID, workflowID, workspaceID, portOut)
}

func (n autoTagNode) Execute(
	ctx context.Context,
	rc *RunContext,
	cfg json.RawMessage,
) (string, map[string]any, error) {
	assetID, err := rcRequireString(rc, "asset_id")
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := rcRequireString(rc, "workspace_id")
	if err != nil {
		return "", nil, err
	}
	if n.deps.Assets == nil || n.deps.AutoTag == nil {
		return "", nil, errors.New("workflow auto_tag dependencies not configured")
	}

	asset, err := n.deps.Assets.Get(ctx, workspaceID, assetID)
	if err != nil {
		return "", nil, err
	}
	if !transform.IsAutoTaggable(asset.MimeType) {
		return "", nil, fmt.Errorf(
			"asset mime type %q is not eligible for auto-tagging: %w",
			asset.MimeType,
			apperr.ErrInvalidInput,
		)
	}

	var nodeCfg struct {
		Mode string `json:"mode"`
	}
	if unmarshalErr := json.Unmarshal(cfg, &nodeCfg); unmarshalErr != nil {
		return "", nil, fmt.Errorf("invalid node config: %w", apperr.ErrInvalidInput)
	}

	mode := strings.TrimSpace(nodeCfg.Mode)
	switch mode {
	case "":
		mode = autoTagModePending
	case autoTagModePending, autoTagModeSilent:
	default:
		return "", nil, fmt.Errorf("unsupported mode %q: %w", mode, apperr.ErrInvalidInput)
	}

	// If the executor pre-populated a continuation (meaning at least one node
	// is wired to our "out" port), snapshot the current context so the job
	// worker can resume the run once tagging is ready.
	cont := snapshotContinuation(rc)

	if err = n.deps.AutoTag.Enqueue(ctx, workspaceID, assetID, mode, cont); err != nil {
		return "", nil, err
	}

	if cont != nil {
		return portContinued, map[string]any{"mode": mode}, nil
	}
	return portOut, map[string]any{"mode": mode}, nil
}
