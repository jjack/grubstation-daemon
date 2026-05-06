package cli

import (
	"context"
	"fmt"
	"net"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/jjack/remote-boot-agent/internal/system"
	"github.com/spf13/cobra"
)

type CLI struct {
	Config  *config.Config
	RootCmd *cobra.Command
}

type SystemResolver interface {
	DiscoverHomeAssistant(ctx context.Context) (string, error)
	DetectSystemHostname() (string, error)
	GetWOLInterfaces() ([]net.Interface, error)
	GetIPv4Info(inf net.Interface) ([]string, map[string]string)
	GetFQDN(hostname string) string
	SaveConfig(cfg *config.Config, path string) error
}

type DefaultSystemResolver struct{}

func (d *DefaultSystemResolver) DiscoverHomeAssistant(ctx context.Context) (string, error) {
	return homeassistant.Discover(ctx)
}

func (d *DefaultSystemResolver) DetectSystemHostname() (string, error) {
	return system.DetectHostname()
}

func (d *DefaultSystemResolver) GetWOLInterfaces() ([]net.Interface, error) {
	return system.GetWOLInterfaces()
}

func (d *DefaultSystemResolver) GetIPv4Info(inf net.Interface) ([]string, map[string]string) {
	return system.GetIPv4Info(inf)
}
func (d *DefaultSystemResolver) GetFQDN(hostname string) string { return system.GetFQDN(hostname) }
func (d *DefaultSystemResolver) SaveConfig(cfg *config.Config, path string) error {
	return config.Save(cfg, path)
}

type CommandDeps struct {
	Config             *config.Config
	BootloaderRegistry *bootloader.Registry
	InitRegistry       *initsystem.Registry
	SystemResolver     SystemResolver
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
		SystemResolver:     &DefaultSystemResolver{},
	}

	var cfgFile string

	rootCmd := &cobra.Command{
		Use:           "remote-boot-agent",
		Short:         "remote-boot-agent reads boot configurations and posts them to Home Assistant",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/etc/remote-boot-agent/config.yaml", "config file")
	rootCmd.PersistentFlags().String(config.FlagMac, "", "MAC Address override")
	rootCmd.PersistentFlags().String(config.FlagName, "", "Name override")
	rootCmd.PersistentFlags().String(config.FlagAddress, "", "Address override")
	rootCmd.PersistentFlags().String(config.FlagBroadcastAddress, "", "Broadcast address override for WOL")
	rootCmd.PersistentFlags().Int(config.FlagBroadcastPort, 9, "Broadcast port override for WOL")
	rootCmd.PersistentFlags().String(config.FlagBootloader, "", "Bootloader type override (e.g., grub)")
	rootCmd.PersistentFlags().String(config.FlagBootloaderPath, "", "Bootloader config path override")
	rootCmd.PersistentFlags().String(config.FlagInitSystem, "", "Initsystem override (e.g., systemd)")
	rootCmd.PersistentFlags().String(config.FlagHassURL, "", "Home Assistant URL override")
	rootCmd.PersistentFlags().String(config.FlagHassWebhook, "", "Home Assistant Webhook ID override")

	deps.BootloaderRegistry.Register("grub", bootloader.NewGrub)
	deps.InitRegistry.Register("systemd", initsystem.NewSystemd)

	rootCmd.AddCommand(NewOptionsCmd(deps))
	rootCmd.AddCommand(NewConfigCmd(deps))
	rootCmd.AddCommand(NewSetupCmd(deps))
	rootCmd.AddCommand(NewApplyCmd(deps))

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
