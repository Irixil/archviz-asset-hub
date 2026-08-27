package admin

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" //nolint:nolintlint // to register the sqlite driver
)

func OpenReadOnly(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("cannot open database at %s: %w", path, err)
	}
	return db, nil
}
