package bootloader

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

const (
	initialBufferSize = 64 * 1024   // 64KB
	maxBufferCapacity = 1024 * 1024 // 1MB
)

const grubBootloader = "grub"

var (
	ErrGrubConfigNotFound = errors.New("no grub config found in known locations")
	ErrInvalidHAURL       = errors.New("invalid home assistant url: scheme and host are required")
	ErrNoGrubTool         = errors.New("neither update-grub nor grub2-mkconfig found in PATH")
)

var (
	hassRemoteBootAgentPath = "/etc/grub.d/99_ha_remote_boot_agent"
	execLookPath            = exec.LookPath
	execCommand             = exec.CommandContext
)

var grubPaths = []string{
	"/boot/grub/grub.cfg",
	"/boot/grub2/grub.cfg",
	"/boot/efi/EFI/fedora/grub.cfg",
	"/boot/efi/EFI/redhat/grub.cfg",
	"/boot/efi/EFI/ubuntu/grub.cfg",
}

//go:embed templates/99_remote_boot_agent.tmpl
var grubTemplate string

type Grub struct{}

func NewGrub() Bootloader {
	return &Grub{}
}

func (g *Grub) Name() string {
	return grubBootloader
}

func (g *Grub) IsActive(ctx context.Context) bool {
	_, err := findGrubConfig()
	return err == nil
}

func (g *Grub) DiscoverConfigPath(ctx context.Context) (string, error) {
	return findGrubConfig()
}

func findGrubConfig() (string, error) {
	for _, path := range grubPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", ErrGrubConfigNotFound
}

// countStructuralBraces ignores braces inside strings or comments to accurately track GRUB submenu lexical scoping.
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

func (g *Grub) GetBootOptions(ctx context.Context, cfg Config) ([]string, error) {
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
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied reading grub config %s (are you running as root?)", grubPath)
		}
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

	// Track brace depth across lines to build GRUB's required flat "Parent>Child" syntax for nested submenus.
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

		// Pop submenus from the tracking stack if we've exited their lexical scope (closing braces).
		for len(stack) > 0 && depth < stack[len(stack)-1].bodyDepth {
			stack = stack[:len(stack)-1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading grub config: %w", err)
	}

	return options, nil
}

// Install creates a new GRUB script in /etc/grub.d and updates the GRUB config by calling
// update-grub or grub2-mkconfig.
func (g *Grub) Setup(ctx context.Context, macAddress, haURL, webhookID string) error {
	u, err := url.Parse(haURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrInvalidHAURL
	}

	tmpl, err := template.New("grub").Parse(grubTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse grub template: %w", err)
	}

	data := struct {
		Protocol   string
		Host       string
		MACAddress string
		WebhookID  string
	}{
		Protocol:   u.Scheme,
		Host:       u.Host,
		MACAddress: macAddress,
		WebhookID:  webhookID,
	}

	var content strings.Builder
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute grub template: %w", err)
	}

	if err := os.WriteFile(hassRemoteBootAgentPath, []byte(content.String()), 0o755); err != nil {
		return fmt.Errorf("failed to create grub script (are you running as root?): %w", err)
	}

	if path, err := execLookPath("update-grub"); err == nil {
		out, err := execCommand(ctx, path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("update-grub failed: %s", string(out))
		}
		return nil
	}
	if path, err := execLookPath("grub2-mkconfig"); err == nil {
		out, err := execCommand(ctx, path, "-o", "/boot/grub2/grub.cfg").CombinedOutput()
		if err != nil {
			return fmt.Errorf("grub2-mkconfig failed: %s", string(out))
		}
		return nil
	}
	return ErrNoGrubTool
}
