package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mpm/manfred/internal/config"
	"github.com/mpm/manfred/internal/session"
	"github.com/mpm/manfred/internal/store"
	"github.com/spf13/cobra"
)

// openSessionStore opens the database and returns a session store.
// The caller must call the returned cleanup function when done.
func openSessionStore(ctx context.Context) (*session.SQLiteStore, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	db, err := store.Open(cfg.Database.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("migrate database: %w", err)
	}

	cleanup := func() { db.Close() }
	return session.NewSQLiteStore(db), cleanup, nil
}

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "GitHub session management commands",
	}

	cmd.AddCommand(newSessionListCmd())
	cmd.AddCommand(newSessionShowCmd())
	cmd.AddCommand(newSessionDeleteCmd())
	cmd.AddCommand(newSessionStatsCmd())

	return cmd
}

func newSessionListCmd() *cobra.Command {
	var (
		repo       string
		phase      string
		activeOnly bool
		limit      int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List GitHub sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionStore, cleanup, err := openSessionStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			filter := session.SessionFilter{
				ActiveOnly: activeOnly,
				Limit:      limit,
			}

			// Parse repo filter (owner/repo format)
			if repo != "" {
				parts := strings.SplitN(repo, "/", 2)
				if len(parts) == 2 {
					filter.RepoOwner = parts[0]
					filter.RepoName = parts[1]
				} else {
					filter.RepoOwner = repo
				}
			}

			// Parse phase filter
			if phase != "" {
				p, err := session.ParsePhase(phase)
				if err != nil {
					return err
				}
				filter.Phase = &p
			}

			sessions, err := sessionStore.List(cmd.Context(), filter)
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			// Header
			fmt.Printf("%-40s  %-18s  %-10s  %s\n", "ID", "PHASE", "ISSUE", "LAST ACTIVITY")
			fmt.Println(strings.Repeat("-", 90))

			for _, s := range sessions {
				issueInfo := fmt.Sprintf("#%d", s.IssueNumber)
				if s.PRNumber != nil {
					issueInfo += fmt.Sprintf(" (PR #%d)", *s.PRNumber)
				}
				fmt.Printf("%-40s  %-18s  %-10s  %s\n",
					truncate(s.ID, 40),
					s.Phase.DisplayName(),
					issueInfo,
					s.LastActivity.Format("2006-01-02 15:04"),
				)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "Filter by repository (owner/repo)")
	cmd.Flags().StringVar(&phase, "phase", "", "Filter by phase")
	cmd.Flags().BoolVar(&activeOnly, "active", false, "Show only active sessions")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of sessions to show")

	return cmd
}

func newSessionShowCmd() *cobra.Command {
	var showEvents bool

	cmd := &cobra.Command{
		Use:   "show <session-id>",
		Short: "Show session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			sessionStore, cleanup, err := openSessionStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			s, err := sessionStore.Get(cmd.Context(), sessionID)
			if err != nil {
				return err
			}
			if s == nil {
				return fmt.Errorf("session not found: %s", sessionID)
			}

			fmt.Printf("ID:           %s\n", s.ID)
			fmt.Printf("Repository:   %s/%s\n", s.RepoOwner, s.RepoName)
			fmt.Printf("Issue:        #%d\n", s.IssueNumber)
			if s.PRNumber != nil {
				fmt.Printf("Pull Request: #%d\n", *s.PRNumber)
			}
			fmt.Printf("Phase:        %s\n", s.Phase.DisplayName())
			fmt.Printf("Branch:       %s\n", s.Branch)
			if s.ContainerID != nil {
				fmt.Printf("Container:    %s\n", *s.ContainerID)
			}
			fmt.Printf("Created:      %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Last Active:  %s\n", s.LastActivity.Format("2006-01-02 15:04:05"))

			if s.ErrorMessage != nil {
				fmt.Printf("\nError: %s\n", *s.ErrorMessage)
			}

			if s.PlanContent != nil && *s.PlanContent != "" {
				fmt.Println("\n--- Plan ---")
				fmt.Println(*s.PlanContent)
			}

			// Show valid transitions
			transitions := s.Phase.ValidTransitions()
			if len(transitions) > 0 {
				fmt.Println("\nValid transitions:")
				for _, t := range transitions {
					fmt.Printf("  -> %s\n", t.DisplayName())
				}
			}

			if showEvents {
				events, err := sessionStore.GetEvents(cmd.Context(), sessionID)
				if err != nil {
					return fmt.Errorf("get events: %w", err)
				}

				if len(events) > 0 {
					fmt.Println("\n--- Events ---")
					for _, e := range events {
						fmt.Printf("[%s] %s\n", e.CreatedAt.Format("2006-01-02 15:04:05"), e.EventType)
						if e.Payload != "" {
							fmt.Printf("    %s\n", truncate(e.Payload, 100))
						}
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showEvents, "events", false, "Show session events")

	return cmd
}

func newSessionDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <session-id>",
		Short: "Delete a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			sessionStore, cleanup, err := openSessionStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			if err := sessionStore.Delete(cmd.Context(), sessionID); err != nil {
				return err
			}

			fmt.Printf("Deleted session: %s\n", sessionID)
			return nil
		},
	}
}

func newSessionStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show session statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionStore, cleanup, err := openSessionStore(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			// Count by phase
			fmt.Println("Sessions by phase:")
			total := 0
			for _, phase := range session.AllPhases() {
				count, err := sessionStore.Count(cmd.Context(), session.SessionFilter{Phase: &phase})
				if err != nil {
					return err
				}
				total += count
				fmt.Printf("  %-20s %d\n", phase.DisplayName()+":", count)
			}
			fmt.Printf("  %-20s %d\n", "Total:", total)

			// Count active
			activeCount, err := sessionStore.Count(cmd.Context(), session.SessionFilter{ActiveOnly: true})
			if err != nil {
				return err
			}
			fmt.Printf("\nActive sessions: %d\n", activeCount)

			return nil
		},
	}
}

// truncate truncates a string to the given length, adding "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
