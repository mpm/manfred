package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mpm/manfred/internal/config"
	"gopkg.in/yaml.v3"
)

// Initializer handles project setup.
type Initializer struct {
	config *config.Config
}

// NewInitializer creates a new project initializer.
func NewInitializer(cfg *config.Config) *Initializer {
	return &Initializer{config: cfg}
}

// Init initializes a new project by cloning the repository.
func (i *Initializer) Init(ctx context.Context, name, repoURL string) error {
	projectDir := filepath.Join(i.config.ProjectsDir, name)
	repoDir := filepath.Join(projectDir, "repository")

	// Check if project already exists
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("project already exists: %s", name)
	}

	// Create project directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Clone repository
	cmd := exec.CommandContext(ctx, "git", "clone", repoURL, repoDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(projectDir) // Cleanup on failure
		return fmt.Errorf("failed to clone repository: %w\n%s", err, output)
	}

	// Detect default branch
	defaultBranch := detectDefaultBranch(ctx, repoDir)

	// Detect compose file
	composeFile := detectComposeFile(repoDir)

	// Generate project.yml
	projectConfig := config.ProjectConfig{
		Name:          name,
		Repo:          repoURL,
		DefaultBranch: defaultBranch,
		Docker: config.DockerConfig{
			ComposeFile: composeFile,
			MainService: "app",
			Workdir:     "/app",
		},
	}

	projectYml := filepath.Join(projectDir, "project.yml")
	data, err := yaml.Marshal(&projectConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize project config: %w", err)
	}

	if err := os.WriteFile(projectYml, data, 0644); err != nil {
		return fmt.Errorf("failed to write project.yml: %w", err)
	}

	return nil
}

func detectDefaultBranch(ctx context.Context, repoDir string) string {
	// Try to get the default branch from git
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "symbolic-ref", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "main"
	}
	branch := string(output)
	if branch != "" {
		return branch[:len(branch)-1] // Remove trailing newline
	}
	return "main"
}

func detectComposeFile(repoDir string) string {
	// Check for common compose file names
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, name := range candidates {
		if _, err := os.Stat(filepath.Join(repoDir, name)); err == nil {
			return name
		}
	}

	return "docker-compose.yml"
}
