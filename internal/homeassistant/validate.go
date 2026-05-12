package homeassistant

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

const WebHookMaxLength = 64

var (
	ErrWebhookIDEmpty       = errors.New("webhook id cannot be empty")
	ErrWebhookIDInvalidChar = errors.New("webhook id can only contain letters, numbers, hyphens, and underscores")
	ErrWebhookIDWrongSize   = errors.New("webhook id should be 64 characters long")
	ErrURLEmpty             = errors.New("home assistant url cannot be empty")
	ErrHTTPSUnsupported     = errors.New("https is not supported by grub; please use an http:// url")
)

func ValidateWebhookID(webhookID string) error {
	if webhookID == "" {
		return ErrWebhookIDEmpty
	}

	if len(webhookID) != WebHookMaxLength {
		return ErrWebhookIDWrongSize
	}

	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(webhookID) {
		return ErrWebhookIDInvalidChar
	}

	return nil
}

func ValidateURL(hassURL string) error {
	if hassURL == "" {
		return ErrURLEmpty
	}

	u, err := url.ParseRequestURI(hassURL)
	if err != nil {
		return fmt.Errorf("invalid home assistant url: %w", err)
	}
	if u.Scheme == "https" {
		return ErrHTTPSUnsupported
	}

	return nil
}
