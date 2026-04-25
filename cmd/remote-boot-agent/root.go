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

			cfg, err := config.Load(cfgFile, cmd.Flags())
			if err != nil {
				return err
			}

			cli.Config = cfg
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	config.RegisterFlags(rootCmd.PersistentFlags())

	// Dependency providers for lazy evaluation to avoid tight coupling in commands
	getBootloader := func() (bootloader.Bootloader, error) {
		return ResolveBootloader(cli.Config.Bootloader.Name)
	}
	getBootloaderConfig := func() config.BootloaderConfig {
		return cli.Config.Bootloader
	}
	getHAConfig := func() config.HomeAssistantConfig {
		return cli.Config.HomeAssistant
	}
	getHostConfig := func() config.HostConfig {
		return cli.Config.Host
	}

	rootCmd.AddCommand(NewGetBootOptions(getBootloader, getBootloaderConfig))
	rootCmd.AddCommand(NewPushBootOptions(getBootloader, getBootloaderConfig, getHAConfig, getHostConfig))
	rootCmd.AddCommand(NewGetRemoteBootOption(getBootloader, getHAConfig, getHostConfig))
	rootCmd.AddCommand(NewGenerateConfigCmd())

	// get rid of the completion command because it doesn't make sense here
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	cli.RootCmd = rootCmd
	return cli
}

func (cli *CLI) Execute() error {
	return cli.RootCmd.Execute()
}

func ResolveBootloader(name string) (bootloader.Bootloader, error) {
	if name != "" {
		bl := bootloader.Get(name)
		if bl == nil {
			return nil, fmt.Errorf("specified bootloader %s not supported", name)
		}
		return bl, nil
	}

	bl, err := bootloader.Detect()
	if err != nil {
		return nil, fmt.Errorf("bootloader detection failed: %w", err)
	}
	return bl, nil
}
