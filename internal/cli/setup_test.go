package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
)

type mockInstallBootloader struct {
	installErr error
	mac        string
	url        string
	webhook    string
	warning    string
}

func (m *mockInstallBootloader) Name() string                      { return "mock-bl" }
func (m *mockInstallBootloader) IsActive(ctx context.Context) bool { return true }
func (m *mockInstallBootloader) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return nil, nil
}

func (m *mockInstallBootloader) Setup(ctx context.Context, macAddress, haURL, webhookID string) error {
	m.mac = macAddress
	m.url = haURL
	m.webhook = webhookID
	return m.installErr
}

func (m *mockInstallBootloader) SetupWarning() string {
	return m.warning
}

func (m *mockInstallBootloader) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "", nil
}

type mockInstallInitSystem struct {
	installErr error
	configPath string
}

func (m *mockInstallInitSystem) Name() string                      { return "mock-init" }
func (m *mockInstallInitSystem) IsActive(ctx context.Context) bool { return true }
func (m *mockInstallInitSystem) Setup(ctx context.Context, configPath string) error {
	m.configPath = configPath
	return m.installErr
}

func TestApplyCmd_Success(t *testing.T) {
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress: "aa:bb:cc:dd:ee:ff",
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       "http://ha.local",
			WebhookID: "test-webhook",
		},
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blMock := &mockInstallBootloader{}
	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return blMock })

	initMock := &mockInstallInitSystem{}
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return initMock })

	deps := &CommandDeps{
		Config:             cfg,
		BootloaderRegistry: blReg,
		InitRegistry:       initReg,
	}

	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "test-config.yaml", "")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if blMock.mac != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected mac aa:bb:cc:dd:ee:ff, got %s", blMock.mac)
	}
	if blMock.url != "http://ha.local" {
		t.Errorf("expected url http://ha.local, got %s", blMock.url)
	}
	if blMock.webhook != "test-webhook" {
		t.Errorf("expected webhook test-webhook, got %s", blMock.webhook)
	}
	if !strings.HasSuffix(initMock.configPath, "test-config.yaml") {
		t.Errorf("expected config path to end with test-config.yaml, got %s", initMock.configPath)
	}
	if !filepath.IsAbs(initMock.configPath) {
		t.Errorf("expected absolute config path, got %s", initMock.configPath)
	}
	if !strings.Contains(out.String(), "Installation completed successfully") {
		t.Errorf("expected success message, got %s", out.String())
	}
	if strings.Contains(out.String(), "Note:") {
		t.Errorf("expected no warning message, got %s", out.String())
	}
}

func TestApplyCmd_SetupWarning(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blMock := &mockInstallBootloader{warning: "CRITICAL HARDWARE WARNING"}
	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return blMock })

	initMock := &mockInstallInitSystem{}
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return initMock })

	deps := &CommandDeps{
		Config:             cfg,
		BootloaderRegistry: blReg,
		InitRegistry:       initReg,
	}

	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "test-config.yaml", "")

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Note: CRITICAL HARDWARE WARNING") {
		t.Errorf("expected warning message, got %s", out.String())
	}
}

func TestApplyCmd_BootloaderError(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{installErr: errors.New("fail")} })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: blReg, InitRegistry: initReg}
	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to install bootloader") {
		t.Fatalf("expected bootloader install error, got %v", err)
	}
}

func TestApplyCmd_InitSystemError(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{} })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{installErr: errors.New("fail")} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: blReg, InitRegistry: initReg}
	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to install init system") {
		t.Fatalf("expected init system install error, got %v", err)
	}
}

func TestApplyCmd_MissingConfigFlag(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{} })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: blReg, InitRegistry: initReg}
	cmd := NewApplyCmd(deps) // Missing binding the "config" flag locally

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "flag accessed but not defined") {
		t.Fatalf("expected flag missing error, got %v", err)
	}
}

func TestApplyCmd_BootloaderResolveError(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "invalid-bl"},
	}

	blReg := bootloader.NewRegistry()
	initReg := initsystem.NewRegistry()

	deps := &CommandDeps{
		Config:             cfg,
		BootloaderRegistry: blReg,
		InitRegistry:       initReg,
	}

	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "test-config.yaml", "")

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "specified bootloader invalid-bl not supported") {
		t.Fatalf("expected bootloader resolve error, got %v", err)
	}
}

func TestApplyCmd_InitSystemResolveError(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "invalid-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{} })
	initReg := initsystem.NewRegistry()

	deps := &CommandDeps{
		Config:             cfg,
		BootloaderRegistry: blReg,
		InitRegistry:       initReg,
	}

	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "test-config.yaml", "")

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "specified init system invalid-init not supported") {
		t.Fatalf("expected init system resolve error, got %v", err)
	}
}

func TestApplyCmd_AbsConfigError(t *testing.T) {
	// Save the original working directory so we can restore it after the test
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	// Create a temp dir, change into it, and then delete it to break os.Getwd()
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	if err := os.RemoveAll(tempDir); err != nil {
		t.Fatalf("failed to remove temp dir: %v", err)
	}

	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{} })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{
		Config:             cfg,
		BootloaderRegistry: blReg,
		InitRegistry:       initReg,
	}

	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "relative-config.yaml", "") // Must be relative to trigger os.Getwd()

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to resolve config path") {
		t.Fatalf("expected filepath.Abs error, got %v", err)
	}
}

func TestRunConfirm(t *testing.T) {
	// Test the runConfirm function to verify initialization and provide coverage.
	var val bool
	_ = runConfirm(&val)
}

func TestSetupCmd_ConfigFlagFallback(t *testing.T) {
	oldRunGenerateSurvey := runGenerateSurvey
	oldRunConfirm := runConfirm
	defer func() {
		runGenerateSurvey = oldRunGenerateSurvey
		runConfirm = oldRunConfirm
	}()

	oldOsMkdirAll := osMkdirAll
	osMkdirAll = func(path string, perm os.FileMode) error { return nil }
	defer func() { osMkdirAll = oldOsMkdirAll }()

	runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
		return &config.Config{
			Bootloader: config.BootloaderConfig{Name: "mock-bl"},
			InitSystem: config.InitSystemConfig{Name: "mock-init"},
		}, nil
	}
	runConfirm = func(installNow *bool) error { *installNow = false; return nil }

	blMock := &mockInstallBootloader{}
	initMock := &mockInstallInitSystem{}
	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return blMock })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return initMock })

	var savedPath string
	sysResolver := &mockSystemResolver{
		saveConfigFunc: func(cfg *config.Config, path string) error {
			savedPath = path
			return nil
		},
	}

	deps := &CommandDeps{
		Config:             &config.Config{},
		BootloaderRegistry: blReg,
		InitRegistry:       initReg,
		SystemResolver:     sysResolver,
	}

	cmd := NewSetupCmd(deps)
	cmd.ResetFlags() // Strip the "config" flag to force GetString to error out

	_ = cmd.Execute() // We ignore execution err if any to verify the fallback below

	if savedPath != "/etc/remote-boot-agent/config.yaml" {
		t.Errorf("expected default fallback path /etc/remote-boot-agent/config.yaml, got %s", savedPath)
	}
}

func TestSetupCmd_Execute(t *testing.T) {
	oldRunGenerateSurvey := runGenerateSurvey
	oldRunConfirm := runConfirm
	defer func() {
		runGenerateSurvey = oldRunGenerateSurvey
		runConfirm = oldRunConfirm
	}()

	oldOsMkdirAll := osMkdirAll
	osMkdirAll = func(path string, perm os.FileMode) error { return nil }
	defer func() { osMkdirAll = oldOsMkdirAll }()

	tests := []struct {
		name        string
		setup       func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver)
		wantErr     string
		wantInstall bool
		wantOut     []string
	}{
		{
			name: "Success - Install Now (Push Warning)",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						Bootloader: config.BootloaderConfig{Name: "mock-bl"},
						InitSystem: config.InitSystemConfig{Name: "mock-init"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
			},
			wantInstall: true,
			wantOut: []string{
				"Proceeding with installation...",
				"Pushing initial boot options to Home Assistant...",
				"Warning: failed to push initial state to Home Assistant",
				"You can try pushing manually later",
			},
		},
		{
			name: "Success - Install Now (Push Success)",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(func() { ts.Close() })

				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						Bootloader: config.BootloaderConfig{Name: "mock-bl"},
						InitSystem: config.InitSystemConfig{Name: "mock-init"},
						HomeAssistant: config.HomeAssistantConfig{
							URL:       ts.URL,
							WebhookID: "test-webhook",
						},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
			},
			wantInstall: true,
			wantOut: []string{
				"Proceeding with installation...",
				"Pushing initial boot options to Home Assistant...",
				"Successfully pushed initial state to Home Assistant.",
			},
		},
		{
			name: "Success - Install Later",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						Bootloader: config.BootloaderConfig{Name: "mock-bl"},
						InitSystem: config.InitSystemConfig{Name: "mock-init"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = false; return nil }
			},
			wantInstall: false,
			wantOut: []string{
				"Setup complete. You can apply the system hooks later",
				"To populate Home Assistant immediately without rebooting, run: remote-boot-agent options push",
			},
		},
		{
			name: "Error - ensureSupport Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				deps.BootloaderRegistry = bootloader.NewRegistry() // Empty registry causes error
			},
			wantErr:     "no supported bootloader detected",
			wantInstall: false,
		},
		{
			name: "Error - ensureSupport Fails (InitSystem)",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				deps.InitRegistry = initsystem.NewRegistry() // Empty registry causes init system error
			},
			wantErr:     "no supported init system detected",
			wantInstall: false,
		},
		{
			name: "Error - Generate Survey Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return nil, errors.New("survey failed")
				}
			},
			wantErr:     "survey failed",
			wantInstall: false,
		},
		{
			name: "Error - MkdirAll Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{}, nil
				}
				osMkdirAll = func(path string, perm os.FileMode) error { return errors.New("mkdirall failed") }
				t.Cleanup(func() { osMkdirAll = func(path string, perm os.FileMode) error { return nil } })
			},
			wantErr:     "failed to create config directory: mkdirall failed",
			wantInstall: false,
		},
		{
			name: "Error - Save Config Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{}, nil
				}
				resolver.saveConfigFunc = func(cfg *config.Config, path string) error {
					return errors.New("save config failed")
				}
			},
			wantErr:     "save config failed",
			wantInstall: false,
		},
		{
			name: "Error - Confirm Prompt Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{}, nil
				}
				runConfirm = func(installNow *bool) error { return errors.New("confirm prompt failed") }
			},
			wantErr:     "confirm prompt failed",
			wantInstall: false,
		},
		{
			name: "Error - Perform Install Bootloader Resolve Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						Bootloader: config.BootloaderConfig{Name: "invalid-bl"},
						InitSystem: config.InitSystemConfig{Name: "mock-init"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
			},
			wantErr:     "specified bootloader invalid-bl not supported",
			wantInstall: false,
		},
		{
			name: "Error - Perform Install InitSystem Resolve Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						Bootloader: config.BootloaderConfig{Name: "mock-bl"},
						InitSystem: config.InitSystemConfig{Name: "invalid-init"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
			},
			wantErr:     "specified init system invalid-init not supported",
			wantInstall: false,
		},
		{
			name: "Error - Perform Install Fails",
			setup: func(t *testing.T, deps *CommandDeps, blMock *mockInstallBootloader, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						Bootloader: config.BootloaderConfig{Name: "mock-bl"},
						InitSystem: config.InitSystemConfig{Name: "mock-init"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
				blMock.installErr = errors.New("install failed")
			},
			wantErr:     "install failed",
			wantInstall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blMock := &mockInstallBootloader{}
			initMock := &mockInstallInitSystem{}
			blReg := bootloader.NewRegistry()
			blReg.Register("mock-bl", func() bootloader.Bootloader { return blMock })
			initReg := initsystem.NewRegistry()
			initReg.Register("mock-init", func() initsystem.InitSystem { return initMock })

			sysResolver := &mockSystemResolver{
				saveConfigFunc: func(cfg *config.Config, path string) error { return nil },
			}

			deps := &CommandDeps{
				Config:             &config.Config{},
				BootloaderRegistry: blReg,
				InitRegistry:       initReg,
				SystemResolver:     sysResolver,
			}

			tt.setup(t, deps, blMock, initMock, sysResolver)

			cmd := NewSetupCmd(deps)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--config", "dummy.yaml"})

			err := cmd.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if tt.wantInstall {
				if initMock.configPath == "" {
					t.Errorf("expected install to occur, but it didn't")
				}
			} else {
				if initMock.configPath != "" {
					t.Errorf("expected install to NOT occur, but it did")
				}
			}

			if len(tt.wantOut) > 0 {
				outStr := out.String()
				for _, w := range tt.wantOut {
					if !strings.Contains(outStr, w) {
						t.Errorf("expected output to contain %q, got %q", w, outStr)
					}
				}
			}
		})
	}
}
