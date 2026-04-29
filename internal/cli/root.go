package cli

import (
	"context"
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/spf13/cobra"
)

type CLI struct {
	Config  *config.Config
	RootCmd *cobra.Command
}

type CommandDeps struct {
	Config             *config.Config
	BootloaderRegistry *bootloader.Registry
	InitRegistry       *initsystem.Registry
}

func (d *CommandDeps) Bootloader(ctx context.Context) (bootloader.Bootloader, error) {
	return ResolveBootloader(ctx, d.Config.Bootloader.Name, d.BootloaderRegistry)
}

func (d *CommandDeps) InitSystem(ctx context.Context) (initsystem.InitSystem, error) {
	return ResolveInitSystem(ctx, d.Config.InitSystem.Name, d.InitRegistry)
}

func NewCLI() *CLI {
	cli := &CLI{}

	deps := &CommandDeps{
		Config:             &config.Config{},
		BootloaderRegistry: bootloader.NewRegistry(),
		InitRegistry:       initsystem.NewRegistry(),
	}

	var cfgFile string

	rootCmd := &cobra.Command{
		Use:           "remote-boot-agent",
		Short:         "remote-boot-agent reads boot configurations and posts them to Home Assistant",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Don't load the config if we're also trying to generate it
			if cmd.CommandPath() == "remote-boot-agent config generate" {
				return nil
			}

			cfg, err := config.Load(cfgFile, cmd.Flags())
			if err != nil {
				return err
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			*deps.Config = *cfg
			cli.Config = deps.Config
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "./config.yaml", "config file")
	rootCmd.PersistentFlags().String("mac", "", "MAC Address override")
	rootCmd.PersistentFlags().String("hostname", "", "Hostname override")
	rootCmd.PersistentFlags().String("broadcast-address", "", "Broadcast address override for WOL")
	rootCmd.PersistentFlags().Int("wol-port", 9, "Broadcast port override for WOL")
	rootCmd.PersistentFlags().String("bootloader", "", "Bootloader type override (e.g., grub)")
	rootCmd.PersistentFlags().String("bootloader-path", "", "Bootloader config path override")
	rootCmd.PersistentFlags().String("init-system", "", "Initsystem override (e.g., systemd)")
	rootCmd.PersistentFlags().String("hass-url", "", "Home Assistant URL override")
	rootCmd.PersistentFlags().String("hass-webhook", "", "Home Assistant Webhook ID override")

	deps.BootloaderRegistry.Register("grub", bootloader.NewGrub)
	deps.InitRegistry.Register("systemd", initsystem.NewSystemd)

	rootCmd.AddCommand(NewListCmd(deps))
	rootCmd.AddCommand(NewPushCmd(deps))
	rootCmd.AddCommand(NewConfigCmd(deps))
	rootCmd.AddCommand(NewInstallCmd(deps))

	// get rid of the completion command because it doesn't make sense here
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	cli.RootCmd = rootCmd
	return cli
}

func (cli *CLI) Execute() error {
	return cli.RootCmd.Execute()
}

func ResolveBootloader(ctx context.Context, name string, registry *bootloader.Registry) (bootloader.Bootloader, error) {
	if name != "" {
		bl := registry.Get(name)
		if bl == nil {
			return nil, fmt.Errorf("specified bootloader %s not supported", name)
		}
		return bl, nil
	}

	bl, err := registry.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("bootloader detection failed: %w", err)
	}
	return bl, nil
}

func ResolveInitSystem(ctx context.Context, name string, registry *initsystem.Registry) (initsystem.InitSystem, error) {
	if name != "" {
		sys := registry.Get(name)
		if sys == nil {
			return nil, fmt.Errorf("specified init system %s not supported", name)
		}
		return sys, nil
	}

	sys, err := registry.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("init system detection failed: %w", err)
	}
	return sys, nil
}
