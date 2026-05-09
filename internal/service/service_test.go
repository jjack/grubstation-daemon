package service

import (
	"context"
	"testing"
)

func TestErrors(t *testing.T) {
	if ErrNotSupported.Error() != "no supported service manager detected" {
		t.Errorf("unexpected error message: %s", ErrNotSupported.Error())
	}
}

type mockService struct {
	name   string
	active bool
}

func (m *mockService) Name() string                                         { return m.name }
func (m *mockService) IsActive(ctx context.Context) bool                    { return m.active }
func (m *mockService) Install(ctx context.Context, configPath string) error { return nil }
func (m *mockService) Uninstall(ctx context.Context) error                  { return nil }
func (m *mockService) Start(ctx context.Context) error                      { return nil }
func (m *mockService) Stop(ctx context.Context) error                       { return nil }

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	r.Register("b_service", func() ServiceManager { return &mockService{name: "b_service", active: false} })
	r.Register("a_service", func() ServiceManager { return &mockService{name: "a_service", active: true} })

	t.Run("Get", func(t *testing.T) {
		if r.Get("a_service") == nil {
			t.Error("expected to find a_service")
		}
		if r.Get("nonexistent") != nil {
			t.Error("expected nil for nonexistent")
		}
	})

	t.Run("Detect", func(t *testing.T) {
		svc, err := r.Detect(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should be a_service because it's active and names are sorted alphabetically before detection
		if svc.Name() != "a_service" {
			t.Errorf("expected a_service, got %s", svc.Name())
		}

		empty := NewRegistry()
		_, err = empty.Detect(context.Background())
		if err != ErrNotSupported {
			t.Errorf("expected ErrNotSupported, got %v", err)
		}
	})

	t.Run("ActiveServices", func(t *testing.T) {
		active := r.ActiveServices(context.Background())
		if len(active) != 1 || active[0] != "a_service" {
			t.Errorf("expected [a_service], got %v", active)
		}
	})

	t.Run("SupportedServices", func(t *testing.T) {
		sup := r.SupportedServices()
		if len(sup) != 2 || sup[0] != "a_service" || sup[1] != "b_service" {
			t.Errorf("expected [a_service, b_service], got %v", sup)
		}
	})
}
