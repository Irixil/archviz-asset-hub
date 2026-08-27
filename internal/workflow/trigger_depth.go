package workflow

import "context"

// triggerDepthKey is the context key carrying the workflow trigger depth.
// Depth 0 (absent) means the mutation originates from a user or system
// action; depth >= 1 means it was caused by a workflow run. The dispatcher
// refuses to fire triggers from contexts at depth >= 1, which structurally
// prevents a workflow from re-triggering itself (or another workflow) in an
// infinite loop.
type triggerDepthKeyType struct{}

var triggerDepthKey triggerDepthKeyType

// WithTriggerDepth returns a context carrying the given workflow trigger depth.
func WithTriggerDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, triggerDepthKey, depth)
}

// TriggerDepthFrom returns the workflow trigger depth carried by ctx, or 0
// when the context does not originate from a workflow run.
func TriggerDepthFrom(ctx context.Context) int {
	if d, ok := ctx.Value(triggerDepthKey).(int); ok {
		return d
	}
	return 0
}
