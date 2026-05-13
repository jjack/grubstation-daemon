//go:build linux

package host

import (
	"net"
	"os"
	"testing"
)

func TestIsWOLCapableInterface_LinuxSpecific(t *testing.T) {
	mac, _ := net.ParseMAC("00:11:22:33:44:55")

	oldOsStat := osStat
	defer func() { osStat = oldOsStat }()

	tests := []struct {
		name     string
		inf      net.Interface
		mockStat func(string) (os.FileInfo, error)
		expected bool
	}{
		{
			name: "virtual prefix docker",
			inf:  net.Interface{Name: "docker0", HardwareAddr: mac, Flags: net.FlagUp},
			mockStat: func(name string) (os.FileInfo, error) {
				return nil, nil
			},
			expected: false,
		},
		{
			name: "virtual prefix veth",
			inf:  net.Interface{Name: "veth123", HardwareAddr: mac, Flags: net.FlagUp},
			mockStat: func(name string) (os.FileInfo, error) {
				return nil, nil
			},
			expected: false,
		},
		{
			name: "physical interface",
			inf:  net.Interface{Name: "eth0", HardwareAddr: mac, Flags: net.FlagUp},
			mockStat: func(name string) (os.FileInfo, error) {
				return nil, nil
			},
			expected: true,
		},
		{
			name: "device not found",
			inf:  net.Interface{Name: "eth0", HardwareAddr: mac, Flags: net.FlagUp},
			mockStat: func(name string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osStat = tt.mockStat
			if got := isWOLCapableInterface(tt.inf); got != tt.expected {
				t.Errorf("isWOLCapableInterface() = %v, want %v", got, tt.expected)
			}
		})
	}
}
