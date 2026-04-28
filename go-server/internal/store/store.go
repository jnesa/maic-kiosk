// Package store is the SQLite repository layer. One Store value holds the
// connection and exposes typed methods grouped by entity (hotels, kiosks,
// users, sessions, audit). The schema is bootstrapped on first open.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store wraps the SQLite handle. Safe for concurrent use by HTTP handlers
// because *sql.DB has its own pool. We keep one *Store for the whole
// process.
type Store struct {
	db *sql.DB
}

// Open dials SQLite at `path` and applies the embedded schema if the
// schema_version table doesn't exist yet. Idempotent — safe to call on
// every boot.
func Open(path string) (*Store, error) {
	// `_pragma=foreign_keys(1)` enforces FKs at the connection level
	// (modernc.org/sqlite respects this DSN form). WAL mode + busy-timeout
	// keeps writes from blocking the kiosk read path.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite likes a small, focused pool. Two writers + a few readers is
	// plenty for admin volume; the kiosk path uses an in-memory cache.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// DB returns the underlying *sql.DB for callers that need transactions.
// Prefer the typed methods on Store wherever possible.
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the connection pool. Call from the server's graceful
// shutdown path.
func (s *Store) Close() error { return s.db.Close() }

// nowISO is the canonical timestamp format we write to SQLite text columns.
// SQLite has no native datetime type — keeping ISO-8601 in TEXT keeps the
// data legible if someone opens data.db in the sqlite3 CLI.
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
