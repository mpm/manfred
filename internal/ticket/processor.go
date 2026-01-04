package ticket

import (
	"context"
	"fmt"

	"github.com/mpm/manfred/internal/config"
	"github.com/mpm/manfred/internal/job"
)

// Processor handles ticket-to-job orchestration.
type Processor struct {
	config *config.Config
}

// NewProcessor creates a new ticket processor.
func NewProcessor(cfg *config.Config) *Processor {
	return &Processor{config: cfg}
}

// Process processes a ticket by running it as a job.
// If ticketID is empty, processes the next pending ticket.
// Returns the updated ticket after processing.
func (p *Processor) Process(ctx context.Context, project string, ticketID string) (*Ticket, error) {
	store := NewFileStore(p.config.TicketsDir, project)

	// Get the ticket to process
	var ticket *Ticket
	var err error

	if ticketID != "" {
		ticket, err = store.Get(ctx, ticketID)
		if err != nil {
			return nil, fmt.Errorf("failed to get ticket: %w", err)
		}
		if ticket == nil {
			return nil, fmt.Errorf("ticket not found: %s", ticketID)
		}
	} else {
		ticket, err = store.NextPending(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get next pending ticket: %w", err)
		}
		if ticket == nil {
			return nil, nil // No tickets to process
		}
	}

	// Validate ticket is processable
	if ticket.Status != StatusPending {
		return nil, fmt.Errorf("ticket %s is not pending (status: %s)", ticket.ID, ticket.Status)
	}

	// Get the prompt content
	prompt := ticket.PromptContent()
	if prompt == "" {
		return nil, fmt.Errorf("ticket %s has no prompt content", ticket.ID)
	}

	// Mark as in progress
	ticket.Status = StatusInProgress
	if err := store.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("failed to update ticket status: %w", err)
	}

	// Create and run the job
	runner, err := job.NewRunner(p.config)
	if err != nil {
		ticket.Status = StatusError
		ticket.AddEntry(EntryTypeComment, "manfred", fmt.Sprintf("Failed to create job runner: %v", err))
		store.Update(ctx, ticket)
		return ticket, fmt.Errorf("failed to create job runner: %w", err)
	}
	defer runner.Close()

	j, err := runner.Run(ctx, project, prompt)
	if err != nil {
		ticket.Status = StatusError
		ticket.AddEntry(EntryTypeComment, "manfred", fmt.Sprintf("Job failed: %v", err))
		store.Update(ctx, ticket)
		return ticket, fmt.Errorf("job failed: %w", err)
	}

	// Update ticket with job results
	ticket.JobID = j.ID

	if j.Status == job.StatusCompleted {
		ticket.Status = StatusCompleted
		comment := fmt.Sprintf("Job completed: %s", j.ID)
		if j.CommitMessage != "" {
			comment += fmt.Sprintf("\n\nCommit message:\n%s", j.CommitMessage)
		}
		ticket.AddEntry(EntryTypeComment, "manfred", comment)
	} else {
		ticket.Status = StatusError
		ticket.AddEntry(EntryTypeComment, "manfred", fmt.Sprintf("Job failed: %s\nError: %s", j.ID, j.Error))
	}

	if err := store.Update(ctx, ticket); err != nil {
		return ticket, fmt.Errorf("failed to update ticket: %w", err)
	}

	return ticket, nil
}
