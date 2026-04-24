package main

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/spf13/cobra"
)

type CLI struct {
	Config  *config.Config
	RootCmd *cobra.Command
}

func applyFlagOverrides(cmd *cobra.Command, config *config.Config) {
	if mac, _ := cmd.Flags().GetString("mac"); mac != "" {
		config.Host.MACAddress = mac
	}
	if hostname, _ := cmd.Flags().GetString("hostname"); hostname != "" {
		config.Host.Hostname = hostname
	}
	if bl, _ := cmd.Flags().GetString("bootloader"); bl != "" {
		config.Bootloader.Name = bl
	}
	if blConfig, _ := cmd.Flags().GetString("bootloader-path"); blConfig != "" {
		config.Bootloader.ConfigPath = blConfig
	}
	if haURL, _ := cmd.Flags().GetString("hass-url"); haURL != "" {
		config.HomeAssistant.URL = haURL
	}
	if haWebhook, _ := cmd.Flags().GetString("hass-webhook"); haWebhook != "" {
		config.HomeAssistant.WebhookID = haWebhook
	}
}

func NewCLI() *CLI {
	cli := &CLI{}

	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "remote-boot-agent",
		Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Don't load the config if we're also trying to generate it
			if cmd.Name() == "generate-config" {
				return nil
			}

			config, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			applyFlagOverrides(cmd, config)

			cli.Config = config
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().String("mac", "", "MAC Address override")
	rootCmd.PersistentFlags().String("hostname", "", "Hostname override")
	rootCmd.PersistentFlags().String("bootloader", "", "Bootloader type override (e.g., grub)")
	rootCmd.PersistentFlags().String("bootloader-path", "", "Bootloader config path override")
	rootCmd.PersistentFlags().String("hass-url", "", "Home Assistant URL override")
	rootCmd.PersistentFlags().String("hass-webhook", "", "Home Assistant Webhook ID override")

	rootCmd.AddCommand(NewGetBootOptions(cli))
	rootCmd.AddCommand(NewPushBootOptions(cli))
	rootCmd.AddCommand(NewGetRemoteBootOption(cli))
	rootCmd.AddCommand(NewGenerateConfigCmd(cli))

	// get rid of the completion command because it doesn't make sense here
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	cli.RootCmd = rootCmd
	return cli
}

func (cli *CLI) Execute() error {
	return cli.RootCmd.Execute()
}

func ResolveBootloader(config *config.Config) (bootloader.Bootloader, error) {
	if config.Bootloader.Name != "" {
		bl := bootloader.Get(config.Bootloader.Name)
		if bl == nil {
			return nil, fmt.Errorf("specified bootloader %s not supported", config.Bootloader.Name)
		}
		return bl, nil
	}

	bl, err := bootloader.Detect()
	if err != nil {
		return nil, fmt.Errorf("bootloader detection failed: %w", err)
	}
	return bl, nil
}
