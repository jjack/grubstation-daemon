//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func DefaultConfigPath() string {
	return filepath.Join(os.Getenv("AppData"), "GrubStation", "config.yaml")
}
