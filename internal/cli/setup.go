package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jjack/grubstation-daemon/internal/cli/survey"
	"github.com/jjack/grubstation-daemon/internal/config"
	"github.com/jjack/grubstation-daemon/internal/grub"
	"github.com/jjack/grubstation-daemon/internal/reporter"
	"github.com/jjack/grubstation-daemon/internal/servicemanager"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yarlson/tap"
)

var osMkdirAll = os.MkdirAll

func performInstall(cmd *cobra.Command, deps *CommandDeps, cfgFile string) error {
	mgr, err := deps.Manager(cmd.Context())
	if err != nil {
		return err
	}

	if err := mgr.CheckPermissions(cmd.Context()); err != nil {
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

		tap.Message("Installing into grub...", tap.MessageOptions{
			Hint: deps.Grub.SetupWarning(),
		})
		if err := deps.Grub.Setup(cmd.Context(), opts); err != nil {
			return fmt.Errorf("failed to install grub: %w", err)
		}

		tap.Message("Pushing initial boot options to Home Assistant...")
		activeMgr, _ := deps.Manager(cmd.Context())
		mgrName := ""
		if activeMgr != nil {
			mgrName = activeMgr.Name()
		}
		rep := reporter.New(deps.Config, deps.Grub, mgrName)
		if err := rep.PushBootOptions(cmd.Context(), ""); err != nil {
			return err
		}
		tap.Message("Successfully pushed initial state to Home Assistant.")
	}

	tap.Message(fmt.Sprintf("Installing into service manager: %s", mgr.Name()))
	if err := mgr.Install(cmd.Context(), absConfig); err != nil {
		return fmt.Errorf("failed to install manager: %w", err)
	}

	tap.Message("Starting service...")
	if err := mgr.Start(cmd.Context()); err != nil {
		return fmt.Errorf("failed to start service: %v", err)
	}

	tap.Message("Installation completed successfully.")

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
			tap.Intro("GrubStation Apply")
			err = performInstall(cmd, deps, cfgFile)
			if err != nil {
				return err
			}
			tap.Outro("Apply complete!")
			return nil
		},
	}
}

func ensureSupport(ctx context.Context, deps *CommandDeps) (servicemanager.Manager, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	mgr, err := deps.Registry.Detect(ctx)
	if err != nil {
		if errors.Is(err, servicemanager.ErrNotSupported) {
			supported := strings.Join(deps.Registry.SupportedServices(), ", ")
			return nil, fmt.Errorf("no supported service manager detected. Please ensure you have one of the following installed: %s", supported)
		}
		return nil, err
	}
	return mgr, nil
}

type surveyDepsAdapter struct {
	deps *CommandDeps
}

func (a surveyDepsAdapter) GetSystemResolver() survey.SystemResolver {
	return a.deps.SystemResolver
}

func (a surveyDepsAdapter) IsInstalled(ctx context.Context) (bool, error) {
	mgr, err := ensureSupport(ctx, a.deps)
	if err != nil {
		return false, err
	}
	return mgr.IsInstalled(ctx)
}

func NewSetupCmd(deps *CommandDeps) *cobra.Command {
	var applyOnly bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Run the automated setup wizard to configure and install the agent",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if applyOnly {
				// For apply, we WANT the default config loading to happen
				return nil
			}
			return nil // Override root config loading, we are generating it from scratch
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if applyOnly {
				cfgPath, _ := cmd.Flags().GetString("config")
				return performInstall(cmd, deps, cfgPath)
			}

			if _, err := ensureSupport(cmd.Context(), deps); err != nil {
				return err
			}

			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil || cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}

			var currentPort int
			if _, err := os.Stat(cfgPath); err == nil {
				v := viper.New()
				v.SetConfigFile(cfgPath)
				if err := v.ReadInConfig(); err == nil {
					currentPort = v.GetInt("daemon.port")
				}
			}

			if err := osMkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Clear the terminal screen before starting the interactive wizard
			cmd.Print("\033[H\033[2J")

			tap.Intro("GrubStation Setup")

			cfg, isDryRun, err := survey.RunGenerateSurvey(cmd.Context(), surveyDepsAdapter{deps: deps}, true, currentPort)
			if err != nil {
				if errors.Is(err, survey.ErrAborted) {
					tap.Message("Setup aborted.")
					tap.Outro("Goodbye!")
					return nil
				}
				return err
			}

			if isDryRun {
				tap.Message("Dry run completed. Configuration shown above was not saved.")
				tap.Outro("Dry run finished")
				return nil
			}

			if err := deps.SystemResolver.SaveConfig(cfg, cfgPath); err != nil {
				return err
			}

			tap.Outro("Configuration setup complete.", tap.MessageOptions{
				Hint: fmt.Sprintf("saved to: %s", cfgPath),
			})

			tap.Intro("Proceeding with installation...")
			// We update the deps config with our freshly generated config so the installer can use it
			*deps.Config = *cfg
			if err := performInstall(cmd, deps, cfgPath); err != nil {
				return err
			}

			tap.Message("Pushing initial boot options to Home Assistant...")
			activeMgr, _ := deps.Manager(cmd.Context())
			mgrName := ""
			if activeMgr != nil {
				mgrName = activeMgr.Name()
			}
			rep := reporter.New(deps.Config, deps.Grub, mgrName)
			if err := rep.PushBootOptions(cmd.Context(), ""); err != nil {
				return err
			}
			tap.Message("Successfully pushed initial state to Home Assistant.")

			tap.Outro("Setup complete! To populate Home Assistant again without rebooting, run: grubstation boot push")
			return nil
		},
	}

	cmd.Flags().BoolVar(&applyOnly, "apply", false, "Skip survey and install service based on current config")

	return cmd
}
