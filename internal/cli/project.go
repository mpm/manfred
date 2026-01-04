package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mpm/manfred/internal/config"
	"github.com/mpm/manfred/internal/project"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project management commands",
	}

	cmd.AddCommand(newProjectInitCmd())
	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newProjectShowCmd())

	return cmd
}

func newProjectInitCmd() *cobra.Command {
	var repoURL string

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Initialize a new project",
		Long: `Initialize a new project by cloning a repository.

Creates a project directory with project.yml configuration.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if repoURL == "" {
				return fmt.Errorf("--repo is required")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			init := project.NewInitializer(cfg)
			if err := init.Init(cmd.Context(), name, repoURL); err != nil {
				return err
			}

			fmt.Printf("Project %s initialized successfully\n", name)
			fmt.Printf("Location: %s\n", filepath.Join(cfg.ProjectsDir, name))
			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "Git repository URL (required)")
	cmd.MarkFlagRequired("repo")

	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			entries, err := os.ReadDir(cfg.ProjectsDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No projects found.")
					return nil
				}
				return err
			}

			for _, e := range entries {
				if e.IsDir() {
					projectYml := filepath.Join(cfg.ProjectsDir, e.Name(), "project.yml")
					if _, err := os.Stat(projectYml); err == nil {
						fmt.Println(e.Name())
					}
				}
			}
			return nil
		},
	}
}

func newProjectShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show project configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			projCfg, err := cfg.ProjectConfig(name)
			if err != nil {
				return err
			}

			fmt.Printf("Name: %s\n", projCfg.Name)
			if projCfg.Repo != "" {
				fmt.Printf("Repo: %s\n", projCfg.Repo)
			}
			if projCfg.DefaultBranch != "" {
				fmt.Printf("Default Branch: %s\n", projCfg.DefaultBranch)
			}
			fmt.Printf("Compose File: %s\n", projCfg.Docker.ComposeFile)
			fmt.Printf("Main Service: %s\n", projCfg.Docker.MainService)
			fmt.Printf("Workdir: %s\n", projCfg.Docker.Workdir)

			return nil
		},
	}
}
