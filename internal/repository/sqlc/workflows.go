package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	"damask/server/internal/repository"
)

type workflowRepo struct {
	writer   *sql.DB // kept for BeginTx; nil on the tx-scoped clone
	writerDB rawDB
	readerDB rawDB
}

func NewWorkflowRepo(d *db.DB) repository.WorkflowRepository {
	return &workflowRepo{writer: d.Writer, writerDB: d.Writer, readerDB: d.Reader}
}

func (r *workflowRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Workflow, error) {
	row := r.readerDB.QueryRowContext(
		ctx,
		`SELECT id, workspace_id, name, description, enabled, trigger_type, trigger_config, graph, notify_on_failure_email, last_run_at, created_by, created_at, updated_at FROM workflows WHERE id = ? AND workspace_id = ?`,
		id,
		workspaceID,
	)
	return scanWorkflow(row)
}

func (r *workflowRepo) List(ctx context.Context, workspaceID string) ([]repository.Workflow, error) {
	rows, err := r.readerDB.QueryContext(
		ctx,
		`SELECT id, workspace_id, name, description, enabled, trigger_type, trigger_config, graph, notify_on_failure_email, last_run_at, created_by, created_at, updated_at FROM workflows WHERE workspace_id = ? ORDER BY created_at DESC, id DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkflowRows(rows)
}

func (r *workflowRepo) ListByTrigger(ctx context.Context, triggerType string) ([]repository.Workflow, error) {
	rows, err := r.readerDB.QueryContext(
		ctx,
		`SELECT id, workspace_id, name, description, enabled, trigger_type, trigger_config, graph, notify_on_failure_email, last_run_at, created_by, created_at, updated_at FROM workflows WHERE trigger_type = ? AND enabled = 1 ORDER BY created_at DESC, id DESC`,
		triggerType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkflowRows(rows)
}

func (r *workflowRepo) ListEnabledByTrigger(
	ctx context.Context,
	workspaceID, triggerType string,
) ([]repository.Workflow, error) {
	rows, err := r.readerDB.QueryContext(
		ctx,
		`SELECT id, workspace_id, name, description, enabled, trigger_type, trigger_config, graph, notify_on_failure_email, last_run_at, created_by, created_at, updated_at FROM workflows WHERE workspace_id = ? AND trigger_type = ? AND enabled = 1 ORDER BY name ASC`,
		workspaceID,
		triggerType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkflowRows(rows)
}

func (r *workflowRepo) Create(ctx context.Context, p repository.CreateWorkflowParams) (repository.Workflow, error) {
	_, err := r.writerDB.ExecContext(
		ctx,
		`INSERT INTO workflows (id, workspace_id, name, description, enabled, trigger_type, trigger_config, graph, notify_on_failure_email, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID,
		p.WorkspaceID,
		p.Name,
		p.Description,
		workflowBoolToInt(p.Enabled),
		p.TriggerType,
		defaultString(p.TriggerConfig, "{}"),
		p.Graph,
		p.NotifyOnFailureEmail,
		p.CreatedBy,
	)
	if err != nil {
		return repository.Workflow{}, err
	}
	return r.GetByID(ctx, p.WorkspaceID, p.ID)
}

func (r *workflowRepo) Update(ctx context.Context, p repository.UpdateWorkflowParams) (repository.Workflow, error) {
	current, err := r.GetByID(ctx, p.WorkspaceID, p.ID)
	if err != nil {
		return repository.Workflow{}, err
	}
	if p.Name != nil {
		current.Name = *p.Name
	}
	if p.Description != nil {
		current.Description = *p.Description
	}
	if p.TriggerType != nil {
		current.TriggerType = *p.TriggerType
	}
	if p.TriggerConfig != nil {
		current.TriggerConfig = *p.TriggerConfig
	}
	if p.Graph != nil {
		current.Graph = *p.Graph
	}
	if p.NotifyOnFailureEmail != nil {
		current.NotifyOnFailureEmail = *p.NotifyOnFailureEmail
	}
	_, err = r.writerDB.ExecContext(
		ctx,
		`UPDATE workflows SET name = ?, description = ?, trigger_type = ?, trigger_config = ?, graph = ?, notify_on_failure_email = ?, updated_at = datetime('now') WHERE id = ? AND workspace_id = ?`,
		current.Name,
		current.Description,
		current.TriggerType,
		defaultString(current.TriggerConfig, "{}"),
		current.Graph,
		current.NotifyOnFailureEmail,
		current.ID,
		current.WorkspaceID,
	)
	if err != nil {
		return repository.Workflow{}, err
	}
	return r.GetByID(ctx, p.WorkspaceID, p.ID)
}

func (r *workflowRepo) FindCoveringWorkflow(
	ctx context.Context,
	workspaceID, assetID, assetProjectID, assetFolderID string,
) (*repository.CoveringWorkflow, error) {
	const q = `
		SELECT id, name, trigger_type, trigger_config, enabled
		FROM workflows
		WHERE workspace_id = ?
		  AND trigger_type = 'trigger.version_uploaded'
		  AND enabled = 1
		  AND (
			(JSON_EXTRACT(trigger_config, '$.project_id') IS NULL
			 AND JSON_EXTRACT(trigger_config, '$.folder_id') IS NULL
			 AND JSON_EXTRACT(trigger_config, '$.asset_id') IS NULL)
			OR JSON_EXTRACT(trigger_config, '$.asset_id') = ?
			OR JSON_EXTRACT(trigger_config, '$.project_id') = ?
			OR JSON_EXTRACT(trigger_config, '$.folder_id') = ?
		  )
		ORDER BY
		  CASE
			WHEN JSON_EXTRACT(trigger_config, '$.asset_id') = ? THEN 0
			WHEN JSON_EXTRACT(trigger_config, '$.folder_id') = ? THEN 1
			WHEN JSON_EXTRACT(trigger_config, '$.project_id') = ? THEN 2
			ELSE 3
		  END
		LIMIT 1`
	row := r.readerDB.QueryRowContext(
		ctx,
		q,
		workspaceID,
		assetID,
		assetProjectID,
		assetFolderID,
		assetID,
		assetFolderID,
		assetProjectID,
	)
	var wf repository.CoveringWorkflow
	var enabled int
	if err := row.Scan(&wf.ID, &wf.Name, &wf.TriggerType, &wf.TriggerConfig, &enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	wf.Enabled = enabled == 1
	return &wf, nil
}

func (r *workflowRepo) SetEnabled(ctx context.Context, workspaceID, id string, enabled bool) error {
	res, err := r.writerDB.ExecContext(
		ctx,
		`UPDATE workflows SET enabled = ?, updated_at = datetime('now') WHERE id = ? AND workspace_id = ?`,
		workflowBoolToInt(enabled),
		id,
		workspaceID,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRepo) Delete(ctx context.Context, workspaceID, id string) error {
	res, err := r.writerDB.ExecContext(ctx, `DELETE FROM workflows WHERE id = ? AND workspace_id = ?`, id, workspaceID)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRepo) TouchLastRunAt(ctx context.Context, id string) error {
	res, err := r.writerDB.ExecContext(
		ctx,
		`UPDATE workflows SET last_run_at = datetime('now'), updated_at = datetime('now') WHERE id = ?`,
		id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRepo) RunInTx(ctx context.Context, fn func(repository.WorkflowRepository) error) error {
	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txRepo := &workflowRepo{writerDB: tx, readerDB: tx}
	if err = fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type workflowRunRepo struct {
	writerDB rawDB
	readerDB rawDB
}

func NewWorkflowRunRepo(d *db.DB) repository.WorkflowRunRepository {
	return &workflowRunRepo{writerDB: d.Writer, readerDB: d.Reader}
}

func (r *workflowRunRepo) GetByID(ctx context.Context, id string) (repository.WorkflowRun, error) {
	row := r.readerDB.QueryRowContext(
		ctx,
		`SELECT id, workflow_id, workspace_id, status, trigger_data, context, error, started_at, completed_at, created_at FROM workflow_runs WHERE id = ?`,
		id,
	)
	return scanWorkflowRun(row)
}

func (r *workflowRunRepo) List(
	ctx context.Context,
	workspaceID, workflowID string,
	limit int,
	cursor string,
) ([]repository.WorkflowRun, error) {
	query := `SELECT id, workflow_id, workspace_id, status, trigger_data, context, error, started_at, completed_at, created_at FROM workflow_runs WHERE workspace_id = ?`
	args := []any{workspaceID}
	if workflowID != "" {
		query += ` AND workflow_id = ?`
		args = append(args, workflowID)
	}
	if cursor != "" {
		query += ` AND (created_at || '|' || id) < ?`
		args = append(args, cursor)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.readerDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkflowRunRows(rows)
}

func (r *workflowRunRepo) Create(
	ctx context.Context,
	p repository.CreateWorkflowRunParams,
) (repository.WorkflowRun, error) {
	_, err := r.writerDB.ExecContext(
		ctx,
		`INSERT INTO workflow_runs (id, workflow_id, workspace_id, status, trigger_data, context, error, started_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID,
		p.WorkflowID,
		p.WorkspaceID,
		defaultString(p.Status, "pending"),
		defaultString(p.TriggerData, "{}"),
		defaultString(p.Context, "{}"),
		p.Error,
		p.StartedAt,
		p.CompletedAt,
	)
	if err != nil {
		return repository.WorkflowRun{}, err
	}
	return r.GetByID(ctx, p.ID)
}

func (r *workflowRunRepo) SetStatus(ctx context.Context, id, status string) error {
	var startedAt any
	if status == "running" {
		startedAt = time.Now().UTC()
	}
	res, err := r.writerDB.ExecContext(
		ctx,
		`UPDATE workflow_runs SET status = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`,
		status,
		startedAt,
		id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRunRepo) SetFinal(ctx context.Context, p repository.SetWorkflowRunFinalParams) error {
	completedAt := p.CompletedAt
	if completedAt == nil {
		completedAt = func() *time.Time { now := time.Now().UTC(); return &now }()
	}
	res, err := r.writerDB.ExecContext(
		ctx,
		`UPDATE workflow_runs SET status = ?, context = ?, error = ?, completed_at = ? WHERE id = ?`,
		p.Status,
		p.Context,
		p.Error,
		completedAt,
		p.ID,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRunRepo) ListSteps(ctx context.Context, runID string) ([]repository.WorkflowRunStep, error) {
	rows, err := r.readerDB.QueryContext(
		ctx,
		`SELECT id, run_id, node_id, node_type, status, attempt, input_ctx, output_ctx, error, started_at, completed_at FROM workflow_run_steps WHERE run_id = ? ORDER BY started_at ASC, id ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkflowStepRows(rows)
}

func (r *workflowRunRepo) CreateStep(
	ctx context.Context,
	p repository.CreateWorkflowRunStepParams,
) (repository.WorkflowRunStep, error) {
	_, err := r.writerDB.ExecContext(
		ctx,
		`INSERT INTO workflow_run_steps (id, run_id, node_id, node_type, status, attempt, input_ctx, output_ctx, error, started_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID,
		p.RunID,
		p.NodeID,
		p.NodeType,
		defaultString(p.Status, "pending"),
		defaultInt(p.Attempt, 1),
		defaultString(p.InputCtx, "{}"),
		p.OutputCtx,
		p.Error,
		p.StartedAt,
		p.CompletedAt,
	)
	if err != nil {
		return repository.WorkflowRunStep{}, err
	}
	rows, err := r.ListSteps(ctx, p.RunID)
	if err != nil {
		return repository.WorkflowRunStep{}, err
	}
	for _, row := range rows {
		if row.ID == p.ID {
			return row, nil
		}
	}
	return repository.WorkflowRunStep{}, apperr.ErrNotFound
}

func (r *workflowRunRepo) SetStepStatus(ctx context.Context, id, status string) error {
	res, err := r.writerDB.ExecContext(ctx, `UPDATE workflow_run_steps SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRunRepo) SetStepFailed(ctx context.Context, id, errMsg string) error {
	res, err := r.writerDB.ExecContext(
		ctx,
		`UPDATE workflow_run_steps SET status = 'failed', error = ?, completed_at = ? WHERE id = ?`,
		errMsg,
		time.Now().UTC(),
		id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRunRepo) SetStepCompleted(ctx context.Context, id, outputCtx string) error {
	res, err := r.writerDB.ExecContext(
		ctx,
		`UPDATE workflow_run_steps SET status = 'completed', output_ctx = ?, completed_at = ? WHERE id = ?`,
		outputCtx,
		time.Now().UTC(),
		id,
	)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

func (r *workflowRunRepo) IncrementStepAttempt(ctx context.Context, id string) error {
	res, err := r.writerDB.ExecContext(ctx, `UPDATE workflow_run_steps SET attempt = attempt + 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return requireRowsAffected(res)
}

type workflowWebhookRepo struct {
	writerDB rawDB
	readerDB rawDB
}

func NewWorkflowWebhookRepo(d *db.DB) repository.WorkflowWebhookRepository {
	return &workflowWebhookRepo{writerDB: d.Writer, readerDB: d.Reader}
}

func (r *workflowWebhookRepo) GetTokenHash(ctx context.Context, workflowID string) (string, error) {
	var tokenHash string
	err := r.readerDB.QueryRowContext(ctx, `SELECT token_hash FROM workflow_webhook_tokens WHERE workflow_id = ?`, workflowID).
		Scan(&tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", apperr.ErrNotFound
		}
		return "", err
	}
	return tokenHash, nil
}

func (r *workflowWebhookRepo) Upsert(ctx context.Context, workflowID, tokenHash string) error {
	_, err := r.writerDB.ExecContext(
		ctx,
		`INSERT INTO workflow_webhook_tokens (workflow_id, token_hash) VALUES (?, ?) ON CONFLICT(workflow_id) DO UPDATE SET token_hash = excluded.token_hash`,
		workflowID,
		tokenHash,
	)
	return err
}

func (r *workflowWebhookRepo) Delete(ctx context.Context, workflowID string) error {
	_, err := r.writerDB.ExecContext(ctx, `DELETE FROM workflow_webhook_tokens WHERE workflow_id = ?`, workflowID)
	return err
}

func scanWorkflow(row scanner) (repository.Workflow, error) {
	var wf repository.Workflow
	var enabled int
	err := row.Scan(
		&wf.ID,
		&wf.WorkspaceID,
		&wf.Name,
		&wf.Description,
		&enabled,
		&wf.TriggerType,
		&wf.TriggerConfig,
		&wf.Graph,
		&wf.NotifyOnFailureEmail,
		&wf.LastRunAt,
		&wf.CreatedBy,
		&wf.CreatedAt,
		&wf.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Workflow{}, apperr.ErrNotFound
		}
		return repository.Workflow{}, err
	}
	wf.Enabled = enabled == 1
	if wf.TriggerConfig == "" {
		wf.TriggerConfig = "{}"
	}
	return wf, nil
}

func scanWorkflowRows(rows *sql.Rows) ([]repository.Workflow, error) {
	out := []repository.Workflow{}
	for rows.Next() {
		wf, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wf)
	}
	return out, rows.Err()
}

func scanWorkflowRun(row scanner) (repository.WorkflowRun, error) {
	var run repository.WorkflowRun
	err := row.Scan(
		&run.ID,
		&run.WorkflowID,
		&run.WorkspaceID,
		&run.Status,
		&run.TriggerData,
		&run.Context,
		&run.Error,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.WorkflowRun{}, apperr.ErrNotFound
		}
		return repository.WorkflowRun{}, err
	}
	return run, nil
}

func scanWorkflowRunRows(rows *sql.Rows) ([]repository.WorkflowRun, error) {
	out := []repository.WorkflowRun{}
	for rows.Next() {
		run, err := scanWorkflowRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func scanWorkflowStepRows(rows *sql.Rows) ([]repository.WorkflowRunStep, error) {
	out := []repository.WorkflowRunStep{}
	for rows.Next() {
		var step repository.WorkflowRunStep
		if err := rows.Scan(
			&step.ID,
			&step.RunID,
			&step.NodeID,
			&step.NodeType,
			&step.Status,
			&step.Attempt,
			&step.InputCtx,
			&step.OutputCtx,
			&step.Error,
			&step.StartedAt,
			&step.CompletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, step)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func workflowBoolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func requireRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func defaultInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}
