package cli

import (
	"fmt"
	"os"

	"github.com/jjack/grubstation/internal/config"
	"github.com/spf13/cobra"
)

func NewConfigInitCmd(deps *CommandDeps) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a default config.yaml in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(output); err == nil {
				return fmt.Errorf("config file already exists: %s", output)
			}

			cfg := &config.Config{
				Host: config.HostConfig{
					Address:    "127.0.0.1",
					MACAddress: "00:00:00:00:00:00",
				},
				HomeAssistant: config.HomeAssistantConfig{
					URL:       "http://homeassistant.local:8123",
					WebhookID: "CHANGE_ME",
				},
				Daemon: config.DaemonConfig{
					Port:              config.DefaultAgentPort,
					ReportBootOptions: true,
				},
				Grub: &config.GrubConfig{
					WaitTimeSeconds: config.DefaultGrubWaitSeconds,
				},
			}

			if err := config.Save(cfg, output); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			cmd.Printf("Default configuration generated at: %s\n", output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "config.yaml", "Path to write the default configuration")

	return cmd
}
