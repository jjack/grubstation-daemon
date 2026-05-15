package grub

import (
	"strings"
	"testing"
)

func TestParseMenuEntries(t *testing.T) {
	const sampleGrubCfg = `
menuentry 'Debian GNU/Linux' {
        linux   /boot/vmlinuz
}
submenu 'Advanced options for Debian GNU/Linux' {
        menuentry 'Debian GNU/Linux, with Linux 6.12.74' {
                linux   /boot/vmlinuz
        }
}
menuentry 'Windows Boot Manager' {
        chainloader /EFI/Microsoft/Boot/bootmgfw.efi
}
`
	expected := []string{
		"Debian GNU/Linux",
		"Advanced options for Debian GNU/Linux>Debian GNU/Linux, with Linux 6.12.74",
		"Windows Boot Manager",
	}

	reader := strings.NewReader(sampleGrubCfg)
	results := parseMenuEntries(reader)

	if len(results) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(results))
	}
	for i, res := range results {
		if res != expected[i] {
			t.Errorf("expected %q, got %q", expected[i], res)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Single Quotes", "menuentry 'Proxmox VE' {", "Proxmox VE"},
		{"Double Quotes", "menuentry \"Windows 11\" {", "Windows 11"},
		{"No Quotes", "menuentry no_quotes {", ""},
		{"Single Quotes with extra args", "submenu 'Advanced options' --class gnu {", "Advanced options"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTitle(tt.input)
			if result != tt.expected {
				t.Errorf("extractTitle(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
