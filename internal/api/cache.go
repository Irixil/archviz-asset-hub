package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
)

const cacheMaxAge = 86_400 // 24 hours

// setCacheHeaders sets Cache-Control, ETag, and Last-Modified response headers,
// then checks If-None-Match and If-Modified-Since conditional request headers.
// Returns true if the response should be 304 Not Modified (caller must return nil).
// When immutable is true, adds the immutable directive (for versioned/variant files).
func setCacheHeaders(c fiber.Ctx, etag string, lastMod time.Time, immutable bool) bool {
	cc := fmt.Sprintf("private, max-age=%d", cacheMaxAge)
	if immutable {
		cc += ", immutable"
	}
	c.Set("Cache-Control", cc)
	c.Set("ETag", `"`+etag+`"`)
	c.Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))

	// ETag check takes precedence over date check.
	// We intentionally do not treat If-None-Match: * as a cache hit: RFC 7232 §3.2
	// reserves that wildcard for PUT precondition checks, not GET cache validation.
	if inm := c.Get("If-None-Match"); inm != "" {
		if inm == `"`+etag+`"` {
			c.Status(fiber.StatusNotModified)
			return true
		}
	}

	// Fallback: date-based conditional check.
	if ims := c.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil {
			if !lastMod.UTC().After(t) {
				c.Status(fiber.StatusNotModified)
				return true
			}
		}
	}

	return false
}

// parseVersionTime parses an AssetVersion.CreatedAt string (RFC3339 or SQLite
// datetime format) into a [time.Time]. Returns the zero time on failure so the
// caller skips conditional-cache logic rather than serving a stale 304.
func parseVersionTime(ctx context.Context, s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	slog.WarnContext(ctx, "parseVersionTime: unrecognized format", "value", s)
	return time.Time{}
}
