package api

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/service"
)

// ctxCheckingProjectService is a minimal service.ProjectService stub that
// returns ctx.Err() when the passed context is already done, mimicking how a
// real context-aware repository call (e.g. database/sql) behaves.
type ctxCheckingProjectService struct {
	service.ProjectService
}

func (ctxCheckingProjectService) Get(ctx context.Context, _, _ string) (*service.ProjectDTO, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &service.ProjectDTO{}, nil
}

func TestValidateShareTarget_CancelledContextReturnsPromptly(t *testing.T) {
	s := &Server{projects: ctxCheckingProjectService{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.validateShareTarget(ctx, "ws_1", apiTargetProject, "prj_1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
