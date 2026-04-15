package grub

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
)

var grubPaths = []string{
	"/boot/grub/grub.cfg",
	"/boot/grub2/grub.cfg",
	"/boot/efi/EFI/fedora/grub.cfg",
	"/boot/efi/EFI/redhat/grub.cfg",
	"/boot/efi/EFI/ubuntu/grub.cfg",
}

type GrubPlugin struct {
	// Add config for the grub plugin here
}

func New() *GrubPlugin {
	return &GrubPlugin{}
}

func (p *GrubPlugin) Name() string {
	return "grub"
}

func (p *GrubPlugin) Detect() bool {
	if _, err := findGrubConfig(); err == nil {
		return true
	}
	return false
}

func findGrubConfig() (string, error) {
	for _, path := range grubPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no grub config found in known locations")
}

func (p *GrubPlugin) Parse(cfg *config.Config) (*bootloader.BootOptions, error) {
	log.Println("Parsing GRUB boot options...")

	grubPath, err := findGrubConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to locate grub config: %w", err)
	}
	log.Printf("Found GRUB config at: %s\n", grubPath)

	file, err := os.Open(grubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open grub config %s: %w", grubPath, err)
	}
	defer file.Close()

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

	return &bootloader.BootOptions{
		AvailableOSes: options,
	}, nil
}
