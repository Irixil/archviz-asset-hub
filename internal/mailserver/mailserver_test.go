package mailserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

const rawEmail = "From: sender@example.com\r\n" +
	"To: hook@example.com\r\n" +
	"Subject: Test Subject\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: text/plain; charset=utf-8\r\n" +
	"\r\n" +
	"Hello, world!\r\n"

func TestSession_HookTriggeredOnMatchingRecipient(t *testing.T) {
	t.Parallel()
	hook := Hook{
		Address:     "hook@example.com",
		Name:        "test-hook",
		WorkspaceID: "ws-1",
	}
	session := &Session{hooks: []Hook{hook}}

	if err := session.Mail("sender@example.com", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}

	if err := session.Rcpt("hook@example.com", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}

	if err := session.Data(strings.NewReader(rawEmail)); err != nil {
		t.Fatalf("Data: %v", err)
	}
}

func TestMailServerImpl_AddHookAndNewSession(t *testing.T) {
	t.Parallel()
	hook := Hook{
		Address:     "hook@example.com",
		Name:        "test-hook",
		WorkspaceID: "ws-1",
	}

	impl := &Impl{}
	impl.AddHook(hook)

	session, err := impl.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	s, ok := session.(*Session)
	if !ok {
		t.Fatalf("expected *Session, got %T", session)
	}
	if len(s.hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(s.hooks))
	}
	if s.hooks[0].Address != hook.Address {
		t.Errorf("hook address: got %q, want %q", s.hooks[0].Address, hook.Address)
	}
}

func buildEmailTo(recipient string) string {
	return "From: sender@example.com\r\n" +
		"To: " + recipient + "\r\n" +
		"Subject: Upload\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary42\"\r\n" +
		"\r\n" +
		"--boundary42\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"See attached.\r\n" +
		"--boundary42\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=\"photo.jpg\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"aGVsbG8=\r\n" +
		"--boundary42--\r\n"
}

func setupMailserverDB(t *testing.T) (*dbgen.Queries, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := dbpkg.Open(dbPath)
	queries, sqlDB := database.WQ, database.Writer
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()
	if _, err = sqlDB.ExecContext(
		ctx,
		`INSERT INTO users (id, email, password_hash, name, created_at) VALUES ('u1','u@x.com','h','U',datetime('now'))`,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err = sqlDB.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, created_at) VALUES ('ws1','Test',datetime('now'))`); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if _, err = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID: "src1", WorkspaceID: "ws1", CreatedBy: "u1",
		Type: "email_api", Label: "Mail drop", Config: "{}",
		PublicToken: "abc123", Enabled: 1, PollIntervalMin: 0,
	}); err != nil {
		t.Fatalf("create ingress source: %v", err)
	}
	return queries, sqlDB
}

func TestSession_EmailAttachmentEnqueuesIngestFetchJob(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := dbpkg.Open(dbPath)
	queries, sqlDB := database.WQ, database.Writer
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()

	// Insert a workspace and user to satisfy FK constraints.
	if _, err = sqlDB.ExecContext(
		ctx,
		`INSERT INTO users (id, email, password_hash, name, created_at) VALUES ('u1','u@x.com','h','U',datetime('now'))`,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err = sqlDB.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, created_at) VALUES ('ws1','Test',datetime('now'))`); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	const token = "abc123"
	if _, err = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID: "src1", WorkspaceID: "ws1", CreatedBy: "u1",
		Type: "email_api", Label: "Mail drop", Config: "{}",
		PublicToken: token, Enabled: 1, PollIntervalMin: 0,
	}); err != nil {
		t.Fatalf("create ingress source: %v", err)
	}

	q := queue.New(queries, 1)
	session := &Session{queries: queries, queue: q}

	if err = session.Rcpt(token+"@ingress.damask.studio", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	if err = session.Data(strings.NewReader(buildEmailTo("abc123@ingress.damask.studio"))); err != nil {
		t.Fatalf("Data: %v", err)
	}

	// One ingress_log entry should exist with status=pending.
	rows, err := sqlDB.QueryContext(
		ctx,
		`SELECT id, source_id, filename, status FROM ingress_log WHERE source_id = 'src1'`,
	)
	if err != nil {
		t.Fatalf("query ingress_log: %v", err)
	}
	defer rows.Close()
	var logID string
	var count int
	for rows.Next() {
		var srcID, filename, status string
		if err = rows.Scan(&logID, &srcID, &filename, &status); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if filename != "photo.jpg" {
			t.Errorf("filename: got %q, want %q", filename, "photo.jpg")
		}
		if status != "pending" {
			t.Errorf("status: got %q, want %q", status, "pending")
		}
		count++
	}
	if count != 1 {
		t.Fatalf("expected 1 ingress_log row, got %d", count)
	}

	// One pending ingest_fetch job should be enqueued.
	n, err := queries.CountPendingJobs(ctx)
	if err != nil {
		t.Fatalf("count pending jobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 pending job, got %d", n)
	}

	job, err := queries.ClaimNextJob(ctx, queue.NoExcludedTypes)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	if job.Type != queue.JobTypeIngestFetch {
		t.Errorf("job type: got %q, want %q", job.Type, queue.JobTypeIngestFetch)
	}

	var payload struct {
		SourceID    string `json:"source_id"`
		WorkspaceID string `json:"workspace_id"`
		LogEntryID  string `json:"log_entry_id"`
		Filename    string `json:"filename"`
		TmpPath     string `json:"tmp_path"`
	}
	if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.SourceID != "src1" {
		t.Errorf("payload source_id: got %q, want %q", payload.SourceID, "src1")
	}
	if payload.WorkspaceID != "ws1" {
		t.Errorf("payload workspace_id: got %q, want %q", payload.WorkspaceID, "ws1")
	}
	if payload.LogEntryID == "" {
		t.Error("payload log_entry_id should not be empty")
	}
	if payload.Filename != "photo.jpg" {
		t.Errorf("payload filename: got %q, want %q", payload.Filename, "photo.jpg")
	}
	if payload.TmpPath == "" {
		t.Error("payload tmp_path should not be empty")
	}
}

func TestSession_FolderRoutingViaSubaddress(t *testing.T) {
	t.Parallel()
	queries, sqlDB := setupMailserverDB(t)
	ctx := context.Background()

	if _, err := sqlDB.ExecContext(
		ctx,
		`INSERT INTO projects (id, workspace_id, name, created_at, updated_at) VALUES ('p1','ws1','Default',datetime('now'),datetime('now'))`,
	); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	// Insert a folder with slug "brand-assets" in ws1
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO folders (id, workspace_id, project_id, parent_id, name, slug, position, created_at)
		 VALUES ('f1','ws1','p1',NULL,'Brand Assets','brand-assets',0,datetime('now'))`); err != nil {
		t.Fatalf("insert folder: %v", err)
	}

	q := queue.New(queries, 1)
	session := &Session{queries: queries, queue: q}

	recipient := "abc123+brand-assets@ingress.damask.studio"
	if err := session.Rcpt(recipient, nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	if err := session.Data(strings.NewReader(buildEmailTo(recipient))); err != nil {
		t.Fatalf("Data: %v", err)
	}

	job, err := queries.ClaimNextJob(ctx, queue.NoExcludedTypes)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}

	var payload struct {
		OverrideFolderID string `json:"override_folder_id"`
	}
	if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OverrideFolderID != "f1" {
		t.Errorf("override_folder_id = %q, want f1", payload.OverrideFolderID)
	}
}

func TestSession_FolderRouting_UnknownTagFallsBack(t *testing.T) {
	t.Parallel()
	queries, _ := setupMailserverDB(t)
	ctx := context.Background()

	// No folder inserted — tag won't match anything
	q := queue.New(queries, 1)
	session := &Session{queries: queries, queue: q}

	recipient := "abc123+nonexistent@ingress.damask.studio"
	if err := session.Rcpt(recipient, nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	if err := session.Data(strings.NewReader(buildEmailTo(recipient))); err != nil {
		t.Fatalf("Data: %v", err)
	}

	// Job still enqueued despite unknown tag
	n, err := queries.CountPendingJobs(ctx)
	if err != nil {
		t.Fatalf("count pending jobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 pending job, got %d", n)
	}

	job, err := queries.ClaimNextJob(ctx, queue.NoExcludedTypes)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	var payload struct {
		OverrideFolderID string `json:"override_folder_id"`
	}
	if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OverrideFolderID != "" {
		t.Errorf("override_folder_id = %q, want empty (fallback)", payload.OverrideFolderID)
	}
}

func TestSession_DataNotTriggeredForNonMatchingHook(t *testing.T) {
	t.Parallel()
	// Hook address differs from the accepted recipient — Data should not call Trigger.
	// We register two hooks: only one matches the recipient.
	hookMatch := Hook{Address: "hook@example.com", Name: "match", WorkspaceID: "ws-1"}
	hookOther := Hook{Address: "other@example.com", Name: "other", WorkspaceID: "ws-2"}

	session := &Session{hooks: []Hook{hookOther, hookMatch}}

	if err := session.Mail("sender@example.com", nil); err != nil {
		t.Fatalf("Mail: %v", err)
	}
	if err := session.Rcpt("hook@example.com", nil); err != nil {
		t.Fatalf("Rcpt: %v", err)
	}
	// Data should succeed — only hookMatch is triggered, hookOther is skipped.
	if err := session.Data(strings.NewReader(rawEmail)); err != nil {
		t.Fatalf("Data: %v", err)
	}
}
