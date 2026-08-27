package ingress

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	sniffFirstBytes  = 512
	ErrorCountCutoff = 5
)

// ErrStorageLimitReached is the sentinel returned by StorageLimitChecker.CheckLimit.
// service.ErrStorageLimitReached equals this value.
var ErrStorageLimitReached = errors.New("storage limit reached")

// StorageLimitChecker is implemented by service.StorageService. Defined here
// to avoid an import cycle between the ingress and service packages.
type StorageLimitChecker interface {
	CheckLimit(ctx context.Context, workspaceID string, incomingBytes int64) error
	Invalidate(workspaceID string)
}

// Worker handles ingest_poll and ingest_fetch jobs.
type Worker struct {
	queries    *dbgen.Queries
	sqlDB      *sql.DB
	storage    storage.Storage
	queue      queue.JobQueue
	cfg        *config.Config
	audit      *audit.EventWriter
	mailer     mail.Mailer
	ingester   assetio.Ingester
	storageSvc StorageLimitChecker // may be nil when not configured
}

// NewWorker creates a Worker.
func NewWorker(
	queries *dbgen.Queries,
	sqlDB *sql.DB,
	stor storage.Storage,
	qu queue.JobQueue,
	cfg *config.Config,
	au *audit.EventWriter,
	mailer mail.Mailer,
	ingester assetio.Ingester,
	storageSvc StorageLimitChecker,
) *Worker {
	return &Worker{
		queries:    queries,
		sqlDB:      sqlDB,
		storage:    stor,
		queue:      qu,
		cfg:        cfg,
		audit:      au,
		mailer:     mailer,
		ingester:   ingester,
		storageSvc: storageSvc,
	}
}

// PollJobPayload is the JSON payload for a JobTypeIngestPoll job.
type PollJobPayload struct {
	SourceID    string `json:"source_id"`
	WorkspaceID string `json:"workspace_id"`
}

// FetchJobPayload is the JSON payload for a JobTypeIngestFetch job.
type FetchJobPayload struct {
	SourceID         string            `json:"source_id"`
	WorkspaceID      string            `json:"workspace_id"`
	LogEntryID       string            `json:"log_entry_id"`
	RemoteID         string            `json:"remote_id"`
	Filename         string            `json:"filename"`
	TmpPath          string            `json:"tmp_path,omitempty"`
	OverrideFolderID string            `json:"override_folder_id,omitempty"`
	Meta             map[string]string `json:"meta,omitempty"`
}

// HandlePoll is the queue.HandlerFunc for JobTypeIngestPoll.
// It opens the source, calls Poll(), inserts log entries, and enqueues fetch jobs.
func (w *Worker) HandlePoll(ctx context.Context, job dbgen.Job) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "workers.ingest.poll")
	defer telemetry.EndSpan(span, err)

	var p PollJobPayload
	if err = json.Unmarshal([]byte(job.Payload), &p); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("ingest_poll: parse payload: %w", err)
	}

	src, err := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: p.SourceID, WorkspaceID: p.WorkspaceID,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("ingest_poll: get source %s: %w", p.SourceID, err)
	}

	span.SetAttributes(attribute.String("damask.ingest.poll.workspace_id", p.WorkspaceID))
	span.SetAttributes(attribute.String("damask.ingest.poll.source_id", p.SourceID))

	if src.Enabled == 0 {
		return nil
	}

	configJSON, err := DecryptConfig(w.cfg.AppSecret, src.Config)
	if err != nil {
		return w.failSource(ctx, src, fmt.Errorf("ingest_poll: decrypt config: %w", err))
	}

	span.SetAttributes(attribute.String("damask.ingest.poll.source_type", src.Type))

	source, err := Build(src.Type, configJSON)
	if err != nil {
		return w.failSource(ctx, src, fmt.Errorf("ingest_poll: build source: %w", err))
	}

	items, err := source.Poll(ctx)
	if err != nil {
		return w.failSource(ctx, src, fmt.Errorf("ingest_poll: poll: %w", err))
	}

	for _, item := range items {
		entryID := uuid.NewString()

		_, itemSpan := telemetry.StartSpan(ctx, "workers.ingest.poll.item",
			attribute.String("damask.ingest.poll.entry_id", entryID),
		)
		defer itemSpan.End()

		var entry dbgen.IngressLog
		entry, err = w.queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
			ID:       entryID,
			SourceID: src.ID,
			RemoteID: item.RemoteID,
			Filename: item.Filename,
		})
		if errors.Is(err, sql.ErrNoRows) {
			// INSERT OR IGNORE: row already exists — item already known
			continue
		}
		if err != nil {
			slog.ErrorContext(ctx, "ingest_poll: insert log entry", "remote_id", item.RemoteID, "error", err)
			continue
		}

		payload, _ := json.Marshal(FetchJobPayload{
			SourceID:    src.ID,
			WorkspaceID: src.WorkspaceID,
			LogEntryID:  entry.ID,
			RemoteID:    item.RemoteID,
			Filename:    item.Filename,
			Meta:        item.Meta,
		})
		if _, enqErr := w.queue.Enqueue(
			ctx,
			src.WorkspaceID,
			queue.JobTypeIngestFetch,
			string(payload),
		); enqErr != nil {
			slog.ErrorContext(ctx, "ingest_poll: enqueue fetch", "remote_id", item.RemoteID, "error", enqErr)
		}
	}

	return w.markPolledSuccess(ctx, src.ID)
}

// HandleFetch is the queue.HandlerFunc for JobTypeIngestFetch.
// It downloads one item, validates MIME, stores it, and creates an asset row.
func (w *Worker) HandleFetch(ctx context.Context, job dbgen.Job) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "workers.ingest.fetch")
	defer telemetry.EndSpan(span, err)

	var p FetchJobPayload
	if err = json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("ingest_fetch: parse payload: %w", err)
	}
	span.SetAttributes(
		attribute.String("damask.ingest.workspace_id", p.WorkspaceID),
		attribute.String("damask.ingest.entry_id", p.LogEntryID),
	)

	entry, err := w.queries.GetIngressLogEntry(ctx, p.LogEntryID)
	if err != nil {
		return fmt.Errorf("ingest_fetch: get log entry %s: %w", p.LogEntryID, err)
	}
	span.SetAttributes(
		attribute.String("damask.ingest.entry_remote_id", entry.RemoteID),
		attribute.String("damask.ingest.entry_status", entry.Status),
	)
	if entry.Status != "pending" { // idempotency
		return nil
	}

	src, source, err := w.resolveSource(ctx, p.SourceID, p.WorkspaceID)
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, err)
	}
	span.SetAttributes(
		attribute.String("damask.ingest.entry_source_id", src.ID),
		attribute.String("damask.ingest.entry_source_label", src.Label),
		attribute.String("damask.ingest.entry_source_type", src.Type),
	)

	rules, rulesErr := w.queries.ListIngressRules(ctx, src.ID)
	if rulesErr != nil {
		slog.WarnContext(
			ctx,
			"ingest_fetch: list rules (continuing without rules)",
			"source_id",
			src.ID,
			"error",
			rulesErr,
		)
	}
	span.SetAttributes(attribute.Int("damask.ingest.entry_source_rules", len(rules)))

	ruleResult, skip, err := w.applyGatingChecks(ctx, entry, p.WorkspaceID, rules)
	span.SetAttributes(attribute.Bool("damask.ingest.entry_source_rules_pass", ruleResult.Allow))
	if skip || err != nil {
		return err
	}

	projectID, folderID := w.resolveDestination(src, p, ruleResult)
	if folderID != nil {
		span.SetAttributes(attribute.String("damask.ingest.entry_folder_id", *folderID))
	}
	if projectID != nil {
		span.SetAttributes(attribute.String("damask.ingest.entry_project_id", *projectID))
	}

	namedTmp, cleanup, err := w.streamToNamedTemp(ctx, entry, src, p, source)
	if err != nil {
		return err
	}
	defer cleanup()

	return w.finalizeImport(ctx, span, namedTmp, entry, src, projectID, folderID)
}

func (w *Worker) resolveSource(ctx context.Context, sourceID, workspaceID string) (dbgen.IngressSource, Source, error) {
	src, err := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return src, nil, fmt.Errorf("ingest_fetch: get source: %w", err)
	}
	configJSON, err := DecryptConfig(w.cfg.AppSecret, src.Config)
	if err != nil {
		return src, nil, fmt.Errorf("ingest_fetch: decrypt config: %w", err)
	}
	source, err := Build(src.Type, configJSON)
	if err != nil {
		return src, nil, fmt.Errorf("ingest_fetch: build source: %w", err)
	}
	return src, source, nil
}

func (w *Worker) applyGatingChecks(
	ctx context.Context,
	entry dbgen.IngressLog,
	workspaceID string,
	rules []dbgen.IngressRule,
) (RuleResult, bool, error) {
	ruleResult := EvaluateRules(rules, ItemMeta{Filename: entry.Filename})
	if !ruleResult.Allow {
		skipped := ingressStatusSkipped
		_ = w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
			Status: ingressStatusSkipped, ID: entry.ID, Error: &skipped,
		})
		return ruleResult, true, nil
	}

	// Storage limit pre-check before fetching any bytes. incomingBytes=0 because
	// the file hasn't been downloaded yet; this blocks workspaces already over
	// their limit (total > limit) but allows workspaces at exactly 100% to
	// receive one more file before being cut off — acceptable for ingress.
	if w.storageSvc != nil {
		if err := w.storageSvc.CheckLimit(ctx, workspaceID, 0); err != nil {
			if errors.Is(err, ErrStorageLimitReached) {
				skipped := "storage limit reached"
				_ = w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
					Status: ingressStatusSkipped, ID: entry.ID, Error: &skipped,
				})
				return ruleResult, true, nil
			}
			return ruleResult, false, err
		}
	}
	return ruleResult, false, nil
}

// resolveDestination applies the priority chain: source defaults < payload override < rule result.
func (w *Worker) resolveDestination(
	src dbgen.IngressSource,
	p FetchJobPayload,
	ruleResult RuleResult,
) (projectID *string, folderID *string) {
	projectID = src.DestProjectID
	folderID = src.DestFolderID
	if p.OverrideFolderID != "" {
		folderID = &p.OverrideFolderID
	}
	if ruleResult.FolderID != nil {
		folderID = ruleResult.FolderID
	}
	if ruleResult.ProjectID != nil {
		projectID = ruleResult.ProjectID
	}
	return projectID, folderID
}

func (w *Worker) streamToNamedTemp(
	ctx context.Context,
	entry dbgen.IngressLog,
	src dbgen.IngressSource,
	p FetchJobPayload,
	source Source,
) (namedTmp string, cleanup func(), err error) {
	// Fetch the item — either from a pre-written temp file (email_api push path)
	// or by calling source.Fetch() (pull sources).
	var rc io.ReadCloser
	if p.TmpPath != "" {
		slog.DebugContext(ctx, "use existing temp file")
		f, openErr := os.Open(p.TmpPath)
		if openErr != nil {
			return "", func() {}, w.failEntry(
				ctx,
				entry.ID,
				src,
				fmt.Errorf("ingest_fetch: open tmp file: %w", openErr),
			)
		}
		rc = f
	} else {
		slog.DebugContext(ctx, "use fetch from source")
		rc, err = source.Fetch(ctx, IngestItem{RemoteID: entry.RemoteID, Filename: entry.Filename, Meta: p.Meta})
		if err != nil {
			return "", func() {}, w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: fetch item: %w", err))
		}
	}
	defer rc.Close()

	// Sniff MIME type from first 512 bytes, then stream to temp file
	sniff := make([]byte, sniffFirstBytes)
	n, _ := io.ReadFull(rc, sniff)
	sniff = sniff[:n]
	mimeType := http.DetectContentType(sniff)
	_ = mimeType // available for future MIME allowlist check

	tmp, err := os.CreateTemp("", "ingest-*")
	if err != nil {
		return "", func() {}, w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: create temp file: %w", err))
	}
	tmpPath := tmp.Name()
	slog.DebugContext(ctx, "use temp file", "path", tmpPath)

	copied, err := io.Copy(tmp, io.MultiReader(bytes.NewReader(sniff), rc))
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", func() {}, w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: write temp file: %w", err))
	}
	slog.DebugContext(ctx, "wrote temp file", "path", tmpPath, "bytes", copied)
	_ = tmp.Close()

	// Rename temp file to use original filename for CreateAsset
	namedTmp = filepath.Join(os.TempDir(), filepath.Base(entry.Filename))
	if err = os.Rename(tmpPath, namedTmp); err != nil {
		namedTmp = tmpPath // fall back to random name
	}
	slog.DebugContext(ctx, "ingest file", "path", namedTmp)

	cleanup = func() {
		_ = os.Remove(namedTmp)
		if p.TmpPath != "" {
			_ = os.Remove(p.TmpPath)
		}
	}
	return namedTmp, cleanup, nil
}

func (w *Worker) finalizeImport(
	ctx context.Context,
	span trace.Span,
	namedTmp string,
	entry dbgen.IngressLog,
	src dbgen.IngressSource,
	projectID *string,
	folderID *string,
) error {
	asset, err := w.ingester.IngestFile(ctx, src.WorkspaceID, namedTmp, assetio.IngestFileOpts{
		ProjectID: projectID,
		FolderID:  folderID,
		UserID:    src.CreatedBy,
	})
	if err != nil {
		var dupErr *assetio.DuplicateConflictError
		if errors.As(err, &dupErr) {
			return w.skipDuplicateEntry(ctx, entry.ID, dupErr.Match.AssetID)
		}
		slog.ErrorContext(ctx, "ingest_fetch: cannot create asset", "error", err)
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: create asset: %w", err))
	}

	slog.DebugContext(ctx, "update ingress log entry", "entry_id", entry.ID, "asset_id", &asset.ID)

	assetID := asset.ID
	if w.storageSvc != nil {
		w.storageSvc.Invalidate(src.WorkspaceID)
	}

	var duplicateOfAssetID *string
	if asset.DuplicateOf != nil {
		duplicateOfAssetID = &asset.DuplicateOf.AssetID
	}
	if err = w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status:             ingressStatusImported,
		AssetID:            &assetID,
		DuplicateOfAssetID: duplicateOfAssetID,
		ID:                 entry.ID,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "ingest_fetch: update log entry", "entry_id", entry.ID, "error", err)
	}

	w.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: src.WorkspaceID,
		AssetID:     asset.ID,
		UserID:      nil,
		ActorType:   audit.ActorTypeSystem,
		EventType:   audit.EventAssetCreated,
		Payload: audit.AssetCreatedPayload{
			V:        1,
			Filename: asset.OriginalFilename,
			Source:   "ingress",
			SourceID: src.Label,
		},
	})

	tag, err := w.queries.GetOrCreateTag(ctx, dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: asset.WorkspaceID,
		Name:        src.Label,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "ingest_fetch: could not get or create tag", "error", err)
	}

	slog.DebugContext(ctx, "tag new entry", "tag", src.Label, "asset_id", &asset.ID)

	_ = w.queries.AddTagToAsset(ctx, dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tag.ID,
	})

	return nil
}

func (w *Worker) failSource(ctx context.Context, src dbgen.IngressSource, err error) error {
	msg := err.Error()
	_ = w.queries.MarkIngressSourceError(ctx, dbgen.MarkIngressSourceErrorParams{
		ID: src.ID, LastError: &msg,
	})
	// Fetch updated error_count to decide which email to send.
	updated, dbErr := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: src.ID, WorkspaceID: src.WorkspaceID,
	})
	disabled := dbErr == nil && updated.ErrorCount > ErrorCountCutoff
	w.notifySourceFailure(ctx, src, msg, disabled)
	return err
}

func (w *Worker) notifySourceFailure(ctx context.Context, src dbgen.IngressSource, errMsg string, disabled bool) {
	var err error
	ctx, span := telemetry.StartSpan(ctx, "workers.ingest.notify_failure")
	defer func() { telemetry.EndSpan(span, err) }()

	members, dbErr := w.queries.ListMembers(ctx, src.WorkspaceID)
	if dbErr != nil {
		return
	}
	notified := map[string]bool{}
	for _, m := range members {
		shouldSend := m.Role == string(auth.Owner) || m.UserID == src.CreatedBy
		if !shouldSend || notified[m.Email] {
			continue
		}
		notified[m.Email] = true
		var mailErr error
		if disabled {
			mailErr = w.mailer.SendIngressSourceDisabled(ctx, m.Email, src.Label, errMsg, src.WorkspaceID)
		} else {
			mailErr = w.mailer.SendIngressSourceFailed(ctx, m.Email, src.Label, errMsg, src.WorkspaceID)
		}
		if mailErr != nil {
			slog.ErrorContext(ctx, "failed to send ingress failure mail", "error", mailErr)
		}
	}
}

func (w *Worker) markPolledSuccess(ctx context.Context, sourceID string) error {
	return w.queries.MarkIngressSourceSuccess(ctx, sourceID)
}

func (w *Worker) markPolledError(ctx context.Context, sourceID, msg string) error {
	return w.queries.MarkIngressSourceError(ctx, dbgen.MarkIngressSourceErrorParams{
		ID: sourceID, LastError: &msg,
	})
}

// skipDuplicateEntry records a block-mode content-hash duplicate as
// "skipped_duplicate" rather than routing it through failEntry: this is
// expected, per-workspace-setting behavior, not a source malfunction, so it
// must not increment the source's error count or trigger a failure notification.
func (w *Worker) skipDuplicateEntry(ctx context.Context, entryID, duplicateOfAssetID string) error {
	_ = w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status:             ingressStatusSkippedDup,
		DuplicateOfAssetID: &duplicateOfAssetID,
		ID:                 entryID,
	})
	return nil
}

func (w *Worker) failEntry(ctx context.Context, entryID string, src dbgen.IngressSource, err error) error {
	msg := err.Error()
	_ = w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status: "error",
		Error:  &msg,
		ID:     entryID,
	})
	_ = w.markPolledError(ctx, src.ID, msg)
	updated, dbErr := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: src.ID, WorkspaceID: src.WorkspaceID,
	})
	disabled := dbErr == nil && updated.ErrorCount > ErrorCountCutoff
	w.notifySourceFailure(ctx, src, msg, disabled)
	return err
}
