package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/jjack/grubstation-daemon/internal/cli/survey"
	"github.com/jjack/grubstation-daemon/internal/config"
	"github.com/jjack/grubstation-daemon/internal/grub"
	"github.com/jjack/grubstation-daemon/internal/reporter"
	"github.com/jjack/grubstation-daemon/internal/service_manager"
	"github.com/spf13/cobra"
)

var (
	osMkdirAll       = os.MkdirAll
	checkWriteAccess = defaultCheckWriteAccess
)

func performInstall(cmd *cobra.Command, deps *CommandDeps, cfgFile string) error {
	mgr, err := deps.Manager(cmd.Context())
	if err != nil {
		return err
	}

	absConfig, err := filepath.Abs(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	if deps.Config.Daemon.ReportBootOptions {
		opts := grub.SetupOptions{
			TargetMAC:       deps.Config.Host.MACAddress,
			TargetURL:       deps.Config.HomeAssistant.URL,
			AuthToken:       deps.Config.HomeAssistant.WebhookID,
			WaitTimeSeconds: deps.Config.Grub.WaitTimeSeconds,
		}

		cmd.Printf("Installing into grub...\n")
		if err := deps.Grub.Setup(cmd.Context(), opts); err != nil {
			return fmt.Errorf("failed to install grub: %w", err)
		}
	}

	cmd.Printf("Installing into service manager: %s\n", mgr.Name())
	if err := mgr.Install(cmd.Context(), absConfig); err != nil {
		return fmt.Errorf("failed to install manager: %w", err)
	}

	cmd.Printf("Starting service...\n")
	if err := mgr.Start(cmd.Context()); err != nil {
		cmd.Printf("Warning: failed to start service: %v\n", err)
	}

	cmd.Println("Installation completed successfully.")

	if deps.Config.Daemon.ReportBootOptions {
		if warning := deps.Grub.SetupWarning(); warning != "" {
			cmd.Printf("\nNote: %s\n", warning)
		}
	}
	return nil
}

func NewApplyCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply the current configuration to the grub and init system",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return fmt.Errorf("failed to read config flag: %w", err)
			}
			return performInstall(cmd, deps, cfgFile)
		},
	}
}

var runConfirm = func(installNow *bool) error {
	return huh.NewConfirm().
		Title("Would you like to install the service now?").
		Description("(Requires root/sudo privileges)").
		Value(installNow).
		Run()
}

func ensureSupport(ctx context.Context, deps *CommandDeps) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := deps.Registry.Detect(ctx)
	if err != nil {
		if errors.Is(err, service_manager.ErrNotSupported) {
			supported := strings.Join(deps.Registry.SupportedServices(), ", ")
			return fmt.Errorf("no supported service manager detected. Please ensure you have one of the following installed: %s", supported)
		}
		return err
	}
	return nil
}

func defaultCheckWriteAccess(path string) error {
	// If the file exists, check if it's writable.
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("config file %s is not writable (try running with sudo?): %w", path, err)
		}
		f.Close()
		return nil
	}

	// File doesn't exist, check directory for write access.
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".grubstation-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("no write access to directory %s (try running with sudo?): %w", dir, err)
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

type surveyDepsAdapter struct {
	deps *CommandDeps
}

func (a surveyDepsAdapter) GetSystemResolver() survey.SystemResolver {
	return a.deps.SystemResolver
}

func NewSetupCmd(deps *CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Run the automated setup wizard to configure and install the agent",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil // Override root config loading, we are generating it from scratch
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureSupport(cmd.Context(), deps); err != nil {
				return err
			}

			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil || cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}

			if err := osMkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			if err := checkWriteAccess(cfgPath); err != nil {
				return err
			}

			// Clear the terminal screen before starting the interactive prompts
			cmd.Print("\033[H\033[2J")

			cfg, err := survey.RunGenerateSurvey(cmd.Context(), surveyDepsAdapter{deps: deps})
			if err != nil {
				return err
			}

			survey.PrintConfigSummary(cmd, cfg, cfgPath)

			if err := deps.SystemResolver.SaveConfig(cfg, cfgPath); err != nil {
				return err
			}

			var installNow bool
			if err := runConfirm(&installNow); err != nil {
				return err
			}

			if installNow {
				cmd.Println("\nProceeding with installation...")
				// We update the deps config with our freshly generated config so the installer can use it
				*deps.Config = *cfg
				if err := performInstall(cmd, deps, cfgPath); err != nil {
					return err
				}

				cmd.Println("\nPushing initial boot options to Home Assistant...")
				mgr, _ := deps.Manager(cmd.Context())
				mgrName := ""
				if mgr != nil {
					mgrName = mgr.Name()
				}
				rep := reporter.New(deps.Config, deps.Grub, mgrName)
				if err := rep.PushBootOptions(cmd.Context(), ""); err != nil {
					cmd.Printf("Warning: failed to push initial state to Home Assistant: %v\n", err)
					cmd.Println("You can try pushing manually later with 'grubstation options push'")
				} else {
					cmd.Println("Successfully pushed initial state to Home Assistant.")
				}
				return nil
			}

			cmd.Println("\nSetup complete. You can apply the system hooks later by running 'grubstation apply'")
			cmd.Println("To populate Home Assistant immediately without rebooting, run: grubstation options push")
			return nil
		},
	}

	return cmd
}
