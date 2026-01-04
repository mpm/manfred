package ticket

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Status represents the current state of a ticket.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusError      Status = "error"
	StatusCompleted  Status = "completed"
)

// AllStatuses returns all valid ticket statuses.
func AllStatuses() []Status {
	return []Status{StatusPending, StatusInProgress, StatusError, StatusCompleted}
}

// EntryType represents the type of a ticket entry.
type EntryType string

const (
	EntryTypePrompt  EntryType = "prompt"
	EntryTypeComment EntryType = "comment"
)

// Entry represents a prompt or comment on a ticket.
type Entry struct {
	Type      EntryType `yaml:"type"`
	Author    string    `yaml:"author"`
	Timestamp time.Time `yaml:"timestamp"`
	Content   string    `yaml:"content"`
}

// Ticket represents a task to be processed.
type Ticket struct {
	ID        string    `yaml:"id"`
	Project   string    `yaml:"project"`
	Status    Status    `yaml:"status"`
	CreatedAt time.Time `yaml:"created_at"`
	JobID     string    `yaml:"job_id,omitempty"`
	Entries   []Entry   `yaml:"entries"`
}

// New creates a new ticket with a generated ID.
func New(project string) *Ticket {
	return &Ticket{
		ID:        generateTicketID(),
		Project:   project,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		Entries:   []Entry{},
	}
}

// AddEntry adds an entry to the ticket.
func (t *Ticket) AddEntry(entryType EntryType, author, content string) {
	t.Entries = append(t.Entries, Entry{
		Type:      entryType,
		Author:    author,
		Timestamp: time.Now(),
		Content:   content,
	})
}

// PromptContent returns the content of the first prompt entry.
func (t *Ticket) PromptContent() string {
	for _, e := range t.Entries {
		if e.Type == EntryTypePrompt {
			return e.Content
		}
	}
	return ""
}

// PromptPreview returns a truncated preview of the prompt.
func (t *Ticket) PromptPreview(maxLen int) string {
	content := t.PromptContent()
	if content == "" {
		return "(no prompt)"
	}

	// Take first line only
	if idx := strings.Index(content, "\n"); idx > 0 {
		content = content[:idx]
	}

	if len(content) > maxLen {
		return content[:maxLen] + "..."
	}
	return content
}

// generateTicketID creates a unique ticket identifier.
func generateTicketID() string {
	// Format: ticket_YYYYMMDD_HHMMSS_xxxx
	now := time.Now()
	timestamp := now.Format("20060102_150405")

	b := make([]byte, 2)
	rand.Read(b)
	suffix := hex.EncodeToString(b)

	return fmt.Sprintf("ticket_%s_%s", timestamp, suffix)
}
