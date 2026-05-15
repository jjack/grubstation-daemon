package grub

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

var (
	ErrInvalidHAURL = errors.New("invalid home assistant url: scheme and host are required")
	ErrNoGrubTool   = errors.New("neither update-grub nor grub2-mkconfig found in PATH")
)

var (
	HassGrubStationPath = "/etc/grub.d/99_grubstation"
	ExecLookPath        = exec.LookPath
	ExecCommand         = exec.CommandContext
)

//go:embed templates/99_grubstation.tmpl
var grubTemplate string

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
