package bootloader

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

const grubBootloader = "grub"

var grubPaths = []string{
	"/boot/grub/grub.cfg",
	"/boot/grub2/grub.cfg",
	"/boot/efi/EFI/fedora/grub.cfg",
	"/boot/efi/EFI/redhat/grub.cfg",
	"/boot/efi/EFI/ubuntu/grub.cfg",
}

type Grub struct{}

func init() {
	Register(grubBootloader, NewGrub)
}

func NewGrub() Bootloader {
	return &Grub{}
}

func (g *Grub) Name() string {
	return grubBootloader
}

func (g *Grub) IsActive() bool {
	return true
}

func findGrubConfig() (string, error) {
	for _, path := range grubPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no grub config found in known locations")
}

func (g *Grub) GetBootOptions(configPath string) ([]string, error) {
	slog.Debug("Parsing GRUB boot options...")

	var grubPath string
	var err error

	if configPath != "" {
		grubPath = configPath
		slog.Debug("Using explicit GRUB config path", slog.String("path", grubPath))
	} else {
		grubPath, err = findGrubConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to locate grub config: %w", err)
		}
	}
	slog.Debug("Found GRUB config at", slog.String("path", grubPath))

	file, err := os.Open(grubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open grub config %s: %w", grubPath, err)
	}
	defer func() { _ = file.Close() }()

	// TODO: add support for submenu entries and other variations (will need to track nesting levels)
	var options []string
	scanner := bufio.NewScanner(file)
	// Match lines like: menuentry 'Ubuntu' ... or menuentry "Windows" ...
	re := regexp.MustCompile(`^menuentry\s+['"]([^'"]+)['"]`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			options = append(options, matches[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading grub config: %w", err)
	}

	return options, nil
}
