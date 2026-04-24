package config

import (
	"fmt"
	"net"
	"regexp"
)

func ValidateMACAddress(v string) error {
	if v == "" {
		return fmt.Errorf("mac address cannot be empty")
	}
	_, err := net.ParseMAC(v)
	if err != nil {
		return fmt.Errorf("invalid MAC address format")
	}
	return nil
}

func ValidateHostname(v string) error {
	if v == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9-.]+$`).MatchString(v) {
		return fmt.Errorf("hostname can only contain letters, numbers, hyphens, and periods")
	}
	return nil
}
