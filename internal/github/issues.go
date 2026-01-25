package github

import (
	"context"
	"fmt"
)

// GetIssue fetches an issue by number.
func (c *Client) GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number)
	var issue Issue
	if err := c.get(ctx, path, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// GetIssueComments fetches all comments on an issue.
func (c *Client) GetIssueComments(ctx context.Context, owner, repo string, number int) ([]Comment, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	var comments []Comment
	if err := c.get(ctx, path, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// AddIssueComment adds a comment to an issue.
func (c *Client) AddIssueComment(ctx context.Context, owner, repo string, number int, body string) (*Comment, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	input := map[string]string{"body": body}
	var comment Comment
	if err := c.post(ctx, path, input, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// AddLabel adds a label to an issue or PR.
func (c *Client) AddLabel(ctx context.Context, owner, repo string, number int, label string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, number)
	input := []string{label}
	return c.post(ctx, path, input, nil)
}

// RemoveLabel removes a label from an issue or PR.
func (c *Client) RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%s", owner, repo, number, label)
	return c.delete(ctx, path)
}

// ListIssueLabels returns all labels on an issue.
func (c *Client) ListIssueLabels(ctx context.Context, owner, repo string, number int) ([]Label, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, number)
	var labels []Label
	if err := c.get(ctx, path, &labels); err != nil {
		return nil, err
	}
	return labels, nil
}
