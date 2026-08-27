package workflow_test

import (
	"errors"
	"testing"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/workflow"
)

func TestRetryPolicyShouldRetry(t *testing.T) {
	p := workflow.DefaultRetryPolicy()
	if !p.ShouldRetry(1, errors.New("timeout")) {
		t.Fatal("expected retry for transient error")
	}
	if p.ShouldRetry(1, apperr.ErrInvalidInput) {
		t.Fatal("expected invalid input to be non-retryable")
	}
}

func TestRetryPolicyWaitFor(t *testing.T) {
	p := workflow.RetryPolicy{MaxAttempts: 5, InitialWait: 2 * time.Second, Multiplier: 2, MaxWait: 30 * time.Second}
	if got := p.WaitFor(3); got != 8*time.Second {
		t.Fatalf("WaitFor(3) = %v, want 8s", got)
	}
}
