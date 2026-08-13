package adapter

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// NtfyNotificationService implements service.NotificationService using ntfy (https://ntfy.sh).
// Configure a private topic by setting a token obtained from ntfy.sh/account.
type NtfyNotificationService struct {
	serverURL  string
	topic      string
	token      string // optional; required for protected topics
	httpClient *http.Client
}

// NtfyConfig holds configuration for NtfyNotificationService.
type NtfyConfig struct {
	// ServerURL is the ntfy server base URL (e.g. "https://ntfy.sh").
	ServerURL string
	// Topic is the ntfy topic to publish to.
	Topic string
	// Token is the optional Bearer token for protected topics (ntfy access token).
	Token string
}

// NewNtfyNotificationService creates a new NtfyNotificationService.
func NewNtfyNotificationService(cfg NtfyConfig) *NtfyNotificationService {
	return &NtfyNotificationService{
		serverURL:  strings.TrimRight(cfg.ServerURL, "/"),
		topic:      cfg.Topic,
		token:      cfg.Token,
		httpClient: &http.Client{},
	}
}

// Notify sends a notification to the configured ntfy topic.
// The title is sent via the X-Title header; the message body is the request body.
func (s *NtfyNotificationService) Notify(ctx context.Context, title, message string) error {
	url := fmt.Sprintf("%s/%s", s.serverURL, s.topic)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("creating ntfy request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Title", title)
	req.Header.Set("X-Priority", "default")

	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending ntfy notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned unexpected status %d", resp.StatusCode)
	}

	return nil
}
