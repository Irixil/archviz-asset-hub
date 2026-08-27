package queue

// This test proves that the per-job-type timeout wired up in processNext
// (jobCtx, cancel := context.WithTimeout(jobCtx, TimeoutFor(job.Type))) is the
// same ctx a job handler passes down to a storage.Storage call — i.e. a
// storage operation inside a job handler respects the job timeout from
// ROADMAP.70, now that storage.Storage methods take ctx (ROADMAP.73 ST-1).

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	dbgen "damask/server/internal/db/gen"
)

// blockingStorage implements the same Get(ctx, key) shape as
// storage.Storage.Get and blocks until ctx is done, simulating a slow/hanging
// backend (e.g. a dead SFTP server). It never completes on its own — the
// only way Get returns is via ctx cancellation, so this proves the job
// handler's storage call actually observes the job timeout deadline rather
// than some detached context.
type blockingStorage struct{}

func (blockingStorage) Get(ctx context.Context, _ string) (io.ReadCloser, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestJobTimeout_PropagatesToStorageCallInHandler(t *testing.T) {
	q, _ := newRetryQueue(t)

	const jobType = "storage_hang_test"
	timeoutByType[jobType] = 50 * time.Millisecond
	t.Cleanup(func() { delete(timeoutByType, jobType) })

	stor := blockingStorage{}

	var handlerErr error
	q.Register(jobType, func(ctx context.Context, _ dbgen.Job) error {
		// A representative job handler storage call (mirrors
		// internal/jobs/variant_thumbnail.go's s.storage.Get(ctx, ...) shape):
		// the ctx passed to storage.Get is exactly the job's timeout-bound ctx.
		_, handlerErr = stor.Get(ctx, "some/key")
		return handlerErr
	})

	id := enqueueID(t, q, jobType)

	start := time.Now()
	q.processNext(context.Background())
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("processNext blocked %v; storage call did not observe the job timeout deadline", elapsed)
	}
	if !errors.Is(handlerErr, context.DeadlineExceeded) {
		t.Fatalf(
			"storage.Get error = %v, want context.DeadlineExceeded (job timeout ctx should reach the storage call)",
			handlerErr,
		)
	}

	job := getJob(t, q, id)
	if job.Status != "pending" {
		t.Fatalf("job status = %q, want pending (timeout is transient, retried)", job.Status)
	}
}
