package bootloader

import (
	"fmt"
	"sort"
)

type Bootloader interface {
	IsActive() bool
	NewGetBootOptions(configPath string) ([]string, error)
	Name() string
}

type Factory func() Bootloader

var registry = make(map[string]Factory)

func Register(name string, factory Factory) {
	registry[name] = factory
}

func Get(name string) Bootloader {
	if factory, ok := registry[name]; ok {
		return factory()
	}
	return nil
}

func Detect() (Bootloader, error) {
	var names []string
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		factory := registry[name]
		bl := factory()
		if bl.IsActive() {
			return bl, nil
		}
	}
	return nil, fmt.Errorf("no supported bootloader detected")
}
