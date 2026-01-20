package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Migration represents a database schema migration.
type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

// migrations is the ordered list of all database migrations.
var migrations = []Migration{
	{
		Version:     1,
		Description: "Create sessions table",
		Up: `
			CREATE TABLE IF NOT EXISTS sessions (
				id TEXT PRIMARY KEY,
				repo_owner TEXT NOT NULL,
				repo_name TEXT NOT NULL,
				issue_number INTEGER NOT NULL,
				pr_number INTEGER,
				phase TEXT NOT NULL DEFAULT 'planning',
				branch TEXT NOT NULL,
				container_id TEXT,
				plan_content TEXT,
				error_message TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				last_activity TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(repo_owner, repo_name, issue_number)
			);

			CREATE INDEX IF NOT EXISTS idx_sessions_repo ON sessions(repo_owner, repo_name);
			CREATE INDEX IF NOT EXISTS idx_sessions_phase ON sessions(phase);
			CREATE INDEX IF NOT EXISTS idx_sessions_last_activity ON sessions(last_activity);
		`,
		Down: `
			DROP INDEX IF EXISTS idx_sessions_last_activity;
			DROP INDEX IF EXISTS idx_sessions_phase;
			DROP INDEX IF EXISTS idx_sessions_repo;
			DROP TABLE IF EXISTS sessions;
		`,
	},
	{
		Version:     2,
		Description: "Create session_events table",
		Up: `
			CREATE TABLE IF NOT EXISTS session_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
				event_type TEXT NOT NULL,
				payload TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_session_events_session ON session_events(session_id);
			CREATE INDEX IF NOT EXISTS idx_session_events_type ON session_events(event_type);
		`,
		Down: `
			DROP INDEX IF EXISTS idx_session_events_type;
			DROP INDEX IF EXISTS idx_session_events_session;
			DROP TABLE IF EXISTS session_events;
		`,
	},
	{
		Version:     3,
		Description: "Create schema_migrations table",
		Up: `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				description TEXT NOT NULL,
				applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
		`,
		Down: `
			DROP TABLE IF EXISTS schema_migrations;
		`,
	},
}

// runMigrations applies all pending migrations to the database.
func runMigrations(ctx context.Context, db *sql.DB) error {
	// First ensure the migrations table exists (bootstrap)
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get current version
	var currentVersion int
	row := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}

		// Skip migration 3 since we already created the table
		if m.Version == 3 {
			// Just record it as applied
			_, err := db.ExecContext(ctx,
				"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
				m.Version, m.Description)
			if err != nil {
				return fmt.Errorf("record migration %d: %w", m.Version, err)
			}
			continue
		}

		// Run migration in transaction
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction for migration %d: %w", m.Version, err)
		}

		if _, err := tx.ExecContext(ctx, m.Up); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Description, err)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
			m.Version, m.Description); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}

	return nil
}

// CurrentVersion returns the current schema version.
func CurrentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	row := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("get current version: %w", err)
	}
	return version, nil
}

// LatestVersion returns the latest available migration version.
func LatestVersion() int {
	if len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].Version
}
