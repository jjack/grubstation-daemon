package servicemanager

import (
	"context"
	"testing"
)

func TestErrors(t *testing.T) {
	if ErrNotSupported.Error() != "no supported service manager detected" {
		t.Errorf("unexpected error message: %s", ErrNotSupported.Error())
	}
}

type mockMgr struct {
	name   string
	active bool
}

func (m *mockMgr) Name() string                                         { return m.name }
func (m *mockMgr) IsActive(ctx context.Context) bool                    { return m.active }
func (m *mockMgr) Install(ctx context.Context, configPath string) error { return nil }
func (m *mockMgr) Uninstall(ctx context.Context) error                  { return nil }
func (m *mockMgr) Start(ctx context.Context) error                      { return nil }
func (m *mockMgr) Stop(ctx context.Context) error                       { return nil }

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	r.Register("b_mgr", func() Manager { return &mockMgr{name: "b_mgr", active: false} })
	r.Register("a_mgr", func() Manager { return &mockMgr{name: "a_mgr", active: true} })

	t.Run("Get", func(t *testing.T) {
		if r.Get("a_mgr") == nil {
			t.Error("expected to find a_mgr")
		}
		if r.Get("nonexistent") != nil {
			t.Error("expected nil for nonexistent")
		}
	})

	t.Run("Detect", func(t *testing.T) {
		mgr, err := r.Detect(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should be a_mgr because it's active and names are sorted alphabetically before detection
		if mgr.Name() != "a_mgr" {
			t.Errorf("expected a_mgr, got %s", mgr.Name())
		}

		empty := NewRegistry()
		_, err = empty.Detect(context.Background())
		if err != ErrNotSupported {
			t.Errorf("expected ErrNotSupported, got %v", err)
		}
	})

	t.Run("ActiveServices", func(t *testing.T) {
		active := r.ActiveServices(context.Background())
		if len(active) != 1 || active[0] != "a_mgr" {
			t.Errorf("expected [a_mgr], got %v", active)
		}
	})

	t.Run("SupportedServices", func(t *testing.T) {
		sup := r.SupportedServices()
		if len(sup) != 2 || sup[0] != "a_mgr" || sup[1] != "b_mgr" {
			t.Errorf("expected [a_mgr, b_mgr], got %v", sup)
		}
	})
}
