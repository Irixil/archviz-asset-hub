package api

import (
	"context"
	"sync"
	"time"

	"damask/server/internal/auth"
)

// roleCacheTTL bounds how stale a cached role can be. Kept short (roadmap
// caps it at ≤30s) since role changes must take effect promptly, and paired
// with explicit invalidation on member update/removal so staleness in
// practice is bounded by request latency, not the TTL.
const roleCacheTTL = 15 * time.Second

type roleCacheKey struct {
	workspaceID string
	userID      string
}

type roleCacheEntry struct {
	role    auth.Role
	expires time.Time
}

// roleCache is a short-TTL, per-(workspace, user) cache in front of
// workspaceRepo.GetMember, which is otherwise called on every request behind
// auth.RequireRole. No background goroutine: expiry is checked lazily on get.
type roleCache struct {
	mu      sync.Mutex
	entries map[roleCacheKey]roleCacheEntry
}

func newRoleCache() *roleCache {
	return &roleCache{entries: make(map[roleCacheKey]roleCacheEntry)}
}

func (c *roleCache) get(workspaceID, userID string) (auth.Role, bool) {
	key := roleCacheKey{workspaceID: workspaceID, userID: userID}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expires) {
		delete(c.entries, key)
		return "", false
	}
	return entry.role, true
}

func (c *roleCache) set(workspaceID, userID string, role auth.Role) {
	key := roleCacheKey{workspaceID: workspaceID, userID: userID}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = roleCacheEntry{role: role, expires: time.Now().Add(roleCacheTTL)}
}

// invalidate drops the cached role for one (workspace, user) pair. Never
// affects any other workspace's cached entry for the same user ID — the key
// is scoped by both fields together.
func (c *roleCache) invalidate(workspaceID, userID string) {
	key := roleCacheKey{workspaceID: workspaceID, userID: userID}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// getRole resolves a user's role in a workspace, serving from roleCache when
// possible. Used as the getRoleFn passed to auth.RequireRole, which is
// otherwise a DB round trip on every role-gated request.
func (s *Server) getRole(ctx context.Context, workspaceID, userID string) (auth.Role, error) {
	if role, ok := s.roleCache.get(workspaceID, userID); ok {
		return role, nil
	}
	member, err := s.workspace.GetMember(ctx, workspaceID, userID)
	if err != nil {
		return "", err
	}
	role := auth.Role(member.Role)
	s.roleCache.set(workspaceID, userID, role)
	return role, nil
}
