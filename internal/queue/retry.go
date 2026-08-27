package queue

import (
	"errors"
	"strings"
	"time"
)

// permanentError marks an error as non-retryable.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps err so the queue fails the job terminally instead of
// retrying it. Handlers should use it for errors that cannot succeed on a
// re-run: payload validation failures, 4xx provider responses, missing rows.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent reports whether err (or any error in its chain) was wrapped
// with Permanent.
func IsPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}

const (
	defaultMaxAttempts = 3
	// Local CPU-bound work (ffmpeg, thumbnails, image ops): a failure is
	// rarely transient, one retry is enough.
	localMaxAttempts = 2
	// AI/network-bound work: 429s and provider blips deserve more retries.
	networkMaxAttempts = 4

	retryBackoffBase = 30 * time.Second
	retryBackoffCap  = 15 * time.Minute

	defaultJobTimeout   = 10 * time.Minute
	networkJobTimeout   = 5 * time.Minute
	transcodeJobTimeout = 30 * time.Minute
)

// maxAttemptsByType holds per-type retry budgets; anything absent uses
// defaultMaxAttempts. Attempts classify by failure mode (network vs local
// CPU); timeoutByType below classifies by expected runtime — a type may
// legitimately appear in only one map.
var maxAttemptsByType = map[string]int64{
	// AI / network-bound jobs.
	JobTypeAIImageDescriptionTextTrack: networkMaxAttempts,
	JobTypeImageBgRemove:               networkMaxAttempts,
	JobTypeImageWithPrompt:             networkMaxAttempts,
	JobTypeImageSmartCrop:              networkMaxAttempts,
	JobTypeAutoTag:                     networkMaxAttempts,
	JobTypeIngestPoll:                  networkMaxAttempts,
	JobTypeIngestFetch:                 networkMaxAttempts,
	JobTypeExportRun:                   networkMaxAttempts,

	// Local CPU-bound jobs.
	JobTypeVersionThumbnail:  localMaxAttempts,
	JobTypeVariantThumbnail:  localMaxAttempts,
	JobTypeVideoCaptureImage: localMaxAttempts,
	JobTypeVideoTranscode:    localMaxAttempts,
	JobTypeVideoWatermark:    localMaxAttempts,
	JobTypeImageResize:       localMaxAttempts,
	JobTypeImageConvert:      localMaxAttempts,
	JobTypeImageCrop:         localMaxAttempts,
	JobTypeImageWatermark:    localMaxAttempts,
	JobTypeExtractAudio:      localMaxAttempts,
	JobTypeTranscodeAudio:    localMaxAttempts,
	JobTypeNormalizeAudio:    localMaxAttempts,
	JobTypeCustomFFmpeg:      localMaxAttempts,
	JobTypeStackMerge:        localMaxAttempts,
	JobTypeOCRTextTrack:      localMaxAttempts,
}

// timeoutByType bounds handler execution per job type; anything absent uses
// defaultJobTimeout.
var timeoutByType = map[string]time.Duration{
	// AI / network-bound jobs.
	JobTypeAIImageDescriptionTextTrack: networkJobTimeout,
	JobTypeImageBgRemove:               networkJobTimeout,
	JobTypeImageWithPrompt:             networkJobTimeout,
	JobTypeImageSmartCrop:              networkJobTimeout,
	JobTypeAutoTag:                     networkJobTimeout,
	JobTypeIngestPoll:                  networkJobTimeout,
	// Network-bound like poll, but fetch downloads whole assets — keep the
	// longer default so large files on slow sources don't time out.
	JobTypeIngestFetch: defaultJobTimeout,

	// Long-running transcode/merge work.
	JobTypeVideoTranscode:  transcodeJobTimeout,
	JobTypeVideoWatermark:  transcodeJobTimeout,
	JobTypeCustomFFmpeg:    transcodeJobTimeout,
	JobTypeExtractAudio:    transcodeJobTimeout,
	JobTypeTranscodeAudio:  transcodeJobTimeout,
	JobTypeNormalizeAudio:  transcodeJobTimeout,
	JobTypeStackMerge:      transcodeJobTimeout,
	JobTypeRebuildVariants: transcodeJobTimeout,
	JobTypeExportRun:       transcodeJobTimeout,
}

// tunedJobTypes returns every job type with a non-default timeout or attempt
// budget — the set the stalled sweep must bucket individually.
func tunedJobTypes() []string {
	seen := make(map[string]struct{}, len(timeoutByType)+len(maxAttemptsByType))
	types := make([]string, 0, len(timeoutByType)+len(maxAttemptsByType))
	for jobType := range timeoutByType {
		seen[jobType] = struct{}{}
		types = append(types, jobType)
	}
	for jobType := range maxAttemptsByType {
		if _, ok := seen[jobType]; !ok {
			types = append(types, jobType)
		}
	}
	return types
}

// MaxAttemptsFor returns the retry budget for a job type.
func MaxAttemptsFor(jobType string) int64 {
	if n, ok := maxAttemptsByType[jobType]; ok {
		return n
	}
	return defaultMaxAttempts
}

// TimeoutFor returns the handler deadline for a job type.
func TimeoutFor(jobType string) time.Duration {
	if d, ok := timeoutByType[jobType]; ok {
		return d
	}
	return defaultJobTimeout
}

// retryBackoff returns the delay before the next attempt: 30s·2^(n-1),
// capped at 15 min. attempts is the number of attempts already made (≥1).
func retryBackoff(attempts int64) time.Duration {
	d := retryBackoffBase
	for i := int64(1); i < attempts; i++ {
		d *= 2
		if d >= retryBackoffCap {
			return retryBackoffCap
		}
	}
	return d
}

// typeList renders job types as the comma-wrapped list format expected by the
// instr() filters in jobs.sql: ",a,b," — empty slice yields "" (matches none).
func typeList(types []string) string {
	if len(types) == 0 {
		return ""
	}
	return "," + strings.Join(types, ",") + ","
}

// NoExcludedTypes is the ClaimNextJob argument meaning "claim any type".
const NoExcludedTypes = ""
