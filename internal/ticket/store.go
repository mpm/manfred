package ticket

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Store defines the interface for ticket storage.
type Store interface {
	List(ctx context.Context, status *Status) ([]Ticket, error)
	Get(ctx context.Context, id string) (*Ticket, error)
	Create(ctx context.Context, prompt string) (*Ticket, error)
	Update(ctx context.Context, ticket *Ticket) error
	Stats(ctx context.Context) (map[Status]int, error)
	NextPending(ctx context.Context) (*Ticket, error)
}

// FileStore implements Store using the filesystem.
type FileStore struct {
	baseDir string
	project string
}

// NewFileStore creates a new filesystem-based ticket store.
func NewFileStore(ticketsDir, project string) *FileStore {
	return &FileStore{
		baseDir: filepath.Join(ticketsDir, project),
		project: project,
	}
}

// List returns all tickets, optionally filtered by status.
func (s *FileStore) List(ctx context.Context, status *Status) ([]Ticket, error) {
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}

	statuses := AllStatuses()
	if status != nil {
		statuses = []Status{*status}
	}

	var tickets []Ticket
	for _, st := range statuses {
		stTickets, err := s.listByStatus(st)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, stTickets...)
	}

	// Sort by creation time
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].CreatedAt.Before(tickets[j].CreatedAt)
	})

	return tickets, nil
}

// Get returns a ticket by ID.
func (s *FileStore) Get(ctx context.Context, id string) (*Ticket, error) {
	for _, status := range AllStatuses() {
		path := s.ticketPath(id, status)
		if _, err := os.Stat(path); err == nil {
			return s.loadTicket(path)
		}
	}
	return nil, nil
}

// Create creates a new ticket with the given prompt.
func (s *FileStore) Create(ctx context.Context, prompt string) (*Ticket, error) {
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}

	ticket := New(s.project)
	ticket.AddEntry(EntryTypePrompt, "user", prompt)

	if err := s.saveTicket(ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// Update saves changes to a ticket.
func (s *FileStore) Update(ctx context.Context, ticket *Ticket) error {
	// Find and remove old file if status changed
	oldStatus := s.findTicketStatus(ticket.ID)
	if oldStatus != nil && *oldStatus != ticket.Status {
		oldPath := s.ticketPath(ticket.ID, *oldStatus)
		os.Remove(oldPath)
	}

	return s.saveTicket(ticket)
}

// Stats returns ticket counts by status.
func (s *FileStore) Stats(ctx context.Context) (map[Status]int, error) {
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}

	stats := make(map[Status]int)
	for _, status := range AllStatuses() {
		dir := s.statusDirectory(status)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				stats[status] = 0
				continue
			}
			return nil, err
		}

		count := 0
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".yml" {
				count++
			}
		}
		stats[status] = count
	}

	return stats, nil
}

// NextPending returns the oldest pending ticket.
func (s *FileStore) NextPending(ctx context.Context) (*Ticket, error) {
	pending := StatusPending
	tickets, err := s.List(ctx, &pending)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		return nil, nil
	}
	return &tickets[0], nil
}

func (s *FileStore) ensureDirectories() error {
	for _, status := range AllStatuses() {
		dir := s.statusDirectory(status)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (s *FileStore) statusDirectory(status Status) string {
	return filepath.Join(s.baseDir, string(status))
}

func (s *FileStore) ticketPath(id string, status Status) string {
	return filepath.Join(s.statusDirectory(status), id+".yml")
}

func (s *FileStore) listByStatus(status Status) ([]Ticket, error) {
	dir := s.statusDirectory(status)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tickets []Ticket
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
			continue
		}

		path := filepath.Join(dir, e.Name())
		ticket, err := s.loadTicket(path)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, *ticket)
	}

	return tickets, nil
}

func (s *FileStore) loadTicket(path string) (*Ticket, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read ticket: %w", err)
	}

	var ticket Ticket
	if err := yaml.Unmarshal(data, &ticket); err != nil {
		return nil, fmt.Errorf("failed to parse ticket: %w", err)
	}

	return &ticket, nil
}

func (s *FileStore) saveTicket(ticket *Ticket) error {
	path := s.ticketPath(ticket.ID, ticket.Status)

	data, err := yaml.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("failed to serialize ticket: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write ticket: %w", err)
	}

	return nil
}

func (s *FileStore) findTicketStatus(id string) *Status {
	for _, status := range AllStatuses() {
		path := s.ticketPath(id, status)
		if _, err := os.Stat(path); err == nil {
			return &status
		}
	}
	return nil
}
