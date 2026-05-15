package grub

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"text/template"
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

var (
	ErrConfigNotFound = errors.New("no grub config found in known locations")
	ErrInvalidHAURL   = errors.New("invalid home assistant url: scheme and host are required")
	ErrNoGrubTool     = errors.New("neither update-grub nor grub2-mkconfig found in PATH")
)

var (
	HassGrubStationPath = "/etc/grub.d/99_grubstation"
	ExecLookPath        = exec.LookPath
	ExecCommand         = exec.CommandContext
)

var knownConfigPaths = []string{
	"/boot/grub/grub.cfg",
	"/boot/grub2/grub.cfg",
	"/boot/efi/EFI/fedora/grub.cfg",
	"/boot/efi/EFI/redhat/grub.cfg",
	"/boot/efi/EFI/ubuntu/grub.cfg",
}

//go:embed templates/99_grubstation.tmpl
var grubTemplate string

// DiscoverConfigPath attempts to auto-detect the GRUB config file path.
func (g *Grub) DiscoverConfigPath(ctx context.Context) (string, error) {
	return findConfig()
}

func generateWaitList(seconds int) string {
	if seconds <= 0 {
		return "1"
	}
	var parts []string
	for i := 1; i <= seconds; i++ {
		parts = append(parts, fmt.Sprintf("%d", i))
	}
	return strings.Join(parts, " ")
}

func findConfig() (string, error) {
	for _, path := range knownConfigPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", ErrConfigNotFound
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

// submenuContext keeps track of where we are in the nested tree
type submenuContext struct {
	depth int
	title string
}

// parseMenuEntries takes an io.Reader and returns a flat list of GRUB boot targets.
func parseMenuEntries(r io.Reader) []string {
	scanner := bufio.NewScanner(r)
	var entries []string
	var stack []submenuContext
	braceDepth := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 1. Process the line if it's a menu declaration
		if strings.HasPrefix(line, "submenu ") {
			title := extractTitle(line)
			if title != "" {
				// Record the depth *before* we enter the block
				stack = append(stack, submenuContext{depth: braceDepth, title: title})
			}
		} else if strings.HasPrefix(line, "menuentry ") {
			title := extractTitle(line)
			if title != "" {
				entries = append(entries, buildTargetString(stack, title))
			}
		}

		// 2. Update our brace depth state
		// We do this after checking the prefixes so the submenu's own opening brace
		// increases the depth to a level *deeper* than the submenu's recorded depth.
		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")

		// 3. Pop the stack if we've exited a submenu block
		// If our depth drops to or below the depth where the current submenu was declared,
		// it means we have closed that submenu's bracket.
		for len(stack) > 0 && braceDepth <= stack[len(stack)-1].depth {
			stack = stack[:len(stack)-1]
		}
	}

	return entries
}

// extractTitle finds the first string wrapped in single or double quotes
func extractTitle(line string) string {
	var quoteChar rune
	start := -1

	for i, char := range line {
		if (char == '\'' || char == '"') && start == -1 {
			quoteChar = char
			start = i + 1
		} else if char == quoteChar && start != -1 {
			return line[start:i]
		}
	}
	return ""
}

// buildTargetString joins the current submenu stack with the final entry name
func buildTargetString(stack []submenuContext, entryTitle string) string {
	if len(stack) == 0 {
		return entryTitle
	}

	var parts []string
	for _, s := range stack {
		parts = append(parts, s.title)
	}
	parts = append(parts, entryTitle)

	return strings.Join(parts, ">")
}

// Setup creates a GRUB remote boot agent script in /etc/grub.d and updates the GRUB config.
func (g *Grub) Setup(ctx context.Context, opts SetupOptions) error {
	u, err := url.Parse(opts.TargetURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrInvalidHAURL
	}

	tmpl, err := template.New("grub").Parse(grubTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse grub template: %w", err)
	}

	data := struct {
		Host            string
		MACAddress      string
		WebhookID       string
		WaitTimeSeconds int
		WaitList        string
	}{
		Host:            u.Host,
		MACAddress:      opts.TargetMAC,
		WebhookID:       opts.AuthToken,
		WaitTimeSeconds: opts.WaitTimeSeconds,
		WaitList:        generateWaitList(opts.WaitTimeSeconds),
	}

	var content strings.Builder
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute grub template: %w", err)
	}

	if err := os.WriteFile(HassGrubStationPath, []byte(content.String()), 0o755); err != nil {
		return fmt.Errorf("failed to create grub script (are you running as root?): %w", err)
	}

	if path, err := ExecLookPath("update-grub"); err == nil {
		out, err := ExecCommand(ctx, path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("update-grub failed: %s", string(out))
		}
		return nil
	}
	if path, err := ExecLookPath("grub2-mkconfig"); err == nil {
		out, err := ExecCommand(ctx, path, "-o", "/boot/grub2/grub.cfg").CombinedOutput()
		if err != nil {
			return fmt.Errorf("grub2-mkconfig failed: %s", string(out))
		}
		return nil
	}
	return ErrNoGrubTool
}

// SetupWarning returns a message about potential hardware incompatibilities with GRUB networking.
func (g *Grub) SetupWarning() string {
	return "note: the exact GRUB networking configuration applied by this tool may not work perfectly " +
		"for every motherboard due to how finicky UEFI and network firmware can be across different " +
		"hardware vendors. If your system struggles to connect to the network from within GRUB, " +
		"you may need to manually troubleshoot your GRUB network settings."
}

// Uninstall removes the GRUB remote boot agent script and updates the GRUB config.
func (g *Grub) Uninstall(ctx context.Context) error {
	if err := os.Remove(HassGrubStationPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove grub script: %w", err)
	}

	if path, err := ExecLookPath("update-grub"); err == nil {
		out, err := ExecCommand(ctx, path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("update-grub failed: %s", string(out))
		}
		return nil
	}
	if path, err := ExecLookPath("grub2-mkconfig"); err == nil {
		out, err := ExecCommand(ctx, path, "-o", "/boot/grub2/grub.cfg").CombinedOutput()
		if err != nil {
			return fmt.Errorf("grub2-mkconfig failed: %s", string(out))
		}
		return nil
	}
	return nil
}
