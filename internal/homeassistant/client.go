package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const HTTP_CLIENT_TIMEOUT = 10 * time.Second

type Client struct {
	BaseURL    string
	WebhookID  string
	HTTPClient *http.Client
}

type PushPayload struct {
	MACAddress       string   `json:"mac"`
	BroadcastAddress string   `json:"broadcast_address,omitempty"`
	BroadcastPort    int      `json:"broadcast_port,omitempty"`
	Name             string   `json:"name"`
	Address          string   `json:"address"`
	Bootloader       string   `json:"bootloader"`
	BootOptions      []string `json:"boot_options"`
}

func NewClient(baseURL, webhookID string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: HTTP_CLIENT_TIMEOUT}
	}
	return &Client{
		BaseURL:    baseURL,
		WebhookID:  webhookID,
		HTTPClient: httpClient,
	}
}

func (c *Client) Push(ctx context.Context, payload PushPayload) error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base url: %w", err)
	}
	targetURL := u.JoinPath("api/webhook", c.WebhookID).String()

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal push payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to home assistant failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code received from home assistant: %d", resp.StatusCode)
	}

	return nil
}
