package job

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the current state of a job.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Job represents a MANFRED job execution.
type Job struct {
	ID          string
	ProjectName string
	Prompt      string
	Status      Status
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string

	// Git-related fields
	BranchName string
	BaseSHA    string

	// Output
	CommitMessage string

	// Paths
	jobsDir string
}

// New creates a new job with a generated ID.
func New(projectName, prompt, jobsDir string) *Job {
	return &Job{
		ID:          generateJobID(),
		ProjectName: projectName,
		Prompt:      prompt,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		jobsDir:     jobsDir,
	}
}

// JobPath returns the path to the job's directory.
func (j *Job) JobPath() string {
	return filepath.Join(j.jobsDir, j.ID)
}

// WorkspacePath returns the path to the job's workspace (cloned repository).
func (j *Job) WorkspacePath() string {
	return filepath.Join(j.JobPath(), "workspace")
}

// CommitMessageFile returns the path to the commit message file.
func (j *Job) CommitMessageFile() string {
	return filepath.Join(j.JobPath(), ".manfred", "commit_message.txt")
}

// PromptFile returns the path to the prompt file.
func (j *Job) PromptFile() string {
	return filepath.Join(j.JobPath(), "prompt.txt")
}

// CredentialsFile returns the path to the credentials file in the job directory.
func (j *Job) CredentialsFile() string {
	return filepath.Join(j.JobPath(), ".credentials.json")
}

// ClaudeBundlePath returns the path to the Claude bundle directory in the job.
func (j *Job) ClaudeBundlePath() string {
	return filepath.Join(j.JobPath(), "claude-bundle")
}

// CreateDirectories creates the job directory structure.
func (j *Job) CreateDirectories() error {
	dirs := []string{
		j.JobPath(),
		filepath.Join(j.JobPath(), ".manfred"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// Start marks the job as running.
func (j *Job) Start() {
	now := time.Now()
	j.StartedAt = &now
	j.Status = StatusRunning
}

// Complete marks the job as completed.
func (j *Job) Complete() {
	now := time.Now()
	j.CompletedAt = &now
	j.Status = StatusCompleted
}

// Fail marks the job as failed with an error message.
func (j *Job) Fail(err string) {
	now := time.Now()
	j.CompletedAt = &now
	j.Status = StatusFailed
	j.Error = err
}

// generateJobID creates a unique job identifier.
func generateJobID() string {
	// Format: job_YYYYMMDD_HHMMSS_xxxx
	now := time.Now()
	timestamp := now.Format("20060102_150405")

	// Generate 4 random hex bytes
	b := make([]byte, 2)
	rand.Read(b)
	suffix := hex.EncodeToString(b)

	return fmt.Sprintf("job_%s_%s", timestamp, suffix)
}
