package auth

import "context"

// Actor identifies who triggered a service call.
// Type is "user" for authenticated users or "system" for background jobs.
type Actor struct {
	Type        string // "user" | "system"
	UserID      *string
	WorkspaceID *string
}

type actorKey struct{}

// WithActor returns a new context carrying the given actor.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFromCtx retrieves the actor from the context.
// Returns a zero-value Actor (Type == "") if none was set.
func ActorFromCtx(ctx context.Context) Actor {
	a, _ := ctx.Value(actorKey{}).(Actor)
	return a
}
