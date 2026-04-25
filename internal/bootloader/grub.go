package bootloader

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

const (
	initialBufferSize = 64 * 1024   // 64KB
	maxBufferCapacity = 1024 * 1024 // 1MB
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

func NewGrub() Bootloader {
	return &Grub{}
}

func (g *Grub) Name() string {
	return grubBootloader
}

func (g *Grub) IsActive() bool {
	_, err := findGrubConfig()
	return err == nil
}

func findGrubConfig() (string, error) {
	for _, path := range grubPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no grub config found in known locations")
}

func countStructuralBraces(line string) (int, int) {
	opens, closes := 0, 0
	inSingleQuote, inDoubleQuote, escapeNext := false, false, false

	for _, r := range line {
		if escapeNext {
			escapeNext = false
			continue
		}

		switch r {
		case '\\':
			escapeNext = true
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '#':
			if !inSingleQuote && !inDoubleQuote {
				return opens, closes // Rest of the line is a comment
			}
		case '{':
			if !inSingleQuote && !inDoubleQuote {
				opens++
			}
		case '}':
			if !inSingleQuote && !inDoubleQuote {
				closes++
			}
		}
	}
	return opens, closes
}

func (g *Grub) GetBootOptions(cfg Config) ([]string, error) {
	slog.Debug("Parsing GRUB boot options...")

	var grubPath string
	var err error

	if cfg.ConfigPath != "" {
		grubPath = cfg.ConfigPath
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

	var options []string
	type submenu struct {
		name      string
		bodyDepth int
	}
	var stack []submenu

	// Match lines like: menuentry 'Ubuntu' ... or menuentry "Windows" ...
	reMenu := regexp.MustCompile(`^menuentry\s+['"]([^'"]+)['"]`)
	reSub := regexp.MustCompile(`^submenu\s+['"]([^'"]+)['"]`)

	// Create a custom buffer (initial size 64KB, max size 1MB)
	buf := make([]byte, initialBufferSize)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(buf, maxBufferCapacity)

	depth := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		opens, closes := countStructuralBraces(line)

		if m := reSub.FindStringSubmatch(line); len(m) > 1 {
			stack = append(stack, submenu{
				name:      m[1],
				bodyDepth: depth + opens,
			})
		} else if m := reMenu.FindStringSubmatch(line); len(m) > 1 {
			entry := m[1]
			if len(stack) > 0 {
				// Flatten hierarchy using GRUB's '>' convention
				var parts []string
				for _, s := range stack {
					parts = append(parts, s.name)
				}
				parts = append(parts, entry)
				entry = strings.Join(parts, ">")
			}
			options = append(options, entry)
		}

		depth += opens
		depth -= closes

		// Pop submenus that we have exited
		for len(stack) > 0 && depth < stack[len(stack)-1].bodyDepth {
			stack = stack[:len(stack)-1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading grub config: %w", err)
	}

	return options, nil
}
