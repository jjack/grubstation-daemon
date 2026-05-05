//go:build ignore

package bootloader

import "context"

const exampleBootloader = "example"

// Example represents a sample implementation of the Bootloader interface.
// It serves as a template for developers looking to add support for new
// bootloaders (e.g., systemd-boot, rEFInd, etc.).
type Example struct{}

// NewExample creates a new instance of the Example bootloader.
func NewExample() Bootloader {
	return &Example{}
}

// IsActive should return true if this bootloader is the one currently managing the system.
// For example, by checking for the existence of specific EFI variables or configuration directories.
func (s *Example) IsActive(ctx context.Context) bool {
	// you should implement your own logic here to determine if this bootloader is active
	return true
}

// GetBootOptions should parse the bootloader's configuration file and return a list of
// available boot entries (Operating Systems) that the user can choose from.
func (s *Example) GetBootOptions(ctx context.Context, cfg Config) ([]string, error) {
	return []string{"Ubuntu", "Windows"}, nil
}

// Name returns the unique string identifier for this bootloader integration.
func (s *Example) Name() string {
	return exampleBootloader
}

// Install should perform the necessary steps to configure the bootloader to fetch
// its next boot target from the Home Assistant webhook.
func (s *Example) Setup(ctx context.Context, macAddress, haURL, webhookID string) error {
	return nil
}

// DiscoverConfigPath should attempt to automatically locate the bootloader's primary
// configuration file on the host filesystem (e.g., /boot/grub/grub.cfg).
func (s *Example) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "/path/to/example.cfg", nil
}
