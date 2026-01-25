package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	defaultBaseURL   = "https://api.github.com"
	defaultUserAgent = "manfred/1.0"
)

// Client provides access to the GitHub API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	userAgent  string

	// Rate limiting
	rateMu        sync.Mutex
	rateLimit     *RateLimit
	rateLimitBuf  int // Stop when this many requests remain
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL (for GitHub Enterprise).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithRateLimitBuffer sets how many requests to keep in reserve.
func WithRateLimitBuffer(n int) ClientOption {
	return func(c *Client) {
		c.rateLimitBuf = n
	}
}

// NewClient creates a new GitHub API client.
func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:      defaultBaseURL,
		token:        token,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		userAgent:    defaultUserAgent,
		rateLimitBuf: 100,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// do performs an HTTP request and decodes the response.
func (c *Client) do(ctx context.Context, method, path string, body, result interface{}) error {
	// Check rate limit before making request
	if err := c.checkRateLimit(); err != nil {
		return err
	}

	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Update rate limit from response headers
	c.updateRateLimit(resp)

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if len(respBody) > 0 {
			_ = json.Unmarshal(respBody, apiErr)
		}
		if apiErr.Message == "" {
			apiErr.Message = fmt.Sprintf("GitHub API error: %s", resp.Status)
		}
		return apiErr
	}

	// Decode successful response
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, result)
}

// post performs a POST request.
func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, result)
}

// patch performs a PATCH request.
func (c *Client) patch(ctx context.Context, path string, body, result interface{}) error {
	return c.do(ctx, http.MethodPatch, path, body, result)
}

// delete performs a DELETE request.
func (c *Client) delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// updateRateLimit extracts rate limit info from response headers.
func (c *Client) updateRateLimit(resp *http.Response) {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	limit := resp.Header.Get("X-RateLimit-Limit")
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	reset := resp.Header.Get("X-RateLimit-Reset")

	if limit == "" || remaining == "" || reset == "" {
		return
	}

	l, _ := strconv.Atoi(limit)
	r, _ := strconv.Atoi(remaining)
	rs, _ := strconv.ParseInt(reset, 10, 64)

	c.rateLimit = &RateLimit{
		Limit:     l,
		Remaining: r,
		Reset:     time.Unix(rs, 0),
	}
}

// checkRateLimit returns an error if we're below the buffer threshold.
func (c *Client) checkRateLimit() error {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	if c.rateLimit == nil {
		return nil
	}

	if c.rateLimit.Remaining <= c.rateLimitBuf {
		waitTime := time.Until(c.rateLimit.Reset)
		if waitTime > 0 {
			return &RateLimitError{
				Remaining: c.rateLimit.Remaining,
				Reset:     c.rateLimit.Reset,
			}
		}
	}

	return nil
}

// GetRateLimit returns the current rate limit status.
func (c *Client) GetRateLimit() *RateLimit {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	if c.rateLimit == nil {
		return nil
	}
	rl := *c.rateLimit
	return &rl
}

// RateLimitError is returned when the rate limit buffer is exhausted.
type RateLimitError struct {
	Remaining int
	Reset     time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit nearly exhausted (%d remaining), resets at %s",
		e.Remaining, e.Reset.Format(time.RFC3339))
}

// TestAuth verifies the token is valid by fetching the authenticated user.
func (c *Client) TestAuth(ctx context.Context) (*User, error) {
	var user User
	if err := c.get(ctx, "/user", &user); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	return &user, nil
}
