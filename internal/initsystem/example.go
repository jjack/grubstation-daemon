//go:build ignore

package initsystem

import "context"

const exampleInitSystem = "example"

// Example represents a sample implementation of the InitSystem interface.
// It serves as a template for developers looking to add support for new
// init systems (e.g., OpenRC, SysVinit, upstart, etc.).
type Example struct{}

// NewExample creates a new instance of the Example init system.
func NewExample() InitSystem {
	return &Example{}
}

// IsActive should return true if this init system is the one currently managing the OS.
// For example, by checking for the existence of specific directories (like /run/systemd/system)
// or by querying the process tree.
func (s *Example) IsActive(ctx context.Context) bool {
	// you should implement your own logic here to determine if this init system is active
	return true
}

// Name returns the unique string identifier for this init system integration.
func (s *Example) Name() string {
	return exampleInitSystem
}

// Install should perform the necessary steps to configure the init system to run
// the 'grub-os-reporter options push' command automatically right before the system shuts down or reboots.
func (s *Example) Setup(ctx context.Context, configPath string) error {
	return nil
}
