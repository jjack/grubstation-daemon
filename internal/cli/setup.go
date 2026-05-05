package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewSetupCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Installs and configures the agent into the bootloader and init system",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			cfgFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return fmt.Errorf("failed to read config flag: %w", err)
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
			return nil
		},
	}
}
