package bootloader

import (
	"fmt"
	"sort"
)

type Bootloader interface {
	IsActive() bool
	GetBootOptions(configPath string) ([]string, error)
	Name() string
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

func (r *Registry) Detect() (Bootloader, error) {
	var names []string
	for name := range r.bootloaders {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		factory := r.bootloaders[name]
		bl := factory()
		if bl.IsActive() {
			return bl, nil
		}
	}
	return nil, fmt.Errorf("no supported bootloader detected")
}
