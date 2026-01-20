// Package store provides database connection management for Manfred.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database connection with Manfred-specific configuration.
type DB struct {
	*sql.DB
	path string
	mu   sync.RWMutex
}

// Open creates or opens a SQLite database at the specified path.
// It configures the database with WAL mode for better concurrency.
func Open(path string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	// Open with modernc.org/sqlite driver
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite works best with single writer
	db.SetMaxIdleConns(1)

	// Enable WAL mode and foreign keys
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("execute %s: %w", pragma, err)
		}
	}

	return &DB{
		DB:   db,
		path: path,
	}, nil
}

// OpenInMemory creates an in-memory SQLite database for testing.
func OpenInMemory() (*DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}

	// Enable foreign keys for in-memory DB
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return &DB{
		DB:   db,
		path: ":memory:",
	}, nil
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// Close closes the database connection.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Checkpoint WAL before closing
	if db.path != ":memory:" {
		_, _ = db.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	}

	return db.DB.Close()
}

// Migrate runs all pending database migrations.
func (db *DB) Migrate(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return runMigrations(ctx, db.DB)
}

// Transaction executes a function within a database transaction.
func (db *DB) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
