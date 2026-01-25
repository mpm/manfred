package github

import (
	"context"
	"fmt"
)

// CreatePullRequest creates a new pull request.
func (c *Client) CreatePullRequest(ctx context.Context, owner, repo string, input *CreatePullRequestInput) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
	var pr PullRequest
	if err := c.post(ctx, path, input, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// GetPullRequest fetches a pull request by number.
func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	var pr PullRequest
	if err := c.get(ctx, path, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// GetPRComments fetches issue comments on a pull request.
// Note: PRs have two types of comments - issue comments and review comments.
// This fetches issue comments (general discussion).
func (c *Client) GetPRComments(ctx context.Context, owner, repo string, number int) ([]Comment, error) {
	// PR comments use the issues endpoint
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	var comments []Comment
	if err := c.get(ctx, path, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// GetPRReviewComments fetches review comments on a pull request.
// These are comments on specific lines of code.
func (c *Client) GetPRReviewComments(ctx context.Context, owner, repo string, number int) ([]ReviewComment, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number)
	var comments []ReviewComment
	if err := c.get(ctx, path, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// AddPRComment adds a general comment to a pull request.
func (c *Client) AddPRComment(ctx context.Context, owner, repo string, number int, body string) (*Comment, error) {
	// PR general comments use the issues endpoint
	return c.AddIssueComment(ctx, owner, repo, number, body)
}

// UpdatePullRequest updates a pull request.
func (c *Client) UpdatePullRequest(ctx context.Context, owner, repo string, number int, update *UpdatePullRequestInput) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	var pr PullRequest
	if err := c.patch(ctx, path, update, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// UpdatePullRequestInput contains fields for updating a pull request.
type UpdatePullRequestInput struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	State string `json:"state,omitempty"` // "open" or "closed"
	Base  string `json:"base,omitempty"`
}

// ListPullRequests lists pull requests for a repository.
func (c *Client) ListPullRequests(ctx context.Context, owner, repo string, opts *ListPullRequestsOptions) ([]PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
	if opts != nil {
		path += opts.queryString()
	}
	var prs []PullRequest
	if err := c.get(ctx, path, &prs); err != nil {
		return nil, err
	}
	return prs, nil
}

// ListPullRequestsOptions contains options for listing pull requests.
type ListPullRequestsOptions struct {
	State     string // "open", "closed", "all"
	Head      string // Filter by head branch ("owner:ref" or "ref")
	Base      string // Filter by base branch
	Sort      string // "created", "updated", "popularity", "long-running"
	Direction string // "asc" or "desc"
}

func (o *ListPullRequestsOptions) queryString() string {
	if o == nil {
		return ""
	}
	params := ""
	sep := "?"
	if o.State != "" {
		params += sep + "state=" + o.State
		sep = "&"
	}
	if o.Head != "" {
		params += sep + "head=" + o.Head
		sep = "&"
	}
	if o.Base != "" {
		params += sep + "base=" + o.Base
		sep = "&"
	}
	if o.Sort != "" {
		params += sep + "sort=" + o.Sort
		sep = "&"
	}
	if o.Direction != "" {
		params += sep + "direction=" + o.Direction
	}
	return params
}

// IsPRMerged checks if a pull request has been merged.
func (c *Client) IsPRMerged(ctx context.Context, owner, repo string, number int) (bool, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number)
	err := c.get(ctx, path, nil)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
