package docker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/client"
)

// ContainerJobPath is where the job directory is mounted inside containers.
const ContainerJobPath = "/manfred-job"

// Client wraps Docker SDK operations.
type Client struct {
	docker *client.Client
}

// ComposeOptions configures a compose up operation.
type ComposeOptions struct {
	ComposeFile string
	ProjectName string
	Env         map[string]string
	Volumes     []VolumeMount
	Stdout      io.Writer // Optional: stream stdout here
	Stderr      io.Writer // Optional: stream stderr here
}

// VolumeMount represents a volume to mount into containers.
type VolumeMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// ExecOptions configures a container exec operation.
type ExecOptions struct {
	Workdir string
	Env     map[string]string
	Stdout  io.Writer
	Stderr  io.Writer
}

// New creates a new Docker client.
func New() (*Client, error) {
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Client{docker: docker}, nil
}

// Close closes the Docker client connection.
func (c *Client) Close() error {
	return c.docker.Close()
}

// ComposeUp starts containers using docker compose.
// We use exec because the Docker SDK doesn't have native compose support.
func (c *Client) ComposeUp(ctx context.Context, opts ComposeOptions) error {
	args := []string{"compose", "-f", opts.ComposeFile}

	// Generate override file for additional volumes
	var overrideFile string
	if len(opts.Volumes) > 0 {
		var err error
		overrideFile, err = c.generateComposeOverride(opts.ComposeFile, opts.Volumes)
		if err != nil {
			return fmt.Errorf("failed to generate compose override: %w", err)
		}
		defer os.Remove(overrideFile)
		args = append(args, "-f", overrideFile)
	}

	args = append(args, "-p", opts.ProjectName, "up", "-d", "--build")

	cmd := exec.CommandContext(ctx, "docker", args...)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Stream output if writers provided
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}

	// If no writers, capture output for error message
	if opts.Stdout == nil && opts.Stderr == nil {
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("compose up failed: %w\n%s", err, output)
		}
		return nil
	}

	// With streaming, just run
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compose up failed: %w", err)
	}

	return nil
}

// ComposeDown stops and removes containers.
func (c *Client) ComposeDown(ctx context.Context, composeFile, projectName string) error {
	args := []string{"compose"}
	if composeFile != "" {
		args = append(args, "-f", composeFile)
	}
	args = append(args, "-p", projectName, "down", "--remove-orphans")

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose down failed: %w\n%s", err, output)
	}

	return nil
}

// Exec runs a command in a container and streams output.
func (c *Client) Exec(ctx context.Context, containerName string, command []string, opts ExecOptions) error {
	args := []string{"exec"}

	if opts.Workdir != "" {
		args = append(args, "-w", opts.Workdir)
	}

	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, containerName)
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)

	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}

	return cmd.Run()
}

// ExecCapture runs a command and returns its output.
func (c *Client) ExecCapture(ctx context.Context, containerName string, command []string) (string, error) {
	args := []string{"exec", containerName}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// ExecSilent runs a command and returns success/failure.
func (c *Client) ExecSilent(ctx context.Context, containerName string, command []string) bool {
	args := []string{"exec", containerName}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd.Run() == nil
}

// IsRunning checks if a container is running.
func (c *Client) IsRunning(ctx context.Context, containerName string) (bool, error) {
	info, err := c.docker.ContainerInspect(ctx, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return info.State.Running, nil
}

// WaitForContainer waits until a container is running.
func (c *Client) WaitForContainer(ctx context.Context, containerName string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			running, err := c.IsRunning(ctx, containerName)
			if err != nil {
				return fmt.Errorf("error checking container %s: %w", containerName, err)
			}
			if running {
				return nil
			}
		}
	}
}

// SetupCredentialSymlinks creates symlinks for Claude credentials inside a container.
func (c *Client) SetupCredentialSymlinks(ctx context.Context, containerName string) error {
	credentialsPath := filepath.Join(ContainerJobPath, ".credentials.json")

	// Check if credentials exist in the job directory
	if !c.ExecSilent(ctx, containerName, []string{"test", "-f", credentialsPath}) {
		return nil // No credentials, skip
	}

	// Get the home directory of the current user in the container
	homeDir, err := c.ExecCapture(ctx, containerName, []string{"sh", "-c", "echo $HOME"})
	if err != nil {
		// Fall back to /root if we can't determine home
		homeDir = "/root"
	}
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		homeDir = "/root"
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	credentialsTarget := filepath.Join(claudeDir, ".credentials.json")

	// Create .claude directory
	output, err := c.ExecCaptureWithError(ctx, containerName, []string{"mkdir", "-p", claudeDir})
	if err != nil {
		return fmt.Errorf("failed to create .claude directory at %s: %v (output: %s)", claudeDir, err, output)
	}

	// Create symlink
	output, err = c.ExecCaptureWithError(ctx, containerName, []string{"ln", "-sf", credentialsPath, credentialsTarget})
	if err != nil {
		return fmt.Errorf("failed to create credential symlink: %v (output: %s)", err, output)
	}

	return nil
}

// ExecCaptureWithError runs a command and returns output and error details.
func (c *Client) ExecCaptureWithError(ctx context.Context, containerName string, command []string) (string, error) {
	args := []string{"exec", containerName}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// DebugContainers shows container information for debugging.
func (c *Client) DebugContainers(ctx context.Context, projectName string, out io.Writer) {
	// Show all containers for this project
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+projectName, "--format", "{{.Names}}\t{{.Status}}")
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Run()

	// Also show recent docker compose logs
	cmd = exec.CommandContext(ctx, "docker", "compose", "-p", projectName, "logs", "--tail=20")
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Run()
}

// generateComposeOverride creates a temporary compose override file with additional volumes.
func (c *Client) generateComposeOverride(composeFile string, volumes []VolumeMount) (string, error) {
	// Read original compose file to find service names
	content, err := os.ReadFile(composeFile)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}

	services := extractServiceNames(string(content))
	if len(services) == 0 {
		return "", fmt.Errorf("no services found in compose file")
	}

	// Build override YAML
	var override strings.Builder
	override.WriteString("services:\n")

	for _, service := range services {
		override.WriteString(fmt.Sprintf("  %s:\n", service))
		override.WriteString("    volumes:\n")

		for _, vol := range volumes {
			mode := "rw"
			if vol.ReadOnly {
				mode = "ro"
			}
			override.WriteString(fmt.Sprintf("      - %s:%s:%s\n", vol.Source, vol.Target, mode))
		}
	}

	// Write to temp file
	dir := filepath.Dir(composeFile)
	tmpFile, err := os.CreateTemp(dir, "manfred-override-*.yml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.WriteString(override.String()); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write override file: %w", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// extractServiceNames parses a docker-compose.yml and returns service names.
func extractServiceNames(content string) []string {
	var services []string
	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	inServices := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check if we're entering the services block
		if trimmed == "services:" {
			inServices = true
			continue
		}

		// If we hit another top-level key, stop
		if inServices && len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}

		// Look for service names (lines with exactly 2 spaces indent followed by name:)
		if inServices && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			name := strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
			if name != "" && !strings.HasPrefix(name, "#") {
				services = append(services, name)
			}
		}
	}

	return services
}

// ContainerName returns the container name for a compose project and service.
func ContainerName(projectName, service string) string {
	return fmt.Sprintf("%s-%s-1", projectName, service)
}
