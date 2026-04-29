package config

import (
	"errors"
	"log/slog"
	"net"
	"net/url"
	"regexp"
	"strconv"
)

var (
	ErrMACAddressEmpty      = errors.New("mac address cannot be empty")
	ErrInvalidMACAddress    = errors.New("invalid MAC address format")
	ErrURLEmpty             = errors.New("url cannot be empty")
	ErrInvalidURL           = errors.New("invalid URL format")
	ErrWebhookIDEmpty       = errors.New("webhook id cannot be empty")
	ErrWebhookIDInvalidChar = errors.New("webhook id can only contain letters, numbers, hyphens, and underscores")
	ErrBroadcastPortEmpty   = errors.New("WOL port cannot be empty")
	ErrInvalidBroadcastPort = errors.New("invalid WOL port: must be a number between 1 and 65535")
	ErrHostnameEmpty        = errors.New("hostname cannot be empty")
	ErrInvalidHostname      = errors.New("hostname can only contain letters, numbers, hyphens, and periods")
)

func ValidateMACAddress(v string) error {
	if v == "" {
		return ErrMACAddressEmpty
	}
	_, err := net.ParseMAC(v)
	if err != nil {
		return ErrInvalidMACAddress
	}
	return nil
}

func ValidateURL(v string) error {
	if v == "" {
		return ErrURLEmpty
	}
	u, err := url.ParseRequestURI(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrInvalidURL
	}
	return nil
}

func ValidateWebhookID(v string) error {
	if v == "" {
		return ErrWebhookIDEmpty
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(v) {
		return ErrWebhookIDInvalidChar
	}
	return nil
}

func ValidateBroadcastPort(v string) error {
	if v == "" {
		return ErrBroadcastPortEmpty
	}
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Debug("Invalid WOL port", "port", port)
		return ErrInvalidBroadcastPort
	}
	return nil
}

func (c *Config) Validate() error {
	if err := ValidateMACAddress(c.Host.MACAddress); err != nil {
		return err
	}
	if err := ValidateHostname(c.Host.Hostname); err != nil {
		return err
	}
	if err := ValidateURL(c.HomeAssistant.URL); err != nil {
		return err
	}
	if err := ValidateWebhookID(c.HomeAssistant.WebhookID); err != nil {
		return err
	}
	if err := ValidateBroadcastPort(strconv.Itoa(c.Host.BroadcastPort)); err != nil {
		return err
	}
	return nil
}

func ValidateHostname(v string) error {
	if v == "" {
		return ErrHostnameEmpty
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString(v) {
		return ErrInvalidHostname
	}
	return nil
}
