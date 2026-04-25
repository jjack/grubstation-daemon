package cli

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type CLI struct {
	Config  *config.Config
	RootCmd *cobra.Command
}

type CommandDeps struct {
	Config   *config.Config
	Registry *bootloader.Registry
}

func (d *CommandDeps) Bootloader() (bootloader.Bootloader, error) {
	return ResolveBootloader(d.Config.Bootloader.Name, d.Registry)
}

func applyFlagOverrides(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "mac":
			cfg.Host.MACAddress = f.Value.String()
		case "hostname":
			cfg.Host.Hostname = f.Value.String()
		case "bootloader":
			cfg.Bootloader.Name = f.Value.String()
		case "bootloader-path":
			cfg.Bootloader.ConfigPath = f.Value.String()
		case "hass-url":
			cfg.HomeAssistant.URL = f.Value.String()
		case "hass-webhook":
			cfg.HomeAssistant.WebhookID = f.Value.String()
		}
	})
}

func NewCLI() *CLI {
	cli := &CLI{}

	deps := &CommandDeps{
		Config:   &config.Config{},
		Registry: bootloader.NewRegistry(),
	}

	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "remote-boot-agent",
		Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Don't load the config if we're also trying to generate it
			if cmd.Name() == "generate-config" {
				return nil
			}

			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			applyFlagOverrides(cmd, cfg)
			*deps.Config = *cfg
			cli.Config = deps.Config
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

	deps.Registry.Register("grub", bootloader.NewGrub)

	rootCmd.AddCommand(NewListCmd(deps))
	rootCmd.AddCommand(NewPushCmd(deps))
	rootCmd.AddCommand(NewGenerateConfigCmd())

	// get rid of the completion command because it doesn't make sense here
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	cli.RootCmd = rootCmd
	return cli
}

func (cli *CLI) Execute() error {
	return cli.RootCmd.Execute()
}

func ResolveBootloader(name string, registry *bootloader.Registry) (bootloader.Bootloader, error) {
	if name != "" {
		bl := registry.Get(name)
		if bl == nil {
			return nil, fmt.Errorf("specified bootloader %s not supported", name)
		}
		return bl, nil
	}

	bl, err := registry.Detect()
	if err != nil {
		return nil, fmt.Errorf("bootloader detection failed: %w", err)
	}
	return bl, nil
}
