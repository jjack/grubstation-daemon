package main

import (
	"testing"
)

func TestNewGenerateConfigCmd(t *testing.T) {
	cmd := NewGenerateConfigCmd()

	if cmd.Use != "generate-config" {
		t.Errorf("expected Use 'generate-config', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	// We can't easily test the RunE function because it triggers interactive UI,
	// network discovery, and writes a file to the current directory.
	// But we can assert the command is correctly constructed.
}
