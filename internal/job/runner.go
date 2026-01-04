package job

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mpm/manfred/internal/config"
	"github.com/mpm/manfred/internal/docker"
)

const (
	// ContainerCommitMessagePath is where Claude writes the commit message.
	ContainerCommitMessagePath = "/manfred-job/.manfred/commit_message.txt"

	// CommitMessagePrompt is the prompt for phase 2.
	CommitMessagePrompt = `Please summarize the changes you made in this session and create a git commit message.

Requirements:
1. First, create the directory if it doesn't exist: mkdir -p /manfred-job/.manfred
2. Write ONLY the commit message to the file: /manfred-job/.manfred/commit_message.txt
3. The commit message should follow conventional commit format
4. Include a brief summary line (max 72 chars) followed by a blank line and bullet points for details

Example format:
feat: Add user authentication system

- Implement login/logout functionality
- Add session management
- Create user model with password hashing`
)

// Runner orchestrates job execution.
type Runner struct {
	config *config.Config
	docker *docker.Client
	logger *Logger
}

// NewRunner creates a new job runner.
func NewRunner(cfg *config.Config) (*Runner, error) {
	dockerClient, err := docker.New()
	if err != nil {
		return nil, err
	}

	return &Runner{
		config: cfg,
		docker: dockerClient,
		logger: NewLogger(),
	}, nil
}

// Run executes a job for the given project and prompt.
func (r *Runner) Run(ctx context.Context, projectName, prompt string) (*Job, error) {
	// Validate project
	projectConfig, err := r.validateProject(projectName)
	if err != nil {
		return nil, err
	}

	// Create job
	job := New(projectName, prompt, r.config.JobsDir)

	r.logger.Manfred(fmt.Sprintf("Starting job %s", job.ID))
	r.logger.Manfred(fmt.Sprintf("Project: %s", projectName))

	promptPreview := prompt
	if idx := strings.Index(prompt, "\n"); idx > 0 {
		promptPreview = prompt[:idx]
	}
	if len(promptPreview) > 60 {
		promptPreview = promptPreview[:60] + "..."
	}
	r.logger.Manfred(fmt.Sprintf("Prompt: %s", promptPreview))

	// Create job directories
	if err := job.CreateDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create job directories: %w", err)
	}

	job.Start()

	// Compose project name
	composeProjectName := fmt.Sprintf("manfred_%s", job.ID)
	containerName := docker.ContainerName(composeProjectName, projectConfig.Docker.MainService)

	// Determine compose file path
	repoPath := r.config.ProjectRepositoryPath(projectName)
	composeFile := filepath.Join(repoPath, projectConfig.Docker.ComposeFile)

	// Execute job
	err = r.executeJob(ctx, job, projectConfig, composeProjectName, containerName, composeFile)

	// Cleanup
	r.logger.Docker("Stopping containers...")
	if cleanupErr := r.docker.ComposeDown(ctx, composeFile, composeProjectName); cleanupErr != nil {
		r.logger.Docker(fmt.Sprintf("Warning: cleanup failed: %v", cleanupErr))
	}
	r.logger.Docker("Containers stopped")

	if err != nil {
		job.Fail(err.Error())
		r.logger.Manfred(fmt.Sprintf("Job failed: %s", err))
	} else {
		job.Complete()
		r.logger.Manfred("Job completed successfully")
	}

	return job, nil
}

func (r *Runner) validateProject(name string) (*config.ProjectConfig, error) {
	projectPath := filepath.Join(r.config.ProjectsDir, name)
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project not found: %s", name)
	}

	projectConfig, err := r.config.ProjectConfig(name)
	if err != nil {
		return nil, err
	}

	repoPath := r.config.ProjectRepositoryPath(name)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project repository not found: %s", repoPath)
	}

	return projectConfig, nil
}

func (r *Runner) executeJob(ctx context.Context, job *Job, projectConfig *config.ProjectConfig, composeProjectName, containerName, composeFile string) error {
	// Clone repository if configured
	if projectConfig.Repo != "" {
		if err := r.cloneRepository(ctx, job, projectConfig); err != nil {
			return err
		}
	}

	// Prepare job directory with credentials and prompt
	if err := r.prepareJobDirectory(job); err != nil {
		return err
	}

	// Determine workdir
	workdir := projectConfig.Docker.Workdir
	if _, err := os.Stat(job.WorkspacePath()); err == nil {
		workdir = filepath.Join(docker.ContainerJobPath, "workspace")
	}

	// Start Docker compose
	r.logger.Docker(fmt.Sprintf("Starting docker compose (project: %s)", composeProjectName))

	dockerOut := r.logger.Writer("DOCKER")
	err := r.docker.ComposeUp(ctx, docker.ComposeOptions{
		ComposeFile: composeFile,
		ProjectName: composeProjectName,
		Env: map[string]string{
			"ANTHROPIC_API_KEY": r.config.Credentials.AnthropicAPIKey,
		},
		Volumes: []docker.VolumeMount{
			{
				Source:   job.JobPath(),
				Target:   docker.ContainerJobPath,
				ReadOnly: false,
			},
		},
		Stdout: dockerOut,
		Stderr: dockerOut,
	})
	if err != nil {
		return fmt.Errorf("failed to start compose: %w", err)
	}

	// Wait for container
	r.logger.Docker(fmt.Sprintf("Waiting for container %s to be ready...", containerName))
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := r.docker.WaitForContainer(waitCtx, containerName); err != nil {
		// Try to get more info about what containers exist
		r.logger.Docker("Container not ready, checking docker ps...")
		r.docker.DebugContainers(ctx, composeProjectName, r.logger.Writer("DOCKER"))
		return fmt.Errorf("timeout waiting for container %s: %w", containerName, err)
	}

	// Setup credential symlinks
	if err := r.docker.SetupCredentialSymlinks(ctx, containerName); err != nil {
		r.logger.Docker(fmt.Sprintf("Warning: failed to setup credentials: %v", err))
	}

	r.logger.Docker(fmt.Sprintf("Container %s started", containerName))

	// Phase 1: Run main task
	r.logger.Manfred("Executing Claude Code with prompt...")
	if err := r.execClaude(ctx, containerName, workdir, job.Prompt, false); err != nil {
		return fmt.Errorf("claude execution failed: %w", err)
	}

	// Phase 2: Get commit message
	r.logger.Manfred("Phase 1 complete, requesting commit message...")
	r.logger.Manfred("Requesting commit message from Claude...")
	if err := r.execClaude(ctx, containerName, workdir, CommitMessagePrompt, true); err != nil {
		r.logger.Manfred(fmt.Sprintf("Warning: failed to get commit message: %v", err))
	} else {
		r.readCommitMessage(job)
	}

	// Verify git state
	r.verifyGitState(job)

	// Finalize
	r.finalizeCommit(job)

	return nil
}

func (r *Runner) cloneRepository(ctx context.Context, job *Job, projectConfig *config.ProjectConfig) error {
	r.logger.Docker(fmt.Sprintf("Cloning repository: %s", projectConfig.Repo))

	branchName := fmt.Sprintf("manfred/%s", job.ID)

	// Clone with full history
	cmd := exec.CommandContext(ctx, "git", "clone", projectConfig.Repo, job.WorkspacePath())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Create feature branch
	r.logger.Docker(fmt.Sprintf("Creating branch: %s", branchName))
	cmd = exec.CommandContext(ctx, "git", "-C", job.WorkspacePath(), "checkout", "-b", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Record base SHA
	cmd = exec.CommandContext(ctx, "git", "-C", job.WorkspacePath(), "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get base SHA: %w", err)
	}

	job.BaseSHA = strings.TrimSpace(string(output))
	job.BranchName = branchName

	r.logger.Docker(fmt.Sprintf("Repository cloned, base SHA: %s", job.BaseSHA))
	return nil
}

func (r *Runner) prepareJobDirectory(job *Job) error {
	r.logger.Docker("Preparing job directory...")

	// Copy credentials if they exist
	if r.config.ClaudeCredentialsExist() {
		src := r.config.Credentials.ClaudeCredentialsFile
		dst := job.CredentialsFile()

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read credentials: %w", err)
		}

		if err := os.WriteFile(dst, data, 0600); err != nil {
			return fmt.Errorf("failed to write credentials: %w", err)
		}

		r.logger.Docker("Copied credentials to job directory")
	} else {
		r.logger.Docker(fmt.Sprintf("WARNING: No Claude credentials found at %s", r.config.Credentials.ClaudeCredentialsFile))
	}

	// Write prompt
	if err := os.WriteFile(job.PromptFile(), []byte(job.Prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt: %w", err)
	}

	// Copy Claude bundle
	if err := r.copyClaudeBundle(job); err != nil {
		return fmt.Errorf("failed to copy Claude bundle: %w", err)
	}

	return nil
}

func (r *Runner) copyClaudeBundle(job *Job) error {
	bundleSrc := r.config.Claude.BundlePath
	bundleDst := job.ClaudeBundlePath()

	// Check if bundle source exists
	info, err := os.Stat(bundleSrc)
	if err != nil {
		return fmt.Errorf("Claude bundle not found at %s: %w", bundleSrc, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("Claude bundle path is not a directory: %s", bundleSrc)
	}

	r.logger.Docker(fmt.Sprintf("Copying Claude bundle from %s", bundleSrc))

	// Copy the entire bundle directory
	if err := copyDir(bundleSrc, bundleDst); err != nil {
		return err
	}

	r.logger.Docker("Claude bundle copied to job directory")
	return nil
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file, preserving permissions.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// For symlinks, copy the link itself
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(link, dst)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (r *Runner) execClaude(ctx context.Context, container, workdir, prompt string, continueSession bool) error {
	// Use the bundled Claude binary from the job directory
	claudeBin := filepath.Join(docker.ContainerJobPath, "claude-bundle", "claude")

	args := []string{claudeBin, "--dangerously-skip-permissions"}
	if continueSession {
		args = append(args, "--continue")
	}
	args = append(args, "-p", prompt)

	return r.docker.Exec(ctx, container, args, docker.ExecOptions{
		Workdir: workdir,
		Env: map[string]string{
			"ANTHROPIC_API_KEY": r.config.Credentials.AnthropicAPIKey,
			"IS_SANDBOX":        "1",
		},
		Stdout: r.logger.Writer("CLAUDE"),
		Stderr: r.logger.Writer("CLAUDE"),
	})
}

func (r *Runner) readCommitMessage(job *Job) {
	path := job.CommitMessageFile()
	data, err := os.ReadFile(path)
	if err != nil {
		r.logger.Manfred(fmt.Sprintf("Warning: could not read commit message: %v", err))
		return
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		r.logger.Manfred("Warning: commit message file is empty")
		return
	}

	job.CommitMessage = content
	r.logger.Manfred("Commit message received")
}

func (r *Runner) verifyGitState(job *Job) {
	if job.WorkspacePath() == "" {
		return
	}

	workspace := job.WorkspacePath()
	if _, err := os.Stat(workspace); os.IsNotExist(err) {
		return
	}

	r.logger.Manfred("Verifying git state...")

	// Check current branch
	cmd := exec.Command("git", "-C", workspace, "branch", "--show-current")
	output, err := cmd.Output()
	if err == nil {
		currentBranch := strings.TrimSpace(string(output))
		if job.BranchName != "" && currentBranch != job.BranchName {
			r.logger.Manfred(fmt.Sprintf("WARNING: Branch changed from %s to %s", job.BranchName, currentBranch))
		}
	}

	// Check for uncommitted changes
	cmd = exec.Command("git", "-C", workspace, "status", "--porcelain")
	output, err = cmd.Output()
	if err == nil {
		status := strings.TrimSpace(string(output))
		if status != "" {
			r.logger.Manfred("WARNING: Uncommitted changes remain:")
			lines := strings.Split(status, "\n")
			for i, line := range lines {
				if i >= 5 {
					r.logger.Manfred("  ...")
					break
				}
				r.logger.Manfred(fmt.Sprintf("  %s", line))
			}
		}
	}

	// Log commits made
	if job.BaseSHA != "" {
		cmd = exec.Command("git", "-C", workspace, "log", fmt.Sprintf("%s..HEAD", job.BaseSHA), "--oneline")
		output, err = cmd.Output()
		if err == nil {
			commits := strings.TrimSpace(string(output))
			if commits == "" {
				r.logger.Manfred("No commits made by Claude")
			} else {
				r.logger.Manfred("Commits on branch:")
				for _, line := range strings.Split(commits, "\n") {
					r.logger.Manfred(fmt.Sprintf("  %s", line))
				}
			}
		}
	}
}

func (r *Runner) finalizeCommit(job *Job) {
	r.logger.Separator()
	r.logger.Manfred("FINALIZE (dummy): Would commit with message:")
	r.logger.Blank()

	if job.CommitMessage != "" {
		for _, line := range strings.Split(job.CommitMessage, "\n") {
			r.logger.Manfred(fmt.Sprintf("  %s", line))
		}
	} else {
		r.logger.Manfred("  (no commit message available)")
	}

	r.logger.Blank()
	r.logger.Separator()

	r.logger.Manfred("In production, this would:")
	if job.BranchName != "" {
		r.logger.Manfred(fmt.Sprintf("  1. Push to branch: %s", job.BranchName))
	} else {
		r.logger.Manfred(fmt.Sprintf("  1. Push to branch: manfred/%s", job.ID))
	}
	r.logger.Manfred("  2. Open a Pull Request")
}

// Close releases resources.
func (r *Runner) Close() error {
	return r.docker.Close()
}
