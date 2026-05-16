//go:build !windows

package cli

import (
	"context"
	"errors"
)

func ElevateAndApply(ctx context.Context, cfgFile string) error {
	return errors.New("auto-elevation is not supported on this platform, please run with sudo")
}
