// Package db manages the SQLite database connection and migrations.
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"sync/atomic"

	db "damask/server/internal/db/gen"

	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	_ "modernc.org/sqlite" //nolint:nolintlint // to register the sqlite driver
)

// memDBSeq disambiguates concurrent/successive in-memory databases within
// one process (e.g. one per test). Without a unique name, cache=shared would
// make every ":memory:" Open() call in the process resolve to the exact same
// underlying database — see Open for details.
var memDBSeq atomic.Uint64

// pragmas are applied per-connection via DSN _pragma params so every pooled
// connection gets them, instead of a one-shot Exec that a recreated
// connection would silently miss.
const pragmas = "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"

// DB holds the split writer/reader connection pools. SQLite allows one
// writer but many concurrent readers under WAL mode; a single shared pool
// with SetMaxOpenConns(1) (the old design) serialized reads behind writes
// for no reason.
type DB struct {
	// Writer is limited to a single connection (SQLite allows one writer at
	// a time) and issues BEGIN IMMEDIATE for every transaction (via the
	// _txlock=immediate DSN param) to avoid deferred-lock upgrade deadlocks.
	Writer *sql.DB
	// Reader allows multiple concurrent connections for read-only queries.
	Reader *sql.DB
	// WQ is a Queries instance bound to Writer.
	WQ *db.Queries
	// RQ is a Queries instance bound to Reader.
	RQ *db.Queries
}

// Open opens the SQLite database, runs migrations, and returns a split
// writer/reader DB.
func Open(dbPath string) (*DB, error) {
	name := filepath.Base(dbPath)

	// An in-memory DB is private per connection unless shared-cache mode is
	// requested; without it, the writer and reader pools would each see a
	// separate, empty database. Real file paths don't need this — the file
	// on disk is already shared across connections via WAL.
	//
	// Shared-cache in-memory databases are identified by name: the literal
	// ":memory:" name is shared by every connection in the process, so two
	// independent Open(":memory:") calls (e.g. two tests) would silently
	// collide on the same database. Mint a unique name per Open() call.
	extra := ""
	if dbPath == ":memory:" {
		dbPath = fmt.Sprintf("memdb%d", memDBSeq.Add(1))
		extra = "&mode=memory&cache=shared"
	}

	writer, err := otelsql.Open("sqlite", fmt.Sprintf("file:%s?_txlock=immediate&%s%s", dbPath, pragmas, extra),
		otelsql.WithAttributes(semconv.DBSystemSqlite),
		otelsql.WithDBName(name),
	)
	if err != nil {
		return nil, fmt.Errorf("open writer db: %w", err)
	}

	if err = RunMigrations(writer); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// SQLite supports one writer at a time; use a single connection to
	// prevent SQLITE_BUSY races between concurrent goroutines.
	writer.SetMaxOpenConns(1)

	reader, err := otelsql.Open("sqlite", fmt.Sprintf("file:%s?%s%s", dbPath, pragmas, extra),
		otelsql.WithAttributes(semconv.DBSystemSqlite),
		otelsql.WithDBName(name),
	)
	if err != nil {
		return nil, fmt.Errorf("open reader db: %w", err)
	}

	const minReaders = 4
	maxReaders := max(runtime.NumCPU(), minReaders)
	reader.SetMaxOpenConns(maxReaders)

	return &DB{
		Writer: writer,
		Reader: reader,
		WQ:     db.New(writer),
		RQ:     db.New(reader),
	}, nil
}

// WithTx returns a DB clone scoped to tx: WQ and RQ both wrap tx, since
// statements inside a transaction must all run on the same connection
// regardless of whether they're individually reads or writes. Writer/Reader
// are carried over unchanged for repos that also need raw [sql.DB] access
// outside the transacted dbgen.Queries calls.
func (d *DB) WithTx(tx *sql.Tx) *DB {
	q := db.New(tx)
	return &DB{Writer: d.Writer, Reader: d.Reader, WQ: q, RQ: q}
}

// Close closes both the writer and reader pools.
func (d *DB) Close() error {
	writerErr := d.Writer.Close()
	readerErr := d.Reader.Close()
	if writerErr != nil {
		return writerErr
	}
	return readerErr
}
