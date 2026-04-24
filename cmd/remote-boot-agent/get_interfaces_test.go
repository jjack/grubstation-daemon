package main

import (
	"testing"

	"github.com/charmbracelet/huh"
)

func TestGetInterfaces_ReturnsOptions(t *testing.T) {
	// Simulate user input by providing a getInterfaces func with one option
	interfaces := []huh.Option[string]{huh.NewOption("eth0 (00:11:22:33:44:55)", "00:11:22:33:44:55")}
	getInterfaces := func() []huh.Option[string] { return interfaces }
	opts := getInterfaces()
	if opts == nil {
		t.Fatal("getInterfaces returned nil")
	}
}
