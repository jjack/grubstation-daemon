package service_manager

import (
	"context"
	"errors"
)

// Manager defines the interface for managing the agent as a background service.
type Manager interface {
	Name() string
	IsActive(ctx context.Context) bool
	Install(ctx context.Context, configPath string) error
	Uninstall(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

var ErrNotSupported = errors.New("no supported service manager detected")
