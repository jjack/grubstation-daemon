package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestMainExecutesWithoutError(t *testing.T) {
	// Set an environment variable or flag so that Execute succeeds
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Overwrite args, "grub-os-reporter help" will exit 0
	os.Args = []string{"grub-os-reporter", "--help"}

	// main normally prints to os.Stderr or calls os.Exit.
	// Since help succeeds, it shouldn't call os.Exit(1).
	// The problem is tests don't normally sandbox os.Exit, but we can test the happy path here:
	main()
}

func TestMain_ExitError(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		os.Args = []string{"grub-os-reporter", "--unknown-flag"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMain_ExitError")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}
