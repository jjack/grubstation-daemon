//go:build !windows

package daemon

import (
	"os/exec"
)

func getShutdownCommand() *exec.Cmd {
	return execCommand("poweroff")
}
