package initsystem

import (
	"context"
	"errors"
	"sort"
)

type InitSystem interface {
	IsActive(ctx context.Context) bool
	Name() string
	Install(ctx context.Context, configPath string) error
}

type Factory func() InitSystem

type Registry struct {
	initsystems map[string]Factory
}

var ErrNotSupported = errors.New("no supported init system detected")

func NewRegistry() *Registry {
	return &Registry{
		initsystems: make(map[string]Factory),
	}
}

func (r *Registry) Register(name string, factory Factory) {
	r.initsystems[name] = factory
}

func (r *Registry) Get(name string) InitSystem {
	if factory, ok := r.initsystems[name]; ok {
		return factory()
	}
	return nil
}

func (r *Registry) Detect(ctx context.Context) (InitSystem, error) {
	// Iterate through map keys in a sorted order for deterministic detection tests
	var names []string
	for name := range r.initsystems {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sys := r.initsystems[name]()
		if sys.IsActive(ctx) {
			return sys, nil
		}
	}
	return nil, ErrNotSupported
}

func (r *Registry) SupportedInitSystems() []string {
	var names []string
	for name := range r.initsystems {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
