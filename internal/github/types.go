// Package github provides a client for GitHub API operations.
package github

import "time"

// Issue represents a GitHub issue.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"` // "open" or "closed"
	User      User      `json:"user"`
	Labels    []Label   `json:"labels"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	HTMLURL   string    `json:"html_url"`
}

// Comment represents a GitHub issue or PR comment.
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	User      User      `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	HTMLURL   string    `json:"html_url"`
}

// ReviewComment represents a GitHub PR review comment (on a specific line).
type ReviewComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	Path      string    `json:"path"`
	Line      *int      `json:"line"`
	User      User      `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	HTMLURL   string    `json:"html_url"`
	DiffHunk  string    `json:"diff_hunk"`
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"` // "open", "closed"
	Merged    bool      `json:"merged"`
	User      User      `json:"user"`
	Head      GitRef    `json:"head"`
	Base      GitRef    `json:"base"`
	Labels    []Label   `json:"labels"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
	HTMLURL   string    `json:"html_url"`
}

// CreatePullRequestInput contains fields for creating a pull request.
type CreatePullRequestInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"` // Branch name or "owner:branch"
	Base  string `json:"base"` // Target branch
	Draft bool   `json:"draft,omitempty"`
}

// User represents a GitHub user.
type User struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

// Label represents a GitHub label.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// GitRef represents a git reference (branch) in a PR.
type GitRef struct {
	Ref  string `json:"ref"`  // Branch name
	SHA  string `json:"sha"`
	Repo *Repo  `json:"repo"`
}

// Repo represents a GitHub repository.
type Repo struct {
	Owner    User   `json:"owner"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	SSHURL   string `json:"ssh_url"`
	HTMLURL  string `json:"html_url"`
}

// RateLimit represents GitHub API rate limit information.
type RateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
}

// APIError represents a GitHub API error response.
type APIError struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
	StatusCode       int    `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}
