package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidateWebhookSignature(t *testing.T) {
	secret := "my-secret-key"
	payload := []byte(`{"action": "opened"}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		wantErr   error
	}{
		{
			name:      "valid signature",
			payload:   payload,
			signature: validSig,
			secret:    secret,
			wantErr:   nil,
		},
		{
			name:      "missing signature",
			payload:   payload,
			signature: "",
			secret:    secret,
			wantErr:   ErrMissingSignature,
		},
		{
			name:      "invalid prefix",
			payload:   payload,
			signature: "sha1=abc123",
			secret:    secret,
			wantErr:   ErrInvalidSignature,
		},
		{
			name:      "wrong secret",
			payload:   payload,
			signature: validSig,
			secret:    "wrong-secret",
			wantErr:   ErrInvalidSignature,
		},
		{
			name:      "modified payload",
			payload:   []byte(`{"action": "closed"}`),
			signature: validSig,
			secret:    secret,
			wantErr:   ErrInvalidSignature,
		},
		{
			name:      "invalid hex",
			payload:   payload,
			signature: "sha256=notvalidhex",
			secret:    secret,
			wantErr:   ErrInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookSignature(tt.payload, tt.signature, tt.secret)
			if err != tt.wantErr {
				t.Errorf("ValidateWebhookSignature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseWebhookEvent(t *testing.T) {
	payload := []byte(`{"action": "opened", "issue": {"number": 1}}`)

	event, err := ParseWebhookEvent("issues", payload)
	if err != nil {
		t.Fatalf("ParseWebhookEvent() error = %v", err)
	}

	if event.Type != "issues" {
		t.Errorf("Type = %q, want %q", event.Type, "issues")
	}
	if event.Action != "opened" {
		t.Errorf("Action = %q, want %q", event.Action, "opened")
	}
}

func TestWebhookEventAsIssueEvent(t *testing.T) {
	payload := []byte(`{
		"action": "opened",
		"issue": {
			"number": 42,
			"title": "Test Issue",
			"body": "Issue body",
			"state": "open"
		},
		"repository": {
			"name": "test-repo",
			"full_name": "owner/test-repo"
		},
		"sender": {
			"login": "testuser"
		}
	}`)

	event, err := ParseWebhookEvent("issues", payload)
	if err != nil {
		t.Fatalf("ParseWebhookEvent() error = %v", err)
	}

	ie, err := event.AsIssueEvent()
	if err != nil {
		t.Fatalf("AsIssueEvent() error = %v", err)
	}

	if ie.Action != "opened" {
		t.Errorf("Action = %q, want %q", ie.Action, "opened")
	}
	if ie.Issue.Number != 42 {
		t.Errorf("Issue.Number = %d, want %d", ie.Issue.Number, 42)
	}
	if ie.Issue.Title != "Test Issue" {
		t.Errorf("Issue.Title = %q, want %q", ie.Issue.Title, "Test Issue")
	}
	if ie.Repo.Name != "test-repo" {
		t.Errorf("Repo.Name = %q, want %q", ie.Repo.Name, "test-repo")
	}
	if ie.Sender.Login != "testuser" {
		t.Errorf("Sender.Login = %q, want %q", ie.Sender.Login, "testuser")
	}
}

func TestWebhookEventAsIssueCommentEvent(t *testing.T) {
	payload := []byte(`{
		"action": "created",
		"issue": {
			"number": 42
		},
		"comment": {
			"id": 123,
			"body": "@claude approved"
		},
		"repository": {
			"name": "test-repo"
		},
		"sender": {
			"login": "reviewer"
		}
	}`)

	event, err := ParseWebhookEvent("issue_comment", payload)
	if err != nil {
		t.Fatalf("ParseWebhookEvent() error = %v", err)
	}

	ice, err := event.AsIssueCommentEvent()
	if err != nil {
		t.Fatalf("AsIssueCommentEvent() error = %v", err)
	}

	if ice.Action != "created" {
		t.Errorf("Action = %q, want %q", ice.Action, "created")
	}
	if ice.Comment.Body != "@claude approved" {
		t.Errorf("Comment.Body = %q, want %q", ice.Comment.Body, "@claude approved")
	}
}

func TestWebhookEventWrongType(t *testing.T) {
	payload := []byte(`{"action": "opened"}`)

	event, _ := ParseWebhookEvent("issues", payload)

	_, err := event.AsIssueCommentEvent()
	if err == nil {
		t.Error("expected error for wrong event type")
	}
}
