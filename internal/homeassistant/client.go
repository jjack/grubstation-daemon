package homeassistant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jjack/remote-boot-agent/internal/config"
)

type HAPayload struct {
	MACAddress string   `json:"mac_address"`
	Hostname   string   `json:"hostname"`
	Bootloader string   `json:"bootloader"`
	OSList     []string `json:"os_list"`
}

type Client struct {
	BaseURL string
	Token   string
}

func NewClient(cfg config.HAConfig) *Client {
	return &Client{
		BaseURL: strings.TrimRight(cfg.BaseURL, "/"),
		Token:   cfg.Token,
	}
}

func (c *Client) GetSelectedOS(mac string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/remote_boot_manager/%s", c.BaseURL, mac)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error communicating with Home Assistant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error reading response: %w", err)
		}
		return string(body), nil
	}
	return "", fmt.Errorf("received HTTP %d from Home Assistant", resp.StatusCode)
}

func (c *Client) PushAvailableOSes(payload HAPayload) error {
	endpoint := fmt.Sprintf("%s/api/webhook/remote_boot_manager_ingest", c.BaseURL)
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error posting to Home Assistant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("received HTTP %d from Home Assistant", resp.StatusCode)
}

