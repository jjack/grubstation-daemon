//go:build windows

package service_manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	windowsServiceName        = "GrubStation"
	windowsServiceDisplayName = "GrubStation"
	windowsServiceDescription = "Persistent daemon for reporting boot options and remote shutdown"
)

type WindowsService struct{}

func NewWindowsService() Manager {
	return &WindowsService{}
}

// RegisterDefaultServices registers the Windows Service manager.
func RegisterDefaultServices(r *Registry) {
	r.Register("windows-service", NewWindowsService)
}

func (w *WindowsService) Name() string {
	return "windows-service"
}

func (w *WindowsService) IsActive(ctx context.Context) bool {
	return true
}

func (w *WindowsService) Install(ctx context.Context, configPath string) error {
	exepath, err := os.Executable()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", windowsServiceName)
	}

	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}

	// The service will run: grubstation.exe daemon --config C:\path\to\config.yaml
	s, err = m.CreateService(windowsServiceName, exepath, mgr.Config{
		DisplayName:    windowsServiceDisplayName,
		Description:    windowsServiceDescription,
		StartType:      mgr.StartAutomatic,
		BinaryPathName: fmt.Sprintf("%s daemon --config %s", exepath, absConfig),
	})
	if err != nil {
		return err
	}
	defer s.Close()

	return nil
}

func (w *WindowsService) Uninstall(ctx context.Context) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return nil // already gone
	}
	defer s.Close()

	err = s.Delete()
	if err != nil {
		return err
	}

	return nil
}

func (w *WindowsService) Start(ctx context.Context) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Start()
}

func (w *WindowsService) Stop(ctx context.Context) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return err
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}

	if status.State != svc.Stopped {
		return fmt.Errorf("failed to stop service, state: %d", status.State)
	}

	return nil
}
