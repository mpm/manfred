package cli

import (
	"fmt"
	"os"

	"github.com/mpm/manfred/internal/config"
	"github.com/mpm/manfred/internal/job"
	"github.com/spf13/cobra"
)

func newJobCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "job <project> <prompt-file>",
		Short: "Run a job for a project",
		Long: `Run a Claude Code job for the specified project.

The prompt file contains the task description that will be sent to Claude Code.
Claude will work on the task inside the project's Docker container.`,
		Args: cobra.ExactArgs(2),
		RunE: runJob,
	}
}

func runJob(cmd *cobra.Command, args []string) error {
	projectName := args[0]
	promptFile := args[1]

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Read prompt
	prompt, err := os.ReadFile(promptFile)
	if err != nil {
		return fmt.Errorf("failed to read prompt file: %w", err)
	}

	// Create and run job
	runner, err := job.NewRunner(cfg)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	j, err := runner.Run(cmd.Context(), projectName, string(prompt))
	if err != nil {
		return fmt.Errorf("job failed: %w", err)
	}

	if j.Status == job.StatusCompleted {
		fmt.Printf("Job %s completed successfully\n", j.ID)
	} else {
		fmt.Printf("Job %s failed: %s\n", j.ID, j.Error)
		return fmt.Errorf("job failed")
	}

	return nil
}
