package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/homeassistant"
	"github.com/jjack/grubstation/internal/host"
	"github.com/jjack/grubstation/internal/servicemanager"
	"github.com/spf13/cobra"
)

type CLI struct {
	Config  *config.Config
	RootCmd *cobra.Command
}

type SystemResolver interface {
	DiscoverHomeAssistant(ctx context.Context) ([]homeassistant.ServiceInstance, error)
	DetectSystemHostname() (string, error)
	GetWOLInterfaces() ([]net.Interface, error)
	GetIPInfo(inf net.Interface) ([]string, map[string]string)
	GetFQDN(hostname string) string
	SaveConfig(cfg *config.Config, path string) error
	DiscoverGrubConfig(ctx context.Context) (string, error)
}

type DefaultSystemResolver struct{}

func (d *DefaultSystemResolver) DiscoverHomeAssistant(ctx context.Context) ([]homeassistant.ServiceInstance, error) {
	return homeassistant.Discover(ctx)
}

func (d *DefaultSystemResolver) DetectSystemHostname() (string, error) {
	return host.DetectHostname()
}

func (d *DefaultSystemResolver) GetWOLInterfaces() ([]net.Interface, error) {
	return host.GetWOLInterfaces()
}

func (d *DefaultSystemResolver) GetIPInfo(inf net.Interface) ([]string, map[string]string) {
	return host.GetIPInfo(inf)
}
func (d *DefaultSystemResolver) GetFQDN(hostname string) string { return host.GetFQDN(hostname) }
func (d *DefaultSystemResolver) SaveConfig(cfg *config.Config, path string) error {
	return config.Save(cfg, path)
}

func (d *DefaultSystemResolver) DiscoverGrubConfig(ctx context.Context) (string, error) {
	g := &grub.Grub{}
	return g.DiscoverConfigPath(ctx)
}

type CommandDeps struct {
	Config         *config.Config
	ConfigFile     string
	Grub           *grub.Grub
	Registry       *servicemanager.Registry
	SystemResolver SystemResolver
}

func (cd *CommandDeps) Manager(ctx context.Context) (servicemanager.Manager, error) {
	mgr, err := cd.Registry.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("manager detection failed: %w", err)
	}
	return mgr, nil
}

func NewCLI() *CLI {
	cli := &CLI{}

	deps := &CommandDeps{
		Config:         &config.Config{},
		Grub:           &grub.Grub{},
		Registry:       servicemanager.NewRegistry(),
		SystemResolver: &DefaultSystemResolver{},
	}

	var cfgFile string
	var debugMode bool

	rootCmd := &cobra.Command{
		Use:           "grubstation",
		Short:         "grubstation reads boot configurations and posts them to Home Assistant",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "help" {
				return nil
			}

			if debugMode || os.Getenv("DEBUG") == "true" {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}

			cfg, err := config.Load(cfgFile, cmd.Flags())
			if err != nil {
				return err
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			*deps.Config = *cfg
			deps.ConfigFile = cfgFile
			cli.Config = deps.Config

			if cfg.Grub != nil && cfg.Grub.ConfigPath != "" {
				deps.Grub.ConfigPath = cfg.Grub.ConfigPath
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", config.DefaultConfigPath(), "config file")
	rootCmd.PersistentFlags().String(config.FlagGrubConfig, "", "GRUB config path override")
	rootCmd.PersistentFlags().String(config.FlagMac, "", "MAC Address override")
	rootCmd.PersistentFlags().String(config.FlagAddress, "", "Address override")
	rootCmd.PersistentFlags().String(config.FlagWolBroadcastAddress, "", "WOL target address override (defaults to 255.255.255.255)")
	rootCmd.PersistentFlags().Int(config.FlagWolBroadcastPort, 9, "WOL target port override (defaults to 9)")
	rootCmd.PersistentFlags().String(config.FlagDaemonKey, "", "API key for the daemon")
	rootCmd.PersistentFlags().String(config.FlagHassURL, "", "Home Assistant URL override")
	rootCmd.PersistentFlags().String(config.FlagHassWebhook, "", "Home Assistant Webhook ID override")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")

	// Register platform-specific services automatically
	servicemanager.RegisterDefaultServices(deps.Registry)

	rootCmd.AddCommand(NewBootCmd(deps))
	rootCmd.AddCommand(NewConfigCmd(deps))
	rootCmd.AddCommand(NewSetupCmd(deps))
	rootCmd.AddCommand(NewServiceCmd(deps))
	rootCmd.AddCommand(NewServeCmd(deps))

	// get rid of the completion command because it doesn't make sense here
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	cli.RootCmd = rootCmd
	return cli
}

func (cli *CLI) Execute() error {
	return cli.RootCmd.Execute()
}
