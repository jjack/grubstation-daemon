//go:build !linux

package daemon

import "context"

func onShutdownHook(ctx context.Context, d *Daemon) {}
