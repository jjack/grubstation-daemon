package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jjack/grub-os-reporter/internal/config"
	"github.com/jjack/grub-os-reporter/internal/grub"
)

func createListTempGrubConfig(t *testing.T, content string) string {
	tempGrub, err := os.CreateTemp("", "grub.cfg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tempGrub.Write([]byte(content))
	_ = tempGrub.Close()
	t.Cleanup(func() { _ = os.Remove(tempGrub.Name()) })
	return tempGrub.Name()
}

func TestGetBootOptionsCommand(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: true},
	}
	tempGrubPath := createListTempGrubConfig(t, "menuentry 'Ubuntu' {}\nmenuentry 'Windows' {}\n")

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: tempGrubPath}}
	cmd := NewListCmd(deps)

	// Intercept stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "- Ubuntu") {
		t.Errorf("output missing boot option 'Ubuntu': %s", output)
	}
	if !strings.Contains(output, "- Windows") {
		t.Errorf("output missing boot option 'Windows': %s", output)
	}
}

func TestGetBootOptionsCommand_GrubError(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: true},
	}

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: t.TempDir()}}
	cmd := NewListCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from GetBootOptions, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get boot options") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetBootOptionsCommand_EmptyOptions(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: true},
	}
	tempGrubPath := createListTempGrubConfig(t, "")

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: tempGrubPath}}
	cmd := NewListCmd(deps)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "(None found)") {
		t.Errorf("output missing '(None found)': %s", string(out))
	}
}
