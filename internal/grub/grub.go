package grub

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// Grub represents the GRUB bootloader on this system.
type Grub struct {
	ConfigPath string
}

type SetupOptions struct {
	TargetMAC       string
	TargetURL       string
	AuthToken       string
	WaitTimeSeconds int
}

// GetBootOptions parses the GRUB configuration and returns available boot options.
func (g *Grub) GetBootOptions(ctx context.Context) ([]string, error) {
	slog.Debug("Parsing GRUB boot options...")

	var grubPath string
	var err error

	if g.ConfigPath != "" {
		grubPath = g.ConfigPath
		slog.Debug("Using explicit GRUB config path", slog.String("path", grubPath))
	} else {
		grubPath, err = findConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to locate grub config: %w", err)
		}
		slog.Debug("Found GRUB config at", slog.String("path", grubPath))
	}

	file, err := os.Open(grubPath)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied reading grub config %s (are you running as root?)", grubPath)
		}
		return nil, fmt.Errorf("failed to open grub config %s: %w", grubPath, err)
	}
	defer func() { _ = file.Close() }()

	return parseMenuEntries(file), nil
}
