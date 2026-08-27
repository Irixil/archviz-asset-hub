package jobs_test

import (
	"testing"
	"time"

	"damask/server/internal/jobs"
)

func TestNextRunAt(t *testing.T) {
	// 01:59 UTC — next run is 02:00 same day
	t1 := time.Date(2026, 4, 5, 1, 59, 0, 0, time.UTC)
	next1 := jobs.NextRunAt(t1)
	if next1.Hour() != 2 || next1.Day() != 5 {
		t.Errorf("expected 02:00 same day, got %v", next1)
	}

	// 02:01 UTC — next run is 02:00 next day
	t2 := time.Date(2026, 4, 5, 2, 1, 0, 0, time.UTC)
	next2 := jobs.NextRunAt(t2)
	if next2.Day() != 6 {
		t.Errorf("expected 02:00 next day, got %v", next2)
	}
}
