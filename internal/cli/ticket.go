package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mpm/manfred/internal/config"
	"github.com/mpm/manfred/internal/ticket"
	"github.com/spf13/cobra"
)

func newTicketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Ticket management commands",
	}

	cmd.AddCommand(newTicketNewCmd())
	cmd.AddCommand(newTicketListCmd())
	cmd.AddCommand(newTicketShowCmd())
	cmd.AddCommand(newTicketStatsCmd())
	cmd.AddCommand(newTicketProcessCmd())

	return cmd
}

func newTicketNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <project> [prompt]",
		Short: "Create a new ticket",
		Long: `Creates a new ticket for the specified project.

If prompt is provided, uses it as the ticket content.
Otherwise, reads from stdin.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			var prompt string

			if len(args) > 1 {
				prompt = args[1]
			} else {
				// Read from stdin
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				prompt = strings.TrimSpace(string(data))
			}

			if prompt == "" {
				return fmt.Errorf("no prompt provided")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			store := ticket.NewFileStore(cfg.TicketsDir, project)
			t, err := store.Create(cmd.Context(), prompt)
			if err != nil {
				return err
			}

			fmt.Printf("Created ticket: %s\n", t.ID)
			fmt.Printf("Project: %s\n", project)
			fmt.Printf("Status: %s\n", t.Status)
			return nil
		},
	}
}

func newTicketListCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "list <project>",
		Short: "List tickets for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			store := ticket.NewFileStore(cfg.TicketsDir, project)

			var statusFilter *ticket.Status
			if status != "" {
				s := ticket.Status(status)
				statusFilter = &s
			}

			tickets, err := store.List(cmd.Context(), statusFilter)
			if err != nil {
				return err
			}

			if len(tickets) == 0 {
				fmt.Println("No tickets found.")
				return nil
			}

			for _, t := range tickets {
				preview := t.PromptPreview(50)
				fmt.Printf("%s  %-12s  %s\n", t.ID, t.Status, preview)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, in_progress, error, completed)")
	return cmd
}

func newTicketShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <project> <ticket-id>",
		Short: "Show ticket details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			ticketID := args[1]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			store := ticket.NewFileStore(cfg.TicketsDir, project)
			t, err := store.Get(cmd.Context(), ticketID)
			if err != nil {
				return err
			}
			if t == nil {
				return fmt.Errorf("ticket not found: %s", ticketID)
			}

			fmt.Printf("ID: %s\n", t.ID)
			fmt.Printf("Project: %s\n", t.Project)
			fmt.Printf("Status: %s\n", t.Status)
			fmt.Printf("Created: %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
			if t.JobID != "" {
				fmt.Printf("Job ID: %s\n", t.JobID)
			}
			fmt.Println()
			fmt.Println("Entries:")
			for _, e := range t.Entries {
				fmt.Println("---")
				fmt.Printf("[%s] %s by %s\n", e.Timestamp.Format("2006-01-02 15:04:05"), e.Type, e.Author)
				fmt.Println(e.Content)
			}
			return nil
		},
	}
}

func newTicketStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats [project]",
		Short: "Show ticket statistics",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			var projects []string
			if len(args) > 0 {
				projects = []string{args[0]}
			} else {
				// List all projects with tickets
				entries, err := os.ReadDir(cfg.TicketsDir)
				if err != nil {
					if os.IsNotExist(err) {
						fmt.Println("No tickets found.")
						return nil
					}
					return err
				}
				for _, e := range entries {
					if e.IsDir() {
						projects = append(projects, e.Name())
					}
				}
			}

			if len(projects) == 0 {
				fmt.Println("No projects with tickets found.")
				return nil
			}

			for _, project := range projects {
				if len(projects) > 1 {
					fmt.Printf("%s:\n", project)
				}

				store := ticket.NewFileStore(cfg.TicketsDir, project)
				stats, err := store.Stats(cmd.Context())
				if err != nil {
					return err
				}

				prefix := ""
				if len(projects) > 1 {
					prefix = "  "
				}

				total := 0
				for _, count := range stats {
					total += count
				}

				fmt.Printf("%sPending:     %d\n", prefix, stats[ticket.StatusPending])
				fmt.Printf("%sIn Progress: %d\n", prefix, stats[ticket.StatusInProgress])
				fmt.Printf("%sError:       %d\n", prefix, stats[ticket.StatusError])
				fmt.Printf("%sCompleted:   %d\n", prefix, stats[ticket.StatusCompleted])
				fmt.Printf("%sTotal:       %d\n", prefix, total)

				if len(projects) > 1 {
					fmt.Println()
				}
			}
			return nil
		},
	}
}

func newTicketProcessCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "process <project> [ticket-id]",
		Short: "Process a ticket (run as job)",
		Long: `Processes a ticket by running it as a MANFRED job.

If ticket-id is provided, processes that specific ticket.
Otherwise, processes the next pending ticket (FIFO).`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			var ticketID string
			if len(args) > 1 {
				ticketID = args[1]
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			processor := ticket.NewProcessor(cfg)
			t, err := processor.Process(cmd.Context(), project, ticketID)
			if err != nil {
				return err
			}

			if t == nil {
				fmt.Println("No tickets to process.")
				return nil
			}

			fmt.Printf("Processed ticket: %s\n", t.ID)
			fmt.Printf("Final status: %s\n", t.Status)
			if t.JobID != "" {
				fmt.Printf("Job ID: %s\n", t.JobID)
			}

			if t.Status != ticket.StatusCompleted {
				return fmt.Errorf("ticket processing failed")
			}
			return nil
		},
	}
}
