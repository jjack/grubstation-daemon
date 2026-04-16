package bootloader

import "fmt"

type Bootloader interface {
	IsActive() bool
	GetBootOptions(configPath string) ([]string, error)
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
	for _, factory := range registry {
		bl := factory()
		if bl.IsActive() {
			return bl, nil
		}
	}
	return nil, fmt.Errorf("no supported bootloader detected")
}
