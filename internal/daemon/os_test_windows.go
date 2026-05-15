//go:build windows

package daemon

import "os/exec"

func getMockShutdownCommand() *exec.Cmd {
	return exec.Command("cmd", "/c", "exit", "0")
}
