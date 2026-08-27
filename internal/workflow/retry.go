package workflow

import (
	"errors"
	"math"
	"time"

	"damask/server/internal/apperr"
)

type RetryPolicy struct {
	MaxAttempts int
	InitialWait time.Duration
	Multiplier  float64
	MaxWait     time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,                //nolint:mnd // default to 3 attempts
		InitialWait: 2 * time.Second,  //nolint:mnd // default to 2 seconds
		Multiplier:  2,                //nolint:mnd // default to doubling the wait time each attempt
		MaxWait:     30 * time.Second, //nolint:mnd // default to max 30 seconds wait
	}
}

func (p RetryPolicy) ShouldRetry(attempt int, err error) bool {
	if attempt >= p.MaxAttempts {
		return false
	}
	if errors.Is(err, apperr.ErrNotFound) || errors.Is(err, apperr.ErrForbidden) ||
		errors.Is(err, apperr.ErrInvalidInput) {
		return false
	}
	return true
}

func (p RetryPolicy) WaitFor(attempt int) time.Duration {
	wait := time.Duration(float64(p.InitialWait) * math.Pow(p.Multiplier, float64(attempt-1)))
	if wait > p.MaxWait {
		return p.MaxWait
	}
	return wait
}
