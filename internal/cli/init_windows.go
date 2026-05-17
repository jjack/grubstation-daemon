//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

func init() {
	// Enable Virtual Terminal Processing for ANSI escape sequences on Windows 10+
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err == nil {
		_ = windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}

	stderr := windows.Handle(os.Stderr.Fd())
	if err := windows.GetConsoleMode(stderr, &mode); err == nil {
		_ = windows.SetConsoleMode(stderr, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}

	// Set console to UTF-8
	_ = windows.SetConsoleCP(65001)
	_ = windows.SetConsoleOutputCP(65001)
}
