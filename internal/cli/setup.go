package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/jjack/remote-boot-agent/internal/grub"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/spf13/cobra"
)

var osMkdirAll = os.MkdirAll

func performInstall(cmd *cobra.Command, deps *CommandDeps, cfgFile string) error {
	sys, err := deps.InitSystem(cmd.Context())
	if err != nil {
		return err
	}

	absConfig, err := filepath.Abs(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	opts := grub.SetupOptions{
		TargetMAC: deps.Config.Host.MACAddress,
		TargetURL: deps.Config.HomeAssistant.URL,
		AuthToken: deps.Config.HomeAssistant.WebhookID,
	}

	cmd.Printf("Installing into grub...\n")
	if err := deps.Grub.Setup(cmd.Context(), opts); err != nil {
		return fmt.Errorf("failed to install grub: %w", err)
	}

	cmd.Printf("Installing into init system: %s\n", sys.Name())
	if err := sys.Setup(cmd.Context(), absConfig); err != nil {
		return fmt.Errorf("failed to install init system: %w", err)
	}

	cmd.Println("Installation completed successfully.")

	if warning := deps.Grub.SetupWarning(); warning != "" {
		cmd.Printf("\nNote: %s\n", warning)
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
		Title("Would you like to install the grub and init system hooks now?").
		Description("(Requires root/sudo privileges)").
		Value(installNow).
		Run()
}

func ensureSupport(ctx context.Context, deps *CommandDeps) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := deps.InitRegistry.Detect(ctx)
	if err != nil {
		if errors.Is(err, initsystem.ErrNotSupported) {
			supported := strings.Join(deps.InitRegistry.SupportedInitSystems(), ", ")
			return fmt.Errorf("no supported init system detected. Please ensure you have one of the following installed: %s", supported)
		}
		return err
	}
	return nil
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

			// Clear the terminal screen before starting the interactive prompts
			cmd.Print("\033[H\033[2J")

			cfg, err := runGenerateSurvey(cmd.Context(), deps)
			if err != nil {
				return err
			}

			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				cfgPath = "/etc/remote-boot-agent/config.yaml"
			}

			printConfigSummary(cmd, cfg, cfgPath)

			if err := osMkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

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
				if err := PushBootOptions(cmd.Context(), deps); err != nil {
					cmd.Printf("Warning: failed to push initial state to Home Assistant: %v\n", err)
					cmd.Println("You can try pushing manually later with 'remote-boot-agent options push'")
				} else {
					cmd.Println("Successfully pushed initial state to Home Assistant.")
				}
				return nil
			}

			cmd.Println("\nSetup complete. You can apply the system hooks later by running 'remote-boot-agent apply'")
			cmd.Println("To populate Home Assistant immediately without rebooting, run: remote-boot-agent options push")
			return nil
		},
	}

	return cmd
}
