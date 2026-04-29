package homeassistant

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

var (
	ErrWebhookIDEmpty       = errors.New("webhook id cannot be empty")
	ErrWebhookIDInvalidChar = errors.New("webhook id can only contain letters, numbers, hyphens, and underscores")
	ErrWebhookIDTooLong     = errors.New("webhook id cannot be longer than 255 characters")
	ErrURLEmpty             = errors.New("home assistant url cannot be empty")
)

func ValidateWebhookID(webhookID string) error {
	if webhookID == "" {
		return ErrWebhookIDEmpty
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(webhookID) {
		return ErrWebhookIDInvalidChar
	}
	if len(webhookID) > 255 {
		return ErrWebhookIDTooLong
	}

	return nil
}

func ValidateURL(hassURL string) error {
	if hassURL == "" {
		return ErrURLEmpty
	}

	_, err := url.ParseRequestURI(hassURL)
	if err != nil {
		return fmt.Errorf("invalid home assistant url: %w", err)
	}

	return nil
}
