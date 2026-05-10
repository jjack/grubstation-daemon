package cli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jjack/grubstation-daemon/internal/cli/survey"
	"github.com/jjack/grubstation-daemon/internal/config"
	"github.com/jjack/grubstation-daemon/internal/grub"
	"github.com/jjack/grubstation-daemon/internal/service_manager"
)

type mockInstallInitSystem struct {
	installErr error
	startErr   error
	configPath string
}

func (m *mockInstallInitSystem) Name() string                      { return "mock-init" }
func (m *mockInstallInitSystem) IsActive(ctx context.Context) bool { return true }
func (m *mockInstallInitSystem) Install(ctx context.Context, configPath string) error {
	m.configPath = configPath
	return m.installErr
}
func (m *mockInstallInitSystem) Uninstall(ctx context.Context) error { return nil }
func (m *mockInstallInitSystem) Start(ctx context.Context) error     { return m.startErr }
func (m *mockInstallInitSystem) Stop(ctx context.Context) error      { return nil }

func TestApplyCmd_GrubError(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{ReportBootOptions: true},
	}

	initReg := service_manager.NewRegistry()
	initReg.Register("mock-init", func() service_manager.Manager { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: "/invalid/path/grub.cfg"}, Registry: initReg}
	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to install grub") {
		t.Fatalf("expected grub install error, got %v", err)
	}
}

func TestApplyCmd_MissingConfigFlag(t *testing.T) {
	cfg := &config.Config{}

	initReg := service_manager.NewRegistry()
	initReg.Register("mock-init", func() service_manager.Manager { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{}, Registry: initReg}
	cmd := NewApplyCmd(deps) // Missing binding the "config" flag locally

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "flag accessed but not defined") {
		t.Fatalf("expected flag missing error, got %v", err)
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

	cfg := &config.Config{}

	initReg := service_manager.NewRegistry()
	initReg.Register("mock-init", func() service_manager.Manager { return &mockInstallInitSystem{} })

	deps := &CommandDeps{
		Config:   cfg,
		Grub:     &grub.Grub{},
		Registry: initReg,
	}

	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "relative-config.yaml", "") // Must be relative to trigger os.Getwd()

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to resolve config path") {
		t.Fatalf("expected filepath.Abs error, got %v", err)
	}
}

func TestSetupCmd_ConfigFlagFallback(t *testing.T) {
	oldRunGenerateSurvey := survey.RunGenerateSurvey
	oldRunConfirm := runConfirm
	defer func() {
		survey.RunGenerateSurvey = oldRunGenerateSurvey
		runConfirm = oldRunConfirm
	}()

	oldOsMkdirAll := osMkdirAll
	oldCheckWriteAccess := checkWriteAccess
	osMkdirAll = func(path string, perm os.FileMode) error { return nil }
	checkWriteAccess = func(path string) error { return nil }
	defer func() {
		osMkdirAll = oldOsMkdirAll
		checkWriteAccess = oldCheckWriteAccess
	}()

	survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
		return &config.Config{}, nil
	}
	runConfirm = func(installNow *bool) error { *installNow = false; return nil }

	initMock := &mockInstallInitSystem{}
	initReg := service_manager.NewRegistry()
	initReg.Register("mock-init", func() service_manager.Manager { return initMock })

	var savedPath string
	sysResolver := &mockSystemResolver{
		saveConfigFunc: func(cfg *config.Config, path string) error {
			savedPath = path
			return nil
		},
	}

	deps := &CommandDeps{
		Config:         &config.Config{},
		Grub:           &grub.Grub{},
		Registry:       initReg,
		SystemResolver: sysResolver,
	}

	cmd := NewSetupCmd(deps)
	cmd.ResetFlags() // Strip the "config" flag to force GetString to error out

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error from setup fallback, got %v", err)
	}

	if savedPath != "/etc/grubstation/config.yaml" {
		t.Errorf("expected default fallback path /etc/grubstation/config.yaml, got %s", savedPath)
	}
}

func TestSetupCmd_Execute(t *testing.T) {
	oldRunGenerateSurvey := survey.RunGenerateSurvey
	oldRunConfirm := runConfirm
	defer func() {
		survey.RunGenerateSurvey = oldRunGenerateSurvey
		runConfirm = oldRunConfirm
	}()

	oldOsMkdirAll := osMkdirAll
	osMkdirAll = func(path string, perm os.FileMode) error { return nil }
	defer func() { osMkdirAll = oldOsMkdirAll }()

	tests := []struct {
		name        string
		setup       func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver)
		wantErr     string
		wantInstall bool
		wantOut     []string
	}{
		{
			name: "Success - Install Later",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
					return &config.Config{}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = false; return nil }
			},
			wantInstall: false,
			wantOut: []string{
				"Setup complete. You can install the system service later",
				"To populate Home Assistant immediately without rebooting, run: grubstation boot push",
			},
		},
		{
			name: "Error - ensureSupport Fails (InitSystem)",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				deps.Registry = service_manager.NewRegistry() // Empty registry causes init system error
			},
			wantErr:     "no supported service manager detected",
			wantInstall: false,
		},
		{
			name: "Error - Generate Survey Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
					return nil, errors.New("survey failed")
				}
			},
			wantErr:     "survey failed",
			wantInstall: false,
		},
		{
			name: "Error - MkdirAll Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
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
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
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
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
					return &config.Config{}, nil
				}
				runConfirm = func(installNow *bool) error { return errors.New("confirm prompt failed") }
			},
			wantErr:     "confirm prompt failed",
			wantInstall: false,
		},
		{
			name: "Error - Perform Install Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub} // will fail since not mocked correctly
				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
					return &config.Config{
						Daemon: config.DaemonConfig{ReportBootOptions: true},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
			},
			wantErr:     "failed to install grub",
			wantInstall: false,
		},
		{
			name: "Success Install, Push Succeeds",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				// Mock successful grub setup
				oldExecLookPath := grub.ExecLookPath
				oldExecCommand := grub.ExecCommand
				oldHassPath := grub.HassGrubStationPath
				grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
				grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
					return exec.CommandContext(ctx, "/bin/true")
				}
				grub.HassGrubStationPath = t.TempDir() + "/99_ha_grub_os_reporter"
				t.Cleanup(func() {
					grub.ExecLookPath = oldExecLookPath
					grub.ExecCommand = oldExecCommand
					grub.HassGrubStationPath = oldHassPath
				})

				// Mock successful GetBootOptions and a working HA endpoint
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("OK"))
				}))
				t.Cleanup(ts.Close)

				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte("menuentry 'OS' {}"), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}

				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
					return &config.Config{
						HomeAssistant: config.HomeAssistantConfig{URL: ts.URL, WebhookID: "fake"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
			},
			wantInstall: true,
			wantOut: []string{
				"Installation completed successfully.",
				"Pushing initial boot options to Home Assistant...",
				"Successfully pushed initial state to Home Assistant.",
			},
		},
		{
			name: "Success Install, Push Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				// Mock successful grub setup
				oldExecLookPath := grub.ExecLookPath
				oldExecCommand := grub.ExecCommand
				oldHassPath := grub.HassGrubStationPath
				grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
				grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
					return exec.CommandContext(ctx, "/bin/true")
				}
				grub.HassGrubStationPath = t.TempDir() + "/99_ha_grub_os_reporter"
				t.Cleanup(func() {
					grub.ExecLookPath = oldExecLookPath
					grub.ExecCommand = oldExecCommand
					grub.HassGrubStationPath = oldHassPath
				})

				// Make GetBootOptions fail to trigger error in PushBootOptions
				deps.Grub = &grub.Grub{ConfigPath: "/non/existent/path"}

				survey.RunGenerateSurvey = func(ctx context.Context, deps survey.SurveyDeps) (*config.Config, error) {
					return &config.Config{
						HomeAssistant: config.HomeAssistantConfig{URL: "http://fake", WebhookID: "fake"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = true; return nil }
			},
			wantInstall: true,
			wantOut: []string{
				"Installation completed successfully.",
				"Pushing initial boot options to Home Assistant...",
				"Warning: failed to push initial state",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prevent mock bleed across test iterations
			origRunGenerateSurvey := survey.RunGenerateSurvey
			origRunConfirm := runConfirm
			defer func() {
				survey.RunGenerateSurvey = origRunGenerateSurvey
				runConfirm = origRunConfirm
			}()

			initMock := &mockInstallInitSystem{}
			initReg := service_manager.NewRegistry()
			initReg.Register("mock-init", func() service_manager.Manager { return initMock })

			sysResolver := &mockSystemResolver{
				saveConfigFunc: func(cfg *config.Config, path string) error { return nil },
			}

			deps := &CommandDeps{
				Config:         &config.Config{},
				Grub:           &grub.Grub{},
				Registry:       initReg,
				SystemResolver: sysResolver,
			}

			tt.setup(t, deps, initMock, sysResolver)

			cmd := NewSetupCmd(deps)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.Flags().String("config", "dummy.yaml", "")
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

func TestEnsureSupport(t *testing.T) {
	t.Run("InitSystem Not Supported", func(t *testing.T) {
		deps := &CommandDeps{}
		initReg := service_manager.NewRegistry()
		deps.Registry = initReg

		err := ensureSupport(context.Background(), deps)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "no supported service manager detected") {
			t.Errorf("expected init system not supported error, got %v", err)
		}
	})
}

func TestEnsureSupport_GenericErrors(t *testing.T) {
	t.Run("Grub Generic Error", func(t *testing.T) {
		deps := &CommandDeps{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := ensureSupport(ctx, deps)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("InitSystem Generic Error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		initReg := service_manager.NewRegistry()
		initReg.Register("systemd", func() service_manager.Manager { return &mockSurveyService{} })

		deps := &CommandDeps{
			Grub:     &grub.Grub{ConfigPath: t.TempDir() + "/grub.cfg"},
			Registry: initReg,
		}
		cancel()

		err := ensureSupport(ctx, deps)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestSurveyDepsAdapter(t *testing.T) {
	initReg := service_manager.NewRegistry()
	initReg.Register("systemd", func() service_manager.Manager { return &mockInstallInitSystem{} })
	deps := &CommandDeps{
		Registry:       initReg,
		SystemResolver: &mockSystemResolver{},
	}

	adapter := surveyDepsAdapter{deps: deps}
	if got := adapter.GetSystemResolver(); got != deps.SystemResolver {
		t.Fatalf("expected system resolver to be returned")
	}
}

func TestApplyCmd_StartServiceWarning(t *testing.T) {
	cfg := &config.Config{}

	initReg := service_manager.NewRegistry()
	initReg.Register("mock-init", func() service_manager.Manager { return &mockInstallInitSystem{startErr: errors.New("start failed")} })

	deps := &CommandDeps{
		Config:   cfg,
		Grub:     &grub.Grub{},
		Registry: initReg,
	}

	cmd := NewApplyCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Warning: failed to start service") {
		t.Fatalf("expected warning about service start failure, got %s", out.String())
	}
}

type mockSystemResolver struct {
	discoverHomeAssistantFunc func(ctx context.Context) (string, error)
	detectSystemHostnameFunc  func() (string, error)
	getWOLInterfacesFunc      func() ([]net.Interface, error)
	getIPv4InfoFunc           func(inf net.Interface) ([]string, map[string]string)
	getFQDNOptsFunc           func(hostname string) string
	saveConfigFunc            func(cfg *config.Config, path string) error
	discoverGrubConfigFunc    func(ctx context.Context) (string, error)
}

func (m *mockSystemResolver) DiscoverHomeAssistant(ctx context.Context) (string, error) {
	if m.discoverHomeAssistantFunc != nil {
		return m.discoverHomeAssistantFunc(ctx)
	}
	return "http://homeassistant.local:8123", nil
}

func (m *mockSystemResolver) DiscoverGrubConfig(ctx context.Context) (string, error) {
	if m.discoverGrubConfigFunc != nil {
		return m.discoverGrubConfigFunc(ctx)
	}
	return "/boot/grub/grub.cfg", nil
}

func (m *mockSystemResolver) DetectSystemHostname() (string, error) {
	if m.detectSystemHostnameFunc != nil {
		return m.detectSystemHostnameFunc()
	}
	return "test-host", nil
}

func (m *mockSystemResolver) GetWOLInterfaces() ([]net.Interface, error) {
	if m.getWOLInterfacesFunc != nil {
		return m.getWOLInterfacesFunc()
	}
	return []net.Interface{{Name: "eth0", HardwareAddr: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}}, nil
}

func (m *mockSystemResolver) GetIPv4Info(inf net.Interface) ([]string, map[string]string) {
	if m.getIPv4InfoFunc != nil {
		return m.getIPv4InfoFunc(inf)
	}
	return []string{"192.168.1.100"}, map[string]string{"192.168.1.100": "192.168.1.255"}
}

func (m *mockSystemResolver) GetFQDN(hostname string) string {
	if m.getFQDNOptsFunc != nil {
		return m.getFQDNOptsFunc(hostname)
	}
	return hostname + ".local"
}

func (m *mockSystemResolver) SaveConfig(cfg *config.Config, path string) error {
	if m.saveConfigFunc != nil {
		return m.saveConfigFunc(cfg, path)
	}
	return nil
}

type mockSurveyService struct{}

func (m *mockSurveyService) Name() string                                         { return "systemd" }
func (m *mockSurveyService) IsActive(ctx context.Context) bool                    { return true }
func (m *mockSurveyService) Install(ctx context.Context, configPath string) error { return nil }
func (m *mockSurveyService) Uninstall(ctx context.Context) error                  { return nil }
func (m *mockSurveyService) Start(ctx context.Context) error                      { return nil }
func (m *mockSurveyService) Stop(ctx context.Context) error                       { return nil }
