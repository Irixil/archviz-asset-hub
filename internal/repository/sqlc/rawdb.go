package reposqlc

import (
	"context"
	"database/sql"
)

// rawDB is satisfied by both [sql.DB] and [sql.Tx], letting repos that bypass
// dbgen (raw hand-rolled SQL) share the same field between pooled and
// transaction-scoped instances.
type rawDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
