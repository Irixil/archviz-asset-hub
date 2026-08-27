package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"damask/server/internal/ai"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/workflow"
)

func noopKeyResolver(context.Context, string, string) (string, ai.KeySource, error) {
	return "", "", nil
}

// validJobDeps returns a Deps with every field required by Deps.validate
// populated, so each test case can zero out exactly one required field.
func validJobDeps() Deps {
	return Deps{
		Queries:          &dbgen.Queries{},
		SQLDB:            &sql.DB{}, // validate() only checks non-nil; never used to make a real query here
		Queue:            fakeJobQueue{},
		AIAPIKeyResolver: ai.KeyResolver(noopKeyResolver),
		WorkflowExec:     workflow.NewExecutor(workflow.Deps{}),
	}
}

type fakeJobQueue struct{}

func (fakeJobQueue) Register(string, queue.HandlerFunc) {}
func (fakeJobQueue) Start(context.Context)              {}
func (fakeJobQueue) Stop()                              {}
func (fakeJobQueue) Enqueue(context.Context, string, string, string) (dbgen.Job, error) {
	return dbgen.Job{}, nil
}

func TestDeps_Validate_MissingQueries(t *testing.T) {
	d := validJobDeps()
	d.Queries = nil
	assertMissingField(t, d, "Queries")
}

func TestDeps_Validate_MissingSQLDB(t *testing.T) {
	d := validJobDeps()
	d.SQLDB = nil
	assertMissingField(t, d, "SQLDB")
}

func TestDeps_Validate_MissingQueue(t *testing.T) {
	d := validJobDeps()
	d.Queue = nil
	assertMissingField(t, d, "Queue")
}

func TestDeps_Validate_MissingAIAPIKeyResolver(t *testing.T) {
	d := validJobDeps()
	d.AIAPIKeyResolver = nil
	assertMissingField(t, d, "AIAPIKeyResolver")
}

func TestDeps_Validate_MissingWorkflowExec(t *testing.T) {
	d := validJobDeps()
	d.WorkflowExec = nil
	assertMissingField(t, d, "WorkflowExec")
}

func assertMissingField(t *testing.T, d Deps, field string) {
	t.Helper()
	err := d.validate()
	if err == nil {
		t.Fatalf("expected validate() to error when %s is nil", field)
	}
	if !strings.Contains(err.Error(), field) {
		t.Fatalf("expected validate() error to name field %q, got: %v", field, err)
	}
}

func TestNewJobServer_PanicsOnMissingRequiredDep(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected NewJobServer to panic on a Deps missing a required field")
		}
		if !strings.Contains(fmt.Sprint(r), "WorkflowExec") {
			t.Fatalf("expected panic to name the missing field, got: %v", r)
		}
	}()
	d := validJobDeps()
	d.WorkflowExec = nil
	NewJobServer(d)
}
