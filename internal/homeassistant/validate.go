package homeassistant

import (
	"fmt"
	"net/url"
	"regexp"
)

func ValidateWebhookID(webhookID string) error {
	if webhookID == "" {
		return fmt.Errorf("webhook id cannot be empty")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(webhookID) {
		return fmt.Errorf("webhook id can only contain letters, numbers, and underscores")
	}
	if len(webhookID) > 255 {
		return fmt.Errorf("webhook id cannot be longer than 255 characters")
	}

	return nil
}

func ValidateURL(hassURL string) error {
	if hassURL == "" {
		return fmt.Errorf("home assistant url cannot be empty")
	}

	_, err := url.ParseRequestURI(hassURL)
	if err != nil {
		return fmt.Errorf("invalid home assistant url: %w", err)
	}

	return nil
}
