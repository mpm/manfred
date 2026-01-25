package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidSignature = errors.New("invalid webhook signature")
	ErrMissingSignature = errors.New("missing webhook signature")
)

// ValidateWebhookSignature verifies the HMAC SHA-256 signature of a webhook payload.
// The signature header should be in the format "sha256=<hex-digest>".
func ValidateWebhookSignature(payload []byte, signature, secret string) error {
	if signature == "" {
		return ErrMissingSignature
	}

	if !strings.HasPrefix(signature, "sha256=") {
		return ErrInvalidSignature
	}

	sig := strings.TrimPrefix(signature, "sha256=")
	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		return ErrInvalidSignature
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)

	if !hmac.Equal(sigBytes, expectedMAC) {
		return ErrInvalidSignature
	}

	return nil
}

// WebhookEvent represents a parsed GitHub webhook event.
type WebhookEvent struct {
	Type    string          // Event type from X-GitHub-Event header
	Action  string          // Action field from payload
	Payload json.RawMessage // Raw payload for further parsing
}

// ParseWebhookEvent parses a webhook payload into a typed event.
func ParseWebhookEvent(eventType string, payload []byte) (*WebhookEvent, error) {
	event := &WebhookEvent{
		Type:    eventType,
		Payload: payload,
	}

	// Extract action field if present
	var base struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(payload, &base); err == nil {
		event.Action = base.Action
	}

	return event, nil
}

// IssueEvent represents an issue webhook event.
type IssueEvent struct {
	Action string `json:"action"` // "opened", "edited", "closed", "labeled", etc.
	Issue  Issue  `json:"issue"`
	Label  *Label `json:"label,omitempty"` // For "labeled"/"unlabeled" actions
	Repo   Repo   `json:"repository"`
	Sender User   `json:"sender"`
}

// IssueCommentEvent represents an issue_comment webhook event.
type IssueCommentEvent struct {
	Action  string  `json:"action"` // "created", "edited", "deleted"
	Issue   Issue   `json:"issue"`
	Comment Comment `json:"comment"`
	Repo    Repo    `json:"repository"`
	Sender  User    `json:"sender"`
}

// PullRequestEvent represents a pull_request webhook event.
type PullRequestEvent struct {
	Action      string      `json:"action"` // "opened", "closed", "synchronize", etc.
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
	Repo        Repo        `json:"repository"`
	Sender      User        `json:"sender"`
}

// PullRequestReviewEvent represents a pull_request_review webhook event.
type PullRequestReviewEvent struct {
	Action      string        `json:"action"` // "submitted", "edited", "dismissed"
	Review      Review        `json:"review"`
	PullRequest PullRequest   `json:"pull_request"`
	Repo        Repo          `json:"repository"`
	Sender      User          `json:"sender"`
}

// Review represents a pull request review.
type Review struct {
	ID        int64  `json:"id"`
	User      User   `json:"user"`
	Body      string `json:"body"`
	State     string `json:"state"` // "approved", "changes_requested", "commented"
	HTMLURL   string `json:"html_url"`
}

// PullRequestReviewCommentEvent represents a pull_request_review_comment webhook event.
type PullRequestReviewCommentEvent struct {
	Action      string        `json:"action"` // "created", "edited", "deleted"
	Comment     ReviewComment `json:"comment"`
	PullRequest PullRequest   `json:"pull_request"`
	Repo        Repo          `json:"repository"`
	Sender      User          `json:"sender"`
}

// ParseAs parses the webhook payload into a specific event type.
func (e *WebhookEvent) ParseAs(v interface{}) error {
	return json.Unmarshal(e.Payload, v)
}

// AsIssueEvent parses the event as an IssueEvent.
func (e *WebhookEvent) AsIssueEvent() (*IssueEvent, error) {
	if e.Type != "issues" {
		return nil, fmt.Errorf("expected issues event, got %s", e.Type)
	}
	var ie IssueEvent
	if err := e.ParseAs(&ie); err != nil {
		return nil, err
	}
	return &ie, nil
}

// AsIssueCommentEvent parses the event as an IssueCommentEvent.
func (e *WebhookEvent) AsIssueCommentEvent() (*IssueCommentEvent, error) {
	if e.Type != "issue_comment" {
		return nil, fmt.Errorf("expected issue_comment event, got %s", e.Type)
	}
	var ice IssueCommentEvent
	if err := e.ParseAs(&ice); err != nil {
		return nil, err
	}
	return &ice, nil
}

// AsPullRequestEvent parses the event as a PullRequestEvent.
func (e *WebhookEvent) AsPullRequestEvent() (*PullRequestEvent, error) {
	if e.Type != "pull_request" {
		return nil, fmt.Errorf("expected pull_request event, got %s", e.Type)
	}
	var pre PullRequestEvent
	if err := e.ParseAs(&pre); err != nil {
		return nil, err
	}
	return &pre, nil
}

// AsPullRequestReviewEvent parses the event as a PullRequestReviewEvent.
func (e *WebhookEvent) AsPullRequestReviewEvent() (*PullRequestReviewEvent, error) {
	if e.Type != "pull_request_review" {
		return nil, fmt.Errorf("expected pull_request_review event, got %s", e.Type)
	}
	var prre PullRequestReviewEvent
	if err := e.ParseAs(&prre); err != nil {
		return nil, err
	}
	return &prre, nil
}

// AsPullRequestReviewCommentEvent parses the event as a PullRequestReviewCommentEvent.
func (e *WebhookEvent) AsPullRequestReviewCommentEvent() (*PullRequestReviewCommentEvent, error) {
	if e.Type != "pull_request_review_comment" {
		return nil, fmt.Errorf("expected pull_request_review_comment event, got %s", e.Type)
	}
	var prrce PullRequestReviewCommentEvent
	if err := e.ParseAs(&prrce); err != nil {
		return nil, err
	}
	return &prrce, nil
}
