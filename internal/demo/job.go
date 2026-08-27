//go:build demo

package demo

import (
	"context"
	"log/slog"
	"math/rand"
	"time"
)

// resetting is a package-level flag used to signal that a reset is in progress.
// It is set by StartResetLoop and read by IsResetting.
var resettingCh = make(chan struct{}, 1)

// IsResetting reports whether a demo reset is currently in progress.
// Handlers use this to return 503 during the reset window.
func (s *Seeder) IsResetting() bool {
	select {
	case <-resettingCh:
		resettingCh <- struct{}{} // put it back
		return true
	default:
		return false
	}
}

// StartResetLoop runs the demo reset on a ticker in a background goroutine.
// Call this once from main.go when DEMO_MODE=true.
// A random jitter of 0–15 minutes is applied to the first tick to avoid
// top-of-hour resets.
func (s *Seeder) StartResetLoop(ctx context.Context) {
	interval := time.Duration(s.cfg.ResetIntervalHours) * time.Hour

	// Apply jitter: wait 0–15 min before the first scheduled reset
	jitter := time.Duration(rand.Intn(15)) * time.Minute //nolint:gosec

	go func() {
		slog.InfoContext(ctx, "demo: reset loop started", "interval", interval, "jitter", jitter)

		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runReset(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// runReset wipes and reseeds the demo workspace, with retry on failure.
func (s *Seeder) runReset(ctx context.Context) {
	slog.InfoContext(ctx, "demo: reset started")
	start := time.Now()

	// Signal that a reset is in progress
	select {
	case resettingCh <- struct{}{}:
		defer func() { <-resettingCh }()
	default:
		// Already resetting
		return
	}

	if err := s.Wipe(ctx); err != nil {
		slog.ErrorContext(ctx, "demo: reset failed", "step", "wipe", "error", err)
		s.scheduleRetry(ctx, 30*time.Minute)
		return
	}

	if err := s.Seed(ctx); err != nil {
		slog.ErrorContext(ctx, "demo: reset failed", "step", "seed", "error", err)
		s.scheduleRetry(ctx, 30*time.Minute)
		return
	}

	s.lastResetAt = time.Now()
	slog.InfoContext(ctx, "demo: reset complete", "duration_ms", time.Since(start).Milliseconds())
}

// scheduleRetry schedules a single retry after the given delay.
func (s *Seeder) scheduleRetry(ctx context.Context, delay time.Duration) {
	slog.InfoContext(ctx, "demo: retry scheduled", "in", delay)
	go func() {
		select {
		case <-time.After(delay):
			slog.InfoContext(ctx, "demo: reset started (retry)")
			start := time.Now()

			// Check if reset is already in progress
			select {
			case resettingCh <- struct{}{}:
				defer func() { <-resettingCh }()
			default:
				return
			}

			if err := s.Wipe(ctx); err != nil {
				slog.ErrorContext(ctx, "demo: retry failed", "step", "wipe", "error", err)
				return
			}
			if err := s.Seed(ctx); err != nil {
				slog.ErrorContext(ctx, "demo: retry failed", "step", "seed", "error", err)
				return
			}
			slog.InfoContext(ctx, "demo: retry complete", "duration_ms", time.Since(start).Milliseconds())
		case <-ctx.Done():
		}
	}()
}
