package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_TestAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Reset", "1704067200")

		json.NewEncoder(w).Encode(User{
			Login:   "testuser",
			ID:      12345,
			HTMLURL: "https://github.com/testuser",
		})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))

	user, err := client.TestAuth(context.Background())
	if err != nil {
		t.Fatalf("TestAuth() error = %v", err)
	}

	if user.Login != "testuser" {
		t.Errorf("Login = %q, want %q", user.Login, "testuser")
	}
	if user.ID != 12345 {
		t.Errorf("ID = %d, want %d", user.ID, 12345)
	}

	// Check rate limit was captured
	rl := client.GetRateLimit()
	if rl == nil {
		t.Fatal("expected rate limit to be set")
	}
	if rl.Limit != 5000 {
		t.Errorf("RateLimit.Limit = %d, want %d", rl.Limit, 5000)
	}
	if rl.Remaining != 4999 {
		t.Errorf("RateLimit.Remaining = %d, want %d", rl.Remaining, 4999)
	}
}

func TestClient_GetIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/issues/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(Issue{
			Number: 42,
			Title:  "Test Issue",
			Body:   "Issue body",
			State:  "open",
		})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))

	issue, err := client.GetIssue(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	if issue.Number != 42 {
		t.Errorf("Number = %d, want %d", issue.Number, 42)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("Title = %q, want %q", issue.Title, "Test Issue")
	}
}

func TestClient_AddIssueComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/issues/42/comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["body"] != "Test comment" {
			t.Errorf("unexpected body: %v", body)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Comment{
			ID:   123,
			Body: "Test comment",
		})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))

	comment, err := client.AddIssueComment(context.Background(), "owner", "repo", 42, "Test comment")
	if err != nil {
		t.Fatalf("AddIssueComment() error = %v", err)
	}

	if comment.ID != 123 {
		t.Errorf("ID = %d, want %d", comment.ID, 123)
	}
}

func TestClient_CreatePullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/pulls" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var input CreatePullRequestInput
		json.NewDecoder(r.Body).Decode(&input)

		if input.Title != "Test PR" {
			t.Errorf("Title = %q, want %q", input.Title, "Test PR")
		}
		if input.Head != "feature-branch" {
			t.Errorf("Head = %q, want %q", input.Head, "feature-branch")
		}
		if input.Base != "main" {
			t.Errorf("Base = %q, want %q", input.Base, "main")
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(PullRequest{
			Number: 1,
			Title:  input.Title,
			State:  "open",
		})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))

	pr, err := client.CreatePullRequest(context.Background(), "owner", "repo", &CreatePullRequestInput{
		Title: "Test PR",
		Body:  "PR body",
		Head:  "feature-branch",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}

	if pr.Number != 1 {
		t.Errorf("Number = %d, want %d", pr.Number, 1)
	}
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Not Found",
		})
	}))
	defer server.Close()

	client := NewClient("test-token", WithBaseURL(server.URL))

	_, err := client.GetIssue(context.Background(), "owner", "repo", 999)
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, 404)
	}
	if apiErr.Message != "Not Found" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Not Found")
	}
}

func TestClient_RateLimitError(t *testing.T) {
	callCount := 0
	// Use a reset time 1 hour in the future
	futureReset := time.Now().Add(time.Hour).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Return very low remaining count
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "5")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", futureReset))

		json.NewEncoder(w).Encode(User{Login: "test"})
	}))
	defer server.Close()

	// Set buffer to 10, so with 5 remaining we should get an error
	client := NewClient("test-token",
		WithBaseURL(server.URL),
		WithRateLimitBuffer(10),
	)

	// First call succeeds (no rate limit info yet)
	_, err := client.TestAuth(context.Background())
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call should fail due to rate limit
	_, err = client.TestAuth(context.Background())
	if err == nil {
		t.Fatal("expected rate limit error")
	}

	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Errorf("expected *RateLimitError, got %T: %v", err, err)
	}

	// Should have only made 1 call
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}
