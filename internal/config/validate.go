package config

import (
	"errors"
	"log/slog"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
)

var (
	ErrBootloaderConfigPathEmpty    = errors.New("bootloader config path cannot be empty")
	ErrBootloaderConfigPathNotExist = errors.New("bootloader config path does not exist")
	ErrAddressEmpty                 = errors.New("address cannot be empty")
	ErrInvalidBroadcastAddress      = errors.New("invalid WOL address: must be a valid IP address")
	ErrInvalidBroadcastPort         = errors.New("invalid WOL port: must be a number between 1 and 65535")
	ErrInvalidHost                  = errors.New("host must be a valid IP address or hostname (letters, numbers, hyphens, dots)")
	ErrInvalidMACAddress            = errors.New("invalid MAC address format")
	ErrInvalidURL                   = errors.New("invalid URL format")
	ErrMACAddressEmpty              = errors.New("mac address cannot be empty")
	ErrNameEmpty                    = errors.New("name cannot be empty")
	ErrURLEmpty                     = errors.New("url cannot be empty")
	ErrWebhookIDEmpty               = errors.New("webhook id cannot be empty")
	ErrWebhookIDInvalidChar         = errors.New("webhook id can only contain letters, numbers, hyphens, and underscores")
)

func ValidateBootloaderConfigPath(v string) error {
	if v == "" {
		return ErrBootloaderConfigPathEmpty
	}
	if _, err := os.Stat(v); os.IsNotExist(err) {
		return ErrBootloaderConfigPathNotExist
	}
	return nil
}

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

func ValidateBroadcastAddress(v string) error {
	// empty means use the default address - 255.255.255.255
	if v == "" {
		return nil
	}
	if net.ParseIP(v) == nil {
		return ErrInvalidBroadcastAddress
	}
	return nil
}

func ValidateBroadcastPort(v string) error {
	// empty means use the default port - 9
	if v == "" {
		return nil
	}
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Error("Invalid WOL port", "port", port)
		return ErrInvalidBroadcastPort
	}
	return nil
}

func (c *Config) Validate() error {
	if err := ValidateMACAddress(c.Host.MACAddress); err != nil {
		return err
	}
	if err := ValidateName(c.Host.Name); err != nil {
		return err
	}
	if err := ValidateHost(c.Host.Address); err != nil {
		return err
	}
	if err := ValidateURL(c.HomeAssistant.URL); err != nil {
		return err
	}
	if err := ValidateWebhookID(c.HomeAssistant.WebhookID); err != nil {
		return err
	}
	portStr := ""
	if c.Host.BroadcastPort != 0 {
		portStr = strconv.Itoa(c.Host.BroadcastPort)
	}
	if err := ValidateBroadcastPort(portStr); err != nil {
		return err
	}
	if err := ValidateBroadcastAddress(c.Host.BroadcastAddress); err != nil {
		return err
	}
	return nil
}

func ValidateName(v string) error {
	if v == "" {
		return ErrNameEmpty
	}
	return nil
}

// hosts can be IP addresses or hostnames, but must not be empty and must only contain valid characters
func ValidateHost(v string) error {
	if v == "" {
		return ErrAddressEmpty
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
