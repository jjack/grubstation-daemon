//go:build !windows

package config

func DefaultConfigPath() string {
	return "/etc/grubstation/config.yaml"
}
