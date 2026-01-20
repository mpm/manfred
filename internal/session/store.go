package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mpm/manfred/internal/store"
)

// Store defines the interface for session persistence.
type Store interface {
	// Create creates a new session.
	Create(ctx context.Context, s *Session) error

	// Get retrieves a session by ID.
	Get(ctx context.Context, id string) (*Session, error)

	// GetByIssue retrieves a session by repository and issue number.
	GetByIssue(ctx context.Context, owner, repo string, issueNumber int) (*Session, error)

	// Update updates an existing session.
	Update(ctx context.Context, s *Session) error

	// Delete deletes a session by ID.
	Delete(ctx context.Context, id string) error

	// List returns sessions matching the filter criteria.
	List(ctx context.Context, filter SessionFilter) ([]Session, error)

	// RecordEvent records an event in the session's history.
	RecordEvent(ctx context.Context, sessionID string, eventType EventType, payload interface{}) error

	// GetEvents retrieves events for a session.
	GetEvents(ctx context.Context, sessionID string) ([]SessionEvent, error)

	// Count returns the number of sessions matching the filter.
	Count(ctx context.Context, filter SessionFilter) (int, error)
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *store.DB
}

// NewSQLiteStore creates a new SQLite-backed session store.
func NewSQLiteStore(db *store.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// Create creates a new session.
func (s *SQLiteStore) Create(ctx context.Context, sess *Session) error {
	if err := sess.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	query := `
		INSERT INTO sessions (
			id, repo_owner, repo_name, issue_number, pr_number,
			phase, branch, container_id, plan_content, error_message,
			created_at, last_activity
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		sess.ID,
		sess.RepoOwner,
		sess.RepoName,
		sess.IssueNumber,
		sess.PRNumber,
		string(sess.Phase),
		sess.Branch,
		sess.ContainerID,
		sess.PlanContent,
		sess.ErrorMessage,
		sess.CreatedAt,
		sess.LastActivity,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("session already exists for %s/%s#%d", sess.RepoOwner, sess.RepoName, sess.IssueNumber)
		}
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// Get retrieves a session by ID.
func (s *SQLiteStore) Get(ctx context.Context, id string) (*Session, error) {
	query := `
		SELECT id, repo_owner, repo_name, issue_number, pr_number,
			   phase, branch, container_id, plan_content, error_message,
			   created_at, last_activity
		FROM sessions
		WHERE id = ?
	`

	sess := &Session{}
	var phase string
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&sess.ID,
		&sess.RepoOwner,
		&sess.RepoName,
		&sess.IssueNumber,
		&sess.PRNumber,
		&phase,
		&sess.Branch,
		&sess.ContainerID,
		&sess.PlanContent,
		&sess.ErrorMessage,
		&sess.CreatedAt,
		&sess.LastActivity,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	sess.Phase = Phase(phase)
	return sess, nil
}

// GetByIssue retrieves a session by repository and issue number.
func (s *SQLiteStore) GetByIssue(ctx context.Context, owner, repo string, issueNumber int) (*Session, error) {
	query := `
		SELECT id, repo_owner, repo_name, issue_number, pr_number,
			   phase, branch, container_id, plan_content, error_message,
			   created_at, last_activity
		FROM sessions
		WHERE repo_owner = ? AND repo_name = ? AND issue_number = ?
	`

	sess := &Session{}
	var phase string
	err := s.db.QueryRowContext(ctx, query, owner, repo, issueNumber).Scan(
		&sess.ID,
		&sess.RepoOwner,
		&sess.RepoName,
		&sess.IssueNumber,
		&sess.PRNumber,
		&phase,
		&sess.Branch,
		&sess.ContainerID,
		&sess.PlanContent,
		&sess.ErrorMessage,
		&sess.CreatedAt,
		&sess.LastActivity,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session by issue: %w", err)
	}

	sess.Phase = Phase(phase)
	return sess, nil
}

// Update updates an existing session.
func (s *SQLiteStore) Update(ctx context.Context, sess *Session) error {
	if err := sess.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	query := `
		UPDATE sessions SET
			pr_number = ?,
			phase = ?,
			container_id = ?,
			plan_content = ?,
			error_message = ?,
			last_activity = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		sess.PRNumber,
		string(sess.Phase),
		sess.ContainerID,
		sess.PlanContent,
		sess.ErrorMessage,
		sess.LastActivity,
		sess.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sess.ID)
	}

	return nil
}

// Delete deletes a session by ID.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM sessions WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("session not found: %s", id)
	}

	return nil
}

// List returns sessions matching the filter criteria.
func (s *SQLiteStore) List(ctx context.Context, filter SessionFilter) ([]Session, error) {
	var conditions []string
	var args []interface{}

	if filter.RepoOwner != "" {
		conditions = append(conditions, "repo_owner = ?")
		args = append(args, filter.RepoOwner)
	}
	if filter.RepoName != "" {
		conditions = append(conditions, "repo_name = ?")
		args = append(args, filter.RepoName)
	}
	if filter.Phase != nil {
		conditions = append(conditions, "phase = ?")
		args = append(args, string(*filter.Phase))
	}
	if filter.ActiveOnly {
		conditions = append(conditions, "phase NOT IN (?, ?)")
		args = append(args, string(PhaseCompleted), string(PhaseError))
	}

	query := `
		SELECT id, repo_owner, repo_name, issue_number, pr_number,
			   phase, branch, container_id, plan_content, error_message,
			   created_at, last_activity
		FROM sessions
	`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY last_activity DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var phase string
		err := rows.Scan(
			&sess.ID,
			&sess.RepoOwner,
			&sess.RepoName,
			&sess.IssueNumber,
			&sess.PRNumber,
			&phase,
			&sess.Branch,
			&sess.ContainerID,
			&sess.PlanContent,
			&sess.ErrorMessage,
			&sess.CreatedAt,
			&sess.LastActivity,
		)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sess.Phase = Phase(phase)
		sessions = append(sessions, sess)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

// RecordEvent records an event in the session's history.
func (s *SQLiteStore) RecordEvent(ctx context.Context, sessionID string, eventType EventType, payload interface{}) error {
	var payloadJSON string
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}
		payloadJSON = string(data)
	}

	query := `
		INSERT INTO session_events (session_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query, sessionID, string(eventType), payloadJSON, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("record event: %w", err)
	}

	return nil
}

// GetEvents retrieves events for a session.
func (s *SQLiteStore) GetEvents(ctx context.Context, sessionID string) ([]SessionEvent, error) {
	query := `
		SELECT id, session_id, event_type, payload, created_at
		FROM session_events
		WHERE session_id = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}
	defer rows.Close()

	var events []SessionEvent
	for rows.Next() {
		var event SessionEvent
		var eventType string
		err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&eventType,
			&event.Payload,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.EventType = EventType(eventType)
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

// Count returns the number of sessions matching the filter.
func (s *SQLiteStore) Count(ctx context.Context, filter SessionFilter) (int, error) {
	var conditions []string
	var args []interface{}

	if filter.RepoOwner != "" {
		conditions = append(conditions, "repo_owner = ?")
		args = append(args, filter.RepoOwner)
	}
	if filter.RepoName != "" {
		conditions = append(conditions, "repo_name = ?")
		args = append(args, filter.RepoName)
	}
	if filter.Phase != nil {
		conditions = append(conditions, "phase = ?")
		args = append(args, string(*filter.Phase))
	}
	if filter.ActiveOnly {
		conditions = append(conditions, "phase NOT IN (?, ?)")
		args = append(args, string(PhaseCompleted), string(PhaseError))
	}

	query := `SELECT COUNT(*) FROM sessions`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}

	return count, nil
}
