package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/export"
	"damask/server/internal/ingress"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/storage"

	"github.com/google/uuid"
)

type exportService struct {
	// queries and sqlDB are kept only to satisfy internal/export.BuildParams, which
	// does its own bulk asset/variant queries directly (like internal/ingress/worker.go,
	// that package is out of scope for the repository-extraction recipe here).
	queries    *dbgen.Queries
	sqlDB      *sql.DB
	storage    storage.Storage
	appSecret  string
	q          queue.JobQueue
	configRepo repository.ExportConfigRepository
	runRepo    repository.ExportRunRepository
	projects   repository.ProjectRepository
}

// NewExportService creates a production ExportService with sqlc-backed repos.
func NewExportService(
	database *db.DB,
	stor storage.Storage,
	appSecret string,
	q queue.JobQueue,
	projects repository.ProjectRepository,
) ExportService {
	return &exportService{
		queries:    database.WQ,
		sqlDB:      database.Writer,
		storage:    stor,
		appSecret:  appSecret,
		q:          q,
		configRepo: reposqlc.NewExportConfigRepo(database),
		runRepo:    reposqlc.NewExportRunRepo(database),
		projects:   projects,
	}
}

// NewExportServiceWithRepos creates an ExportService with explicit repos (for tests).
func NewExportServiceWithRepos(
	queries *dbgen.Queries,
	sqlDB *sql.DB,
	stor storage.Storage,
	appSecret string,
	q queue.JobQueue,
	configRepo repository.ExportConfigRepository,
	runRepo repository.ExportRunRepository,
	projects repository.ProjectRepository,
) ExportService {
	return &exportService{
		queries:    queries,
		sqlDB:      sqlDB,
		storage:    stor,
		appSecret:  appSecret,
		q:          q,
		configRepo: configRepo,
		runRepo:    runRepo,
		projects:   projects,
	}
}

// stampWorkspaceID injects workspace_id into a dest_config JSON blob so that
// destinations (e.g. gdrive) can look up the OAuth connection at job time,
// when the request context no longer carries the workspace.
func stampWorkspaceID(cfg json.RawMessage, workspaceID string) (json.RawMessage, error) {
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return cfg, fmt.Errorf("dest_config: unmarshal: %w", err)
	}
	m["workspace_id"] = workspaceID
	out, err := json.Marshal(m)
	if err != nil {
		return cfg, fmt.Errorf("dest_config: re-marshal: %w", err)
	}
	return out, nil
}

func (s *exportService) Create(
	ctx context.Context,
	workspaceID, userID string,
	p CreateExportConfigParams,
) (*ExportConfigDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	stamped, err := stampWorkspaceID(p.DestConfig, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("export: stamp workspace: %w", err)
	}
	enc, err := ingress.EncryptConfig(s.appSecret, stamped)
	if err != nil {
		return nil, fmt.Errorf("export: encrypt dest config: %w", err)
	}
	cfg := repository.ExportConfig{
		ID:              uuid.NewString(),
		WorkspaceID:     workspaceID,
		ProjectID:       p.ProjectID,
		CreatedBy:       userID,
		Label:           p.Label,
		DestType:        p.DestType,
		DestConfigEnc:   enc,
		Versions:        p.Versions,
		IncludeVariants: p.IncludeVariants,
		ScheduleType:    p.ScheduleType,
		QuietMinutes:    p.QuietMinutes,
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	created, err := s.configRepo.Create(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return exportConfigToDTO(created), nil
}

func (s *exportService) Get(ctx context.Context, workspaceID, id string) (*ExportConfigDTO, error) {
	cfg, err := s.configRepo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return exportConfigToDTO(cfg), nil
}

func (s *exportService) List(ctx context.Context, workspaceID string) ([]*ExportConfigDTO, error) {
	cfgs, err := s.configRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(cfgs, exportConfigToDTO)
	return out, nil
}

func (s *exportService) ListByProject(ctx context.Context, workspaceID, projectID string) ([]*ExportConfigDTO, error) {
	cfgs, err := s.configRepo.ListByProject(ctx, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(cfgs, exportConfigToDTO)
	return out, nil
}

func (s *exportService) Update(
	ctx context.Context,
	workspaceID, id string,
	p UpdateExportConfigParams,
) (*ExportConfigDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	existing, err := s.configRepo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	enc := existing.DestConfigEnc
	if len(p.DestConfig) > 0 {
		stamped, stampErr := stampWorkspaceID(p.DestConfig, workspaceID)
		if stampErr != nil {
			return nil, fmt.Errorf("export: stamp workspace: %w", stampErr)
		}
		enc, err = ingress.EncryptConfig(s.appSecret, stamped)
		if err != nil {
			return nil, fmt.Errorf("export: encrypt dest config: %w", err)
		}
	}
	existing.Label = p.Label
	existing.DestType = p.DestType
	existing.DestConfigEnc = enc
	existing.Versions = p.Versions
	existing.IncludeVariants = p.IncludeVariants
	existing.ScheduleType = p.ScheduleType
	existing.QuietMinutes = p.QuietMinutes
	existing.Enabled = p.Enabled
	existing.UpdatedAt = time.Now()

	updated, err := s.configRepo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return exportConfigToDTO(updated), nil
}

func (s *exportService) Delete(ctx context.Context, workspaceID, id string) error {
	return s.configRepo.Delete(ctx, workspaceID, id)
}

func (s *exportService) ValidateDestination(ctx context.Context, workspaceID, configID string) error {
	cfg, err := s.configRepo.Get(ctx, workspaceID, configID)
	if err != nil {
		return err
	}
	configJSON, err := ingress.DecryptConfig(s.appSecret, cfg.DestConfigEnc)
	if err != nil {
		slog.ErrorContext(ctx, "export: decrypt dest config", "error", err)
		return fmt.Errorf("export: decrypt dest config: %w", err)
	}
	dest, err := export.NewDestination(cfg.DestType, configJSON)
	if err != nil {
		slog.ErrorContext(ctx, "export: decrypt new dest", "error", err)
		return fmt.Errorf("%w: %s", apperr.ErrInvalidInput, err.Error())
	}
	return dest.Validate(ctx)
}

func (s *exportService) ValidateDestinationConfig(
	ctx context.Context,
	workspaceID, destType string,
	destConfig json.RawMessage,
) error {
	stamped, err := stampWorkspaceID(destConfig, workspaceID)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrInvalidInput, err.Error())
	}
	dest, err := export.NewDestination(destType, stamped)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrInvalidInput, err.Error())
	}
	return dest.Validate(ctx)
}

func (s *exportService) TriggerManual(
	ctx context.Context,
	workspaceID, userID, configID string,
) (*ExportRunDTO, error) {
	cfg, err := s.configRepo.Get(ctx, workspaceID, configID)
	if err != nil {
		return nil, err
	}
	run := repository.ExportRun{
		ID:             uuid.NewString(),
		ExportConfigID: cfg.ID,
		WorkspaceID:    workspaceID,
		TriggeredBy:    &userID,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	created, err := s.runRepo.Create(ctx, run)
	if err != nil {
		return nil, err
	}
	payload := fmt.Sprintf(`{"export_config_id":%q,"export_run_id":%q}`, cfg.ID, created.ID)
	if _, enqErr := s.q.Enqueue(ctx, workspaceID, queue.JobTypeExportRun, payload); enqErr != nil {
		return nil, fmt.Errorf("export: enqueue job: %w", enqErr)
	}
	return exportRunToDTO(created), nil
}

func (s *exportService) GetRun(ctx context.Context, workspaceID, runID string) (*ExportRunDTO, error) {
	run, err := s.runRepo.Get(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	return exportRunToDTO(run), nil
}

func (s *exportService) ListRuns(
	ctx context.Context,
	workspaceID, configID string,
	limit, offset int,
) ([]*ExportRunDTO, error) {
	if _, err := s.configRepo.Get(ctx, workspaceID, configID); err != nil {
		return nil, err
	}
	runs, err := s.runRepo.List(ctx, configID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := mapDTOs(runs, exportRunToDTO)
	return out, nil
}

// ExecuteRun carries out the full lifecycle of an export run: load config & run,
// build the destination, stream progress, and record the final result.
func (s *exportService) ExecuteRun(ctx context.Context, workspaceID, configID, runID string) error {
	cfg, err := s.configRepo.Get(ctx, workspaceID, configID)
	if err != nil {
		return fmt.Errorf("export: load config %s: %w", configID, err)
	}

	configJSON, err := ingress.DecryptConfig(s.appSecret, cfg.DestConfigEnc)
	if err != nil {
		return fmt.Errorf("export: decrypt config: %w", err)
	}

	dest, err := export.NewDestination(cfg.DestType, configJSON)
	if err != nil {
		return fmt.Errorf("export: build destination: %w", err)
	}

	run, err := s.runRepo.Get(ctx, workspaceID, runID)
	if err != nil {
		return fmt.Errorf("export: load run %s: %w", runID, err)
	}

	if startErr := s.runRepo.Start(ctx, run.ID); startErr != nil {
		slog.WarnContext(ctx, "export: mark run started", "error", startErr)
	}

	project, err := s.projects.GetByID(ctx, cfg.WorkspaceID, cfg.ProjectID)
	if err != nil {
		s.failRun(ctx, run.ID, cfg.ID, fmt.Sprintf("load project: %s", err))
		return fmt.Errorf("export: load project: %w", err)
	}

	result, buildErr := export.Build(ctx, export.BuildParams{
		Config:  cfg,
		Run:     run,
		Dest:    dest,
		Storage: s.storage,
		Queries: s.queries,
		SQLite:  s.sqlDB,
		Project: projectToDbgen(project),
		OnProgress: func(prog export.BuildProgress) {
			_ = s.runRepo.UpdateProgress(ctx, run.ID, repository.ExportProgress{
				AssetsExported: prog.AssetsExported,
				AssetsSkipped:  prog.AssetsSkipped,
				BytesWritten:   prog.BytesWritten,
			})
		},
	})

	if buildErr != nil {
		s.failRun(ctx, run.ID, cfg.ID, buildErr.Error())
		return fmt.Errorf("export: build: %w", buildErr)
	}

	configStatus := "ok"
	if result.AssetsExported > 0 && result.AssetsSkipped > 0 {
		configStatus = "partial"
	}

	if finishErr := s.runRepo.Finish(ctx, run.ID, repository.ExportFinish{
		Status:         "done",
		AssetsTotal:    result.AssetsTotal,
		AssetsExported: result.AssetsExported,
		AssetsSkipped:  result.AssetsSkipped,
		BytesWritten:   result.BytesWritten,
	}); finishErr != nil {
		slog.WarnContext(ctx, "export: finish run", "error", finishErr)
	}

	if setErr := s.configRepo.SetLastRun(ctx, cfg.ID, repository.ExportRunResult{
		LastRunAt:     time.Now(),
		LastRunStatus: configStatus,
	}); setErr != nil {
		slog.WarnContext(ctx, "export: set last run", "error", setErr)
	}

	return nil
}

func (s *exportService) failRun(ctx context.Context, runID, configID, errMsg string) {
	_ = s.runRepo.Finish(ctx, runID, repository.ExportFinish{
		Status: "failed",
		Error:  &errMsg,
	})
	_ = s.configRepo.SetLastRun(ctx, configID, repository.ExportRunResult{
		LastRunAt:     time.Now(),
		LastRunStatus: "failed",
		LastError:     &errMsg,
	})
}

// Scheduler-facing methods used via the local exportService interface in the jobs package.

func (s *exportService) ListDueConfigs(ctx context.Context) ([]repository.ExportConfig, error) {
	return s.configRepo.ListDue(ctx)
}

func (s *exportService) CreateRun(ctx context.Context, run repository.ExportRun) (repository.ExportRun, error) {
	return s.runRepo.Create(ctx, run)
}

func (s *exportService) SetConfigLastRun(ctx context.Context, configID string, p repository.ExportRunResult) error {
	return s.configRepo.SetLastRun(ctx, configID, p)
}

// projectToDbgen adapts a repository.Project into the raw dbgen.Project shape
// expected by internal/export.BuildParams, which queries assets/variants directly.
func projectToDbgen(p repository.Project) dbgen.Project {
	return dbgen.Project{
		ID:             p.ID,
		WorkspaceID:    p.WorkspaceID,
		Name:           p.Name,
		Description:    p.Description,
		Color:          p.Color,
		CoverAssetID:   p.CoverAssetID,
		CoverVersionID: p.CoverVersionID,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func exportConfigToDTO(c repository.ExportConfig) *ExportConfigDTO {
	return &ExportConfigDTO{
		ID:              c.ID,
		WorkspaceID:     c.WorkspaceID,
		ProjectID:       c.ProjectID,
		Label:           c.Label,
		DestType:        c.DestType,
		Versions:        c.Versions,
		IncludeVariants: c.IncludeVariants,
		ScheduleType:    c.ScheduleType,
		QuietMinutes:    c.QuietMinutes,
		Enabled:         c.Enabled,
		LastRunAt:       c.LastRunAt,
		LastRunStatus:   c.LastRunStatus,
		LastError:       c.LastError,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

func exportRunToDTO(r repository.ExportRun) *ExportRunDTO {
	return &ExportRunDTO{
		ID:             r.ID,
		ExportConfigID: r.ExportConfigID,
		TriggeredBy:    r.TriggeredBy,
		Status:         r.Status,
		AssetsTotal:    r.AssetsTotal,
		AssetsExported: r.AssetsExported,
		AssetsSkipped:  r.AssetsSkipped,
		BytesWritten:   r.BytesWritten,
		Error:          r.Error,
		StartedAt:      r.StartedAt,
		CompletedAt:    r.CompletedAt,
		CreatedAt:      r.CreatedAt,
	}
}
