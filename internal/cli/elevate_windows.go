//go:build windows

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func ElevateAndApply(ctx context.Context, cfgFile string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	args := fmt.Sprintf("setup --apply --config \"%s\"", cfgFile)
	
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-WindowStyle", "Normal", "-Command", 
		fmt.Sprintf("Start-Process -FilePath '%s' -ArgumentList '%s' -Verb RunAs -Wait", exe, args))
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to request elevation: %w", err)
	}
	return nil
}
