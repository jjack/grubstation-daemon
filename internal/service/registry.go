package service

import (
	"context"
	"sort"
)

type Factory func() ServiceManager

type Registry struct {
	services map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]Factory),
	}
}

func (r *Registry) Register(name string, factory Factory) {
	r.services[name] = factory
}

func (r *Registry) Get(name string) ServiceManager {
	if factory, ok := r.services[name]; ok {
		return factory()
	}
	return nil
}

func (r *Registry) Detect(ctx context.Context) (ServiceManager, error) {
	var names []string
	for name := range r.services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		svc := r.services[name]()
		if svc.IsActive(ctx) {
			return svc, nil
		}
	}
	return nil, ErrNotSupported
}

func (r *Registry) ActiveServices(ctx context.Context) []string {
	var names []string
	for name, factory := range r.services {
		if factory().IsActive(ctx) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (r *Registry) SupportedServices() []string {
	var names []string
	for name := range r.services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
