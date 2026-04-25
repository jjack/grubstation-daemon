package bootloader

import (
	"context"
	"fmt"
	"sort"
)

type Config struct {
	ConfigPath string
}

type Bootloader interface {
	IsActive(ctx context.Context) bool
	GetBootOptions(ctx context.Context, cfg Config) ([]string, error)
	Name() string
	Install(ctx context.Context, macAddress string, haURL string) error
}

type Factory func() Bootloader

type Registry struct {
	bootloaders map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{
		bootloaders: make(map[string]Factory),
	}
}

func (r *Registry) Register(name string, factory Factory) {
	r.bootloaders[name] = factory
}

func (r *Registry) Get(name string) Bootloader {
	if factory, ok := r.bootloaders[name]; ok {
		return factory()
	}
	return nil
}

func (r *Registry) Detect(ctx context.Context) (Bootloader, error) {
	var names []string
	for name := range r.bootloaders {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		factory := r.bootloaders[name]
		bl := factory()
		if bl.IsActive(ctx) {
			return bl, nil
		}
	}
	return nil, fmt.Errorf("no supported bootloader detected")
}

func (r *Registry) SupportedBootloaders() []string {
	var names []string
	for name := range r.bootloaders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
