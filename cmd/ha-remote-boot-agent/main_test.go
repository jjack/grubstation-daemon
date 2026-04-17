package main

import (
	"os"
	"testing"
)

func TestMainExecutesWithoutError(t *testing.T) {
	// Set an environment variable or flag so that Execute succeeds
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Overwrite args, "ha-remote-boot-agent help" will exit 0
	os.Args = []string{"ha-remote-boot-agent", "--help"}

	// main normally prints to os.Stderr or calls os.Exit.
	// Since help succeeds, it shouldn't call os.Exit(1).
	// The problem is tests don't normally sandbox os.Exit, but we can test the happy path here:
	main()
}
