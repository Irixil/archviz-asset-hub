package queue

// NOTE: some tests below mutate the package-level timeoutByType map (with
// t.Cleanup restores). Keep this package's tests serial — no t.Parallel().

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
)

func getJob(t *testing.T, q *Queue, id string) dbgen.Job {
	t.Helper()
	job, err := q.queries.GetJobByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	return job
}

func enqueueID(t *testing.T, q *Queue, jobType string) string {
	t.Helper()
	job, err := q.Enqueue(context.Background(), "ws_1", jobType, "{}")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	return job.ID
}

// clearRunAfter makes a requeued job immediately claimable again.
func clearRunAfter(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE jobs SET run_after = datetime('now', '-1 minute') WHERE id = ?`, id); err != nil {
		t.Fatalf("clear run_after: %v", err)
	}
}

func newRetryQueue(t *testing.T) (*Queue, *sql.DB) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// A single conn: the in-memory DB vanishes per-connection otherwise, and
	// it serializes concurrent worker access like the real single-writer DB.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = dbpkg.RunMigrations(sqlDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err = sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES ('ws_1', 'Test')`); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	return New(dbgen.New(sqlDB), 4), sqlDB
}

func TestRetry_TransientFailureRequeuesWithBackoff(t *testing.T) {
	q, _ := newRetryQueue(t)
	q.Register("flaky", func(context.Context, dbgen.Job) error {
		return errors.New("smtp hiccup")
	})
	id := enqueueID(t, q, "flaky")

	q.processNext(context.Background())

	job := getJob(t, q, id)
	if job.Status != "pending" {
		t.Fatalf("status = %q, want pending", job.Status)
	}
	if job.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", job.Attempts)
	}
	if job.Error == nil || *job.Error != "smtp hiccup" {
		t.Fatalf("error = %v, want smtp hiccup", job.Error)
	}
	if job.RunAfter == nil || !job.RunAfter.After(time.Now().UTC().Add(20*time.Second)) {
		t.Fatalf("run_after = %v, want ≥ ~30s in the future", job.RunAfter)
	}
}

func TestRetry_ExhaustedAttemptsFailTerminally(t *testing.T) {
	q, db := newRetryQueue(t)
	q.Register("flaky", func(context.Context, dbgen.Job) error {
		return errors.New("still broken")
	})
	id := enqueueID(t, q, "flaky")

	maxAtt := MaxAttemptsFor("flaky") // default budget
	for range maxAtt {
		q.processNext(context.Background())
		clearRunAfter(t, db, id)
	}

	job := getJob(t, q, id)
	if job.Status != "failed" {
		t.Fatalf("status = %q after %d attempts, want failed", job.Status, maxAtt)
	}
	if job.Attempts != maxAtt {
		t.Fatalf("attempts = %d, want %d", job.Attempts, maxAtt)
	}
	if job.Error == nil || *job.Error != "still broken" {
		t.Fatalf("error = %v, want preserved message", job.Error)
	}
}

func TestRetry_PermanentErrorFailsOnFirstAttempt(t *testing.T) {
	q, _ := newRetryQueue(t)
	q.Register("invalid", func(context.Context, dbgen.Job) error {
		return Permanent(errors.New("bad payload"))
	})
	id := enqueueID(t, q, "invalid")

	q.processNext(context.Background())

	job := getJob(t, q, id)
	if job.Status != "failed" {
		t.Fatalf("status = %q, want failed", job.Status)
	}
	if job.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", job.Attempts)
	}
}

func TestRetry_PanicRequeuesLikeTransient(t *testing.T) {
	q, _ := newRetryQueue(t)
	q.Register("panics", func(context.Context, dbgen.Job) error {
		panic("boom")
	})
	id := enqueueID(t, q, "panics")

	q.processNext(context.Background())

	job := getJob(t, q, id)
	if job.Status != "pending" {
		t.Fatalf("status = %q, want pending", job.Status)
	}
	if job.RunAfter == nil {
		t.Fatal("run_after not set")
	}
}

func TestRetry_PreservesWorkspaceIDAcrossRequeues(t *testing.T) {
	q, db := newRetryQueue(t)
	q.Register("flaky", func(context.Context, dbgen.Job) error {
		return errors.New("nope")
	})
	id := enqueueID(t, q, "flaky")

	q.processNext(context.Background())
	clearRunAfter(t, db, id)
	q.processNext(context.Background())

	job := getJob(t, q, id)
	if job.WorkspaceID != "ws_1" {
		t.Fatalf("workspace_id = %q, want ws_1", job.WorkspaceID)
	}
}

func TestClaim_SkipsFutureRunAfter(t *testing.T) {
	q, db := newRetryQueue(t)
	id := enqueueID(t, q, "whatever")
	if _, err := db.Exec(`UPDATE jobs SET run_after = datetime('now', '+10 minutes') WHERE id = ?`, id); err != nil {
		t.Fatalf("set run_after: %v", err)
	}

	if _, err := q.queries.ClaimNextJob(context.Background(), NoExcludedTypes); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim = %v, want ErrNoRows", err)
	}

	clearRunAfter(t, db, id)
	job, err := q.queries.ClaimNextJob(context.Background(), NoExcludedTypes)
	if err != nil {
		t.Fatalf("claim after run_after elapsed: %v", err)
	}
	if job.ID != id {
		t.Fatalf("claimed %q, want %q", job.ID, id)
	}
}

func TestClaim_SkipsRunAfterWrittenByRetryPath(t *testing.T) {
	// End-to-end: the *driver-formatted* run_after written by FailJobRetry
	// must be skipped by the datetime() comparison in ClaimNextJob.
	q, _ := newRetryQueue(t)
	q.Register("flaky", func(context.Context, dbgen.Job) error {
		return errors.New("transient")
	})
	enqueueID(t, q, "flaky")

	q.processNext(context.Background())

	if _, err := q.queries.ClaimNextJob(context.Background(), NoExcludedTypes); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim = %v, want ErrNoRows (run_after in the future)", err)
	}
}

func TestTimeout_HungHandlerFailsAtDeadlineThenWorkerContinues(t *testing.T) {
	q, _ := newRetryQueue(t)

	timeoutByType["hang_test"] = 50 * time.Millisecond
	t.Cleanup(func() { delete(timeoutByType, "hang_test") })

	q.Register("hang_test", func(ctx context.Context, _ dbgen.Job) error {
		<-ctx.Done()
		return ctx.Err()
	})
	var ran bool
	q.Register("after", func(context.Context, dbgen.Job) error {
		ran = true
		return nil
	})

	hungID := enqueueID(t, q, "hang_test")
	afterID := enqueueID(t, q, "after")

	start := time.Now()
	q.processNext(context.Background()) // hung job, returns at deadline
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("processNext blocked %v, deadline not applied", elapsed)
	}
	q.processNext(context.Background()) // next job

	hung := getJob(t, q, hungID)
	if hung.Status != "pending" {
		t.Fatalf("hung job status = %q, want pending (timeout is transient)", hung.Status)
	}
	if !ran {
		t.Fatal("worker did not claim the next job after the timeout")
	}
	if after := getJob(t, q, afterID); after.Status != "done" {
		t.Fatalf("after job status = %q, want done", after.Status)
	}
}

func TestSweep_RequeuesJobStuckPastTypeTimeout(t *testing.T) {
	q, db := newRetryQueue(t)

	stuckID := enqueueID(t, q, "some_default_type")
	freshID := enqueueID(t, q, "some_default_type")
	// Simulate a worker that died mid-processing long ago.
	if _, err := db.Exec(
		`UPDATE jobs SET status = 'processing', updated_at = datetime('now', '-1 hour') WHERE id = ?`, stuckID,
	); err != nil {
		t.Fatalf("mark stuck: %v", err)
	}
	// A job legitimately processing right now must not be touched.
	if _, err := db.Exec(`UPDATE jobs SET status = 'processing' WHERE id = ?`, freshID); err != nil {
		t.Fatalf("mark fresh: %v", err)
	}

	q.SweepStalledJobs(context.Background())

	if job := getJob(t, q, stuckID); job.Status != "pending" {
		t.Fatalf("stuck job status = %q, want pending", job.Status)
	}
	if job := getJob(t, q, freshID); job.Status != "processing" {
		t.Fatalf("fresh job status = %q, want processing", job.Status)
	}
}

func TestSweep_RespectsLongerTranscodeTimeout(t *testing.T) {
	q, db := newRetryQueue(t)

	id := enqueueID(t, q, JobTypeVideoTranscode)
	// 20 min in: stalled for a default-timeout job, but transcode gets 30 min.
	if _, err := db.Exec(
		`UPDATE jobs SET status = 'processing', updated_at = datetime('now', '-20 minutes') WHERE id = ?`, id,
	); err != nil {
		t.Fatalf("mark processing: %v", err)
	}

	q.SweepStalledJobs(context.Background())

	if job := getJob(t, q, id); job.Status != "processing" {
		t.Fatalf("transcode job status = %q, want still processing", job.Status)
	}
}

func TestClaimSkip_EmailsRunWhileTranscodesSaturated(t *testing.T) {
	q, db := newRetryQueue(t)

	release := make(chan struct{})
	var transcodeStarted sync.WaitGroup
	transcodeStarted.Add(transcodeSemaphoreLimit)
	q.Register(JobTypeVideoTranscode, func(context.Context, dbgen.Job) error {
		transcodeStarted.Done()
		<-release
		return nil
	})
	var emailDone sync.WaitGroup
	emailDone.Add(2)
	q.Register("email_test", func(context.Context, dbgen.Job) error {
		emailDone.Done()
		return nil
	})

	// 4 transcodes enqueued before 2 emails (older created_at → claimed first).
	var transcodeIDs, emailIDs []string
	for range 4 {
		transcodeIDs = append(transcodeIDs, enqueueID(t, q, JobTypeVideoTranscode))
	}
	for range 2 {
		emailIDs = append(emailIDs, enqueueID(t, q, "email_test"))
	}
	if _, err := db.Exec(`UPDATE jobs SET created_at = datetime('now', '-1 minute') WHERE type = ?`,
		JobTypeVideoTranscode); err != nil {
		t.Fatalf("age transcodes: %v", err)
	}

	// Two "workers" grab the two transcode slots and block in the handler.
	var workers sync.WaitGroup
	for range transcodeSemaphoreLimit {
		workers.Go(func() {
			q.processNext(context.Background())
		})
	}
	transcodeStarted.Wait()

	// Two more workers: with both slots held they must skip the 2 remaining
	// transcodes and run the emails instead of blocking (head-of-line test).
	for range 2 {
		workers.Go(func() {
			q.processNext(context.Background())
		})
	}

	done := make(chan struct{})
	go func() { emailDone.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("emails starved behind saturated transcode slots")
	}

	close(release)
	workers.Wait()

	for _, id := range emailIDs {
		if job := getJob(t, q, id); job.Status != "done" {
			t.Fatalf("email job %s status = %q, want done", id, job.Status)
		}
	}
	// The 2 remaining transcodes are still pending, never claimed-and-held.
	var pending int
	if err := db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE type = ? AND status = 'pending'`,
		JobTypeVideoTranscode).Scan(&pending); err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 2 {
		t.Fatalf("pending transcodes = %d, want 2", pending)
	}
	_ = transcodeIDs
}

func TestClaimSkip_SemaphoreReleasedOnPanicAndTimeout(t *testing.T) {
	q, _ := newRetryQueue(t)

	timeoutByType[JobTypeCustomFFmpeg] = 50 * time.Millisecond
	t.Cleanup(func() { timeoutByType[JobTypeCustomFFmpeg] = transcodeJobTimeout })

	q.Register(JobTypeVideoTranscode, func(context.Context, dbgen.Job) error {
		panic("boom")
	})
	q.Register(JobTypeCustomFFmpeg, func(ctx context.Context, _ dbgen.Job) error {
		<-ctx.Done()
		return ctx.Err()
	})

	enqueueID(t, q, JobTypeVideoTranscode)
	enqueueID(t, q, JobTypeCustomFFmpeg)

	q.processNext(context.Background()) // panics
	q.processNext(context.Background()) // times out

	for _, g := range q.semGroups {
		if n := len(g.sem); n != 0 {
			t.Fatalf("semaphore for %v leaked %d slot(s)", g.types, n)
		}
	}
}

func TestSweep_ExhaustedAttemptsFailTerminally(t *testing.T) {
	q, db := newRetryQueue(t)

	overBudgetID := enqueueID(t, q, "some_default_type")
	underBudgetID := enqueueID(t, q, "some_default_type")
	// Both died mid-processing long ago; one already spent its attempt budget
	// (e.g. the handler OOM-kills the process every run).
	if _, err := db.Exec(
		`UPDATE jobs SET status = 'processing', attempts = ?, updated_at = datetime('now', '-1 hour') WHERE id = ?`,
		defaultMaxAttempts, overBudgetID,
	); err != nil {
		t.Fatalf("mark over budget: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE jobs SET status = 'processing', attempts = 1, updated_at = datetime('now', '-1 hour') WHERE id = ?`,
		underBudgetID,
	); err != nil {
		t.Fatalf("mark under budget: %v", err)
	}

	q.SweepStalledJobs(context.Background())

	over := getJob(t, q, overBudgetID)
	if over.Status != "failed" {
		t.Fatalf("over-budget job status = %q, want failed (no infinite requeue loop)", over.Status)
	}
	if over.Error == nil || *over.Error != "stalled: attempt budget exhausted" {
		t.Fatalf("over-budget job error = %v, want stalled message", over.Error)
	}
	if under := getJob(t, q, underBudgetID); under.Status != "pending" {
		t.Fatalf("under-budget job status = %q, want pending", under.Status)
	}
}

func TestSettle_StaleWorkerCannotClobberReclaimedJob(t *testing.T) {
	// A worker that outlives its timeout holds job.Attempts=N; once the sweep
	// requeues the job and another worker re-claims it (attempts=N+1), the
	// stale worker's settle writes must match zero rows.
	q, db := newRetryQueue(t)
	ctx := context.Background()

	id := enqueueID(t, q, "some_default_type")
	stale, err := q.queries.ClaimNextJob(ctx, NoExcludedTypes)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	// Sweep requeues the stalled job, a second worker re-claims it.
	if _, err = db.Exec(
		`UPDATE jobs SET updated_at = datetime('now', '-1 hour') WHERE id = ?`, id,
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	q.SweepStalledJobs(ctx)
	fresh, err := q.queries.ClaimNextJob(ctx, NoExcludedTypes)
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if fresh.Attempts != stale.Attempts+1 {
		t.Fatalf("re-claim attempts = %d, want %d", fresh.Attempts, stale.Attempts+1)
	}

	// Stale complete is a no-op.
	n, err := q.queries.CompleteJob(ctx, dbgen.CompleteJobParams{ID: id, Attempts: stale.Attempts})
	if err != nil || n != 0 {
		t.Fatalf("stale CompleteJob = (%d, %v), want (0, nil)", n, err)
	}
	// Stale retry-requeue is a no-op.
	errMsg := "late failure"
	runAfter := time.Now().UTC().Add(time.Minute)
	n, err = q.queries.FailJobRetry(ctx, dbgen.FailJobRetryParams{
		Error: &errMsg, RunAfter: &runAfter, ID: id, Attempts: stale.Attempts,
	})
	if err != nil || n != 0 {
		t.Fatalf("stale FailJobRetry = (%d, %v), want (0, nil)", n, err)
	}
	if job := getJob(t, q, id); job.Status != "processing" || job.Attempts != fresh.Attempts {
		t.Fatalf("job = (%q, %d), want (processing, %d) — stale writer clobbered state",
			job.Status, job.Attempts, fresh.Attempts)
	}

	// The current claimant settles normally.
	n, err = q.queries.CompleteJob(ctx, dbgen.CompleteJobParams{ID: id, Attempts: fresh.Attempts})
	if err != nil || n != 1 {
		t.Fatalf("fresh CompleteJob = (%d, %v), want (1, nil)", n, err)
	}
	if job := getJob(t, q, id); job.Status != "done" {
		t.Fatalf("status = %q, want done", job.Status)
	}
}
