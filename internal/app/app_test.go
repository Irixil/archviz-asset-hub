package app_test

import (
	"net/url"
	"strings"
	"testing"

	"damask/server/internal/app"
	"damask/server/internal/config"
	dbpkg "damask/server/internal/db"
	"damask/server/internal/events"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

// buildArgs holds every argument to app.Build so tests can zero out one at
// a time to exercise the fail-fast validation.
type buildArgs struct {
	cfg    *config.Config
	db     *dbpkg.DB
	stor   storage.Storage
	hub    events.EventHub
	q      queue.JobQueue
	mailer mail.Mailer
	trf    transform.Transformer
	tmb    transform.Thumbnailer
}

func validBuildArgs(t *testing.T) buildArgs {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	trf := transform.NewTransformer()
	baseURL, _ := url.Parse("http://localhost")
	return buildArgs{
		cfg:    &config.Config{AppSecret: "test-secret", BaseURL: baseURL},
		db:     database,
		stor:   stor,
		hub:    events.NewEventHub(),
		q:      queue.New(database.WQ, 1),
		mailer: mail.NewMailer(&mail.Config{}),
		trf:    trf,
		tmb:    transform.NewThumbnailer(trf),
	}
}

func (a buildArgs) build() (*app.Deps, error) {
	return app.Build(a.cfg, a.db, a.stor, a.hub, a.q, a.mailer, a.trf, a.tmb)
}

func TestBuild_Succeeds(t *testing.T) {
	a := validBuildArgs(t)
	deps, err := a.build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if deps.Tags == nil || deps.Variants == nil || deps.WorkflowExec == nil {
		t.Fatal("Build returned Deps with nil core services")
	}
}

func TestBuild_MissingConfig_Errors(t *testing.T) {
	a := validBuildArgs(t)
	a.cfg = nil
	assertBuildErrors(t, a)
}

func TestBuild_MissingDB_Errors(t *testing.T) {
	a := validBuildArgs(t)
	a.db = nil
	assertBuildErrors(t, a)
}

func TestBuild_MissingStorage_Errors(t *testing.T) {
	a := validBuildArgs(t)
	a.stor = nil
	assertBuildErrors(t, a)
}

func TestBuild_MissingHub_Errors(t *testing.T) {
	a := validBuildArgs(t)
	a.hub = nil
	assertBuildErrors(t, a)
}

func TestBuild_MissingQueue_Errors(t *testing.T) {
	a := validBuildArgs(t)
	a.q = nil
	assertBuildErrors(t, a)
}

func assertBuildErrors(t *testing.T, a buildArgs) {
	t.Helper()
	_, err := a.build()
	if err == nil {
		t.Fatal("expected Build to error on a missing required dependency")
	}
	if !strings.Contains(err.Error(), "app: Build requires") {
		t.Fatalf("unexpected error: %v", err)
	}
}
