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
	ErrHostEmpty            = errors.New("host cannot be empty")
	ErrInvalidHost          = errors.New("host must be a valid IP address or hostname (letters, numbers, hyphens, dots)")
	ErrEntityTypeEmpty      = errors.New("entity type cannot be empty")
	ErrInvalidEntityType    = errors.New("entity type must be either 'button' or 'switch'")
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
		slog.Error("Invalid WOL port", "port", port)
		return ErrInvalidBroadcastPort
	}
	return nil
}

func ValidateEntityType(v EntityType) error {
	if v == "" {
		return ErrEntityTypeEmpty
	}
	if v != EntityTypeButton && v != EntityTypeSwitch {
		return ErrInvalidEntityType
	}
	return nil
}

func (c *Config) Validate() error {
	if err := ValidateMACAddress(c.Server.MACAddress); err != nil {
		return err
	}
	if err := ValidateHost(c.Server.Host); err != nil {
		return err
	}
	if err := ValidateEntityType(c.HomeAssistant.EntityType); err != nil {
		return err
	}
	if err := ValidateURL(c.HomeAssistant.URL); err != nil {
		return err
	}
	if err := ValidateWebhookID(c.HomeAssistant.WebhookID); err != nil {
		return err
	}
	if err := ValidateBroadcastPort(strconv.Itoa(c.Server.BroadcastPort)); err != nil {
		return err
	}
	return nil
}

// hosts can be IP addresses or hostnames, but must not be empty and must only contain valid characters
func ValidateHost(v string) error {
	if v == "" {
		return ErrHostEmpty
	}

	// it's a valid ip
	if net.ParseIP(v) != nil {
		return nil
	}

	// it's a valid hostname
	if regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString(v) {
		return nil
	}
	return ErrInvalidHost
}
