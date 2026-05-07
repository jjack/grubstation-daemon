package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jjack/grub-os-reporter/internal/config"
	"github.com/jjack/grub-os-reporter/internal/grub"
	"github.com/jjack/grub-os-reporter/internal/initsystem"
)

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

func TestApplyCmd_GrubError(t *testing.T) {
	cfg := &config.Config{
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: "/invalid/path/grub.cfg"}, InitRegistry: initReg}
	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to install grub") {
		t.Fatalf("expected grub install error, got %v", err)
	}
}

func TestApplyCmd_MissingConfigFlag(t *testing.T) {
	cfg := &config.Config{
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{}, InitRegistry: initReg}
	cmd := NewApplyCmd(deps) // Missing binding the "config" flag locally

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "flag accessed but not defined") {
		t.Fatalf("expected flag missing error, got %v", err)
	}
}

func TestApplyCmd_InitSystemResolveError(t *testing.T) {
	cfg := &config.Config{
		InitSystem: config.InitSystemConfig{Name: "invalid-init"},
	}

	initReg := initsystem.NewRegistry()

	deps := &CommandDeps{
		Config:       cfg,
		Grub:         &grub.Grub{},
		InitRegistry: initReg,
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
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{
		Config:       cfg,
		Grub:         &grub.Grub{},
		InitRegistry: initReg,
	}

	cmd := NewApplyCmd(deps)
	cmd.Flags().String("config", "relative-config.yaml", "") // Must be relative to trigger os.Getwd()

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to resolve config path") {
		t.Fatalf("expected filepath.Abs error, got %v", err)
	}
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
			InitSystem: config.InitSystemConfig{Name: "mock-init"},
		}, nil
	}
	runConfirm = func(installNow *bool) error { *installNow = false; return nil }

	initMock := &mockInstallInitSystem{}
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
		Config:         &config.Config{},
		Grub:           &grub.Grub{},
		InitRegistry:   initReg,
		SystemResolver: sysResolver,
	}

	cmd := NewSetupCmd(deps)
	cmd.ResetFlags() // Strip the "config" flag to force GetString to error out

	_ = cmd.Execute() // We ignore execution err if any to verify the fallback below

	if savedPath != "/etc/grub-os-reporter/config.yaml" {
		t.Errorf("expected default fallback path /etc/grub-os-reporter/config.yaml, got %s", savedPath)
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
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						InitSystem: config.InitSystemConfig{Name: "mock-init"},
					}, nil
				}
				runConfirm = func(installNow *bool) error { *installNow = false; return nil }
			},
			wantInstall: false,
			wantOut: []string{
				"Setup complete. You can apply the system hooks later",
				"To populate Home Assistant immediately without rebooting, run: grub-os-reporter options push",
			},
		},
		{
			name: "Error - ensureSupport Fails (InitSystem)",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				deps.InitRegistry = initsystem.NewRegistry() // Empty registry causes init system error
			},
			wantErr:     "no supported init system detected",
			wantInstall: false,
		},
		{
			name: "Error - Generate Survey Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
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
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
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
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{}, nil
				}
				runConfirm = func(installNow *bool) error { return errors.New("confirm prompt failed") }
			},
			wantErr:     "confirm prompt failed",
			wantInstall: false,
		},
		{
			name: "Error - Perform Install InitSystem Resolve Fails",
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
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
			setup: func(t *testing.T, deps *CommandDeps, initMock *mockInstallInitSystem, resolver *mockSystemResolver) {
				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte(""), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub} // will fail since not mocked correctly
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						InitSystem: config.InitSystemConfig{Name: "mock-init"},
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
				oldHassPath := grub.HassGrubOSReporterPath
				grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
				grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
					return exec.CommandContext(ctx, "/bin/true")
				}
				grub.HassGrubOSReporterPath = t.TempDir() + "/99_ha_grub_os_reporter"
				t.Cleanup(func() {
					grub.ExecLookPath = oldExecLookPath
					grub.ExecCommand = oldExecCommand
					grub.HassGrubOSReporterPath = oldHassPath
				})

				// Mock successful GetBootOptions and a working HA endpoint
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(ts.Close)

				tempGrub := t.TempDir() + "/grub.cfg"
				_ = os.WriteFile(tempGrub, []byte("menuentry 'OS' {}"), 0o644)
				deps.Grub = &grub.Grub{ConfigPath: tempGrub}

				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						InitSystem:    config.InitSystemConfig{Name: "mock-init"},
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
				oldHassPath := grub.HassGrubOSReporterPath
				grub.ExecLookPath = func(file string) (string, error) { return "/bin/true", nil }
				grub.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
					return exec.CommandContext(ctx, "/bin/true")
				}
				grub.HassGrubOSReporterPath = t.TempDir() + "/99_ha_grub_os_reporter"
				t.Cleanup(func() {
					grub.ExecLookPath = oldExecLookPath
					grub.ExecCommand = oldExecCommand
					grub.HassGrubOSReporterPath = oldHassPath
				})

				// Make GetBootOptions fail to trigger error in PushBootOptions
				deps.Grub = &grub.Grub{ConfigPath: "/non/existent/path"}

				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						InitSystem:    config.InitSystemConfig{Name: "mock-init"},
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
			origRunGenerateSurvey := runGenerateSurvey
			origRunConfirm := runConfirm
			defer func() {
				runGenerateSurvey = origRunGenerateSurvey
				runConfirm = origRunConfirm
			}()

			initMock := &mockInstallInitSystem{}
			initReg := initsystem.NewRegistry()
			initReg.Register("mock-init", func() initsystem.InitSystem { return initMock })

			sysResolver := &mockSystemResolver{
				saveConfigFunc: func(cfg *config.Config, path string) error { return nil },
			}

			deps := &CommandDeps{
				Config:         &config.Config{},
				Grub:           &grub.Grub{},
				InitRegistry:   initReg,
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
		deps := setupSurveyDeps(t)
		initReg := initsystem.NewRegistry()
		deps.InitRegistry = initReg

		err := ensureSupport(context.Background(), deps)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "no supported init system detected") {
			t.Errorf("expected init system not supported error, got %v", err)
		}
	})
}

func TestEnsureSupport_GenericErrors(t *testing.T) {
	t.Run("Grub Generic Error", func(t *testing.T) {
		deps := setupSurveyDeps(t)
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

		initReg := initsystem.NewRegistry()
		initReg.Register("systemd", func() initsystem.InitSystem { return &mockSurveyInitSystem{} })

		deps := &CommandDeps{
			Grub:         &grub.Grub{ConfigPath: setupSurveyDeps(t).Grub.ConfigPath},
			InitRegistry: initReg,
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
