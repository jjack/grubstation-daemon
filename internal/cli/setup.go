package cli

import (
	"fmt"
	"path/filepath"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
)

func performInstall(cmd *cobra.Command, deps *CommandDeps, cfgFile string) error {
	bl, err := deps.Bootloader(cmd.Context())
	if err != nil {
		return err
	}

	sys, err := deps.InitSystem(cmd.Context())
	if err != nil {
		return err
	}

	macAddress := deps.Config.Host.MACAddress
	haURL := deps.Config.HomeAssistant.URL
	webhookID := deps.Config.HomeAssistant.WebhookID

	cmd.Printf("Installing into bootloader: %s\n", bl.Name())
	if err := bl.Setup(cmd.Context(), macAddress, haURL, webhookID); err != nil {
		return fmt.Errorf("failed to install bootloader: %w", err)
	}

	absConfig, err := filepath.Abs(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	cmd.Printf("Installing into init system: %s\n", sys.Name())
	if err := sys.Setup(cmd.Context(), absConfig); err != nil {
		return fmt.Errorf("failed to install init system: %w", err)
	}

	cmd.Println("Installation completed successfully.")

	// Optional interface check to see if the bootloader has any hardware warnings to share
	if warner, ok := bl.(interface{ SetupWarning() string }); ok {
		if warning := warner.SetupWarning(); warning != "" {
			cmd.Printf("\nNote: %s\n", warning)
		}
	}
	return nil
}

func NewApplyCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply the current configuration to the bootloader and init system",
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
		Title("Would you like to install the bootloader and init system hooks now?").
		Description("(Requires root/sudo privileges)").
		Value(installNow).
		Run()
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
				return performInstall(cmd, deps, cfgPath)
			}

			cmd.Println("\nSetup complete. You can apply the system hooks later by running 'remote-boot-agent apply'")
			return nil
		},
	}

	// In setup, we default to the system path instead of local,
	// because this is expected to be run as sudo for permanent installation.
	cmd.Flags().String("config", "/etc/remote-boot-agent/config.yaml", "Path to save the generated config file")
	return cmd
}
