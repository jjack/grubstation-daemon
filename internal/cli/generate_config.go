package cli

import (
	"fmt"
	"strings"

	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/jjack/remote-boot-agent/internal/system"
	"github.com/spf13/cobra"
)

var (
	discoverHomeAssistant = homeassistant.Discover
	detectSystemHostname  = system.DetectHostname
	getSystemInterfaces   = system.GetInterfaceOptions
	runGenerateForm       = GenerateConfigForm
	saveConfigFile        = config.Save
)

// NewGenerateConfigCmd walks the user through generating a config interactively
func NewGenerateConfigCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "generate-config",
		Short: "Interactively generate a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := deps.BootloaderRegistry.Detect(cmd.Context())
			if err != nil {
				if err.Error() == "no supported bootloader detected" {
					supported := strings.Join(deps.BootloaderRegistry.SupportedBootloaders(), ", ")
					return fmt.Errorf("no supported bootloader detected. Please ensure you have one of the following installed: %s", supported)
				}
				return err
			}

			sys, err := deps.InitRegistry.Detect(cmd.Context())
			if err != nil {
				if err.Error() == "no supported init system detected" {
					supported := strings.Join(deps.InitRegistry.SupportedInitSystems(), ", ")
					return fmt.Errorf("no supported init system detected. Please ensure you have one of the following installed: %s", supported)
				}
				return err
			}

			defaultBootloaderPath := ""
			if path, err := bl.DiscoverConfigPath(cmd.Context()); err == nil {
				defaultBootloaderPath = path
			}

			opts := GenerateFormOptions{
				DiscoverHomeAssistant: discoverHomeAssistant,
				DetectHostname:        detectSystemHostname,
				GetInterfaces:         getSystemInterfaces,
				BootloaderOptions:     deps.BootloaderRegistry.SupportedBootloaders(),
				DefaultBootloader:     bl.Name(),
				DefaultBootloaderPath: defaultBootloaderPath,
				InitSystemOptions:     deps.InitRegistry.SupportedInitSystems(),
				DefaultInitSystem:     sys.Name(),
			}

			cfg, err := runGenerateForm(opts)
			if err != nil {
				return err
			}

			fmt.Println("\nGenerated config (keys may be in a different order than shown here):")
			fmt.Printf("---\n")
			fmt.Printf("host:\n  hostname: %s\n  mac_address: %s\n  broadcast_address: %s\n  broadcast_port: %d\n", cfg.Host.Hostname, cfg.Host.MACAddress, cfg.Host.BroadcastAddress, cfg.Host.BroadcastPort)
			fmt.Printf("homeassistant:\n  url: %s\n  webhook_id: %s\n", cfg.HomeAssistant.URL, cfg.HomeAssistant.WebhookID)
			fmt.Printf("bootloader:\n  name: %s\n  config_path: %s\n", cfg.Bootloader.Name, cfg.Bootloader.ConfigPath)
			fmt.Printf("initsystem:\n  name: %s\n", cfg.InitSystem.Name)

			return saveConfigFile(cfg, "./config.yaml")
		},
	}
}
