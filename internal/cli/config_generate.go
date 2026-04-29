package cli

import (
	"context"
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
	runGenerateSurvey     = GenerateConfigSurvey
	saveConfigFile        = config.Save
)

func ensureSupportedSystems(ctx context.Context, deps *CommandDeps) error {
	_, err := deps.BootloaderRegistry.Detect(ctx)
	if err != nil {
		if err.Error() == "no supported bootloader detected" {
			supported := strings.Join(deps.BootloaderRegistry.SupportedBootloaders(), ", ")
			return fmt.Errorf("no supported bootloader detected. Please ensure you have one of the following installed: %s", supported)
		}
		return err
	}

	_, err = deps.InitRegistry.Detect(ctx)
	if err != nil {
		if err.Error() == "no supported init system detected" {
			supported := strings.Join(deps.InitRegistry.SupportedInitSystems(), ", ")
			return fmt.Errorf("no supported init system detected. Please ensure you have one of the following installed: %s", supported)
		}
		return err
	}
	return nil
}

// NewConfigGenerateCmd walks the user through generating a config interactively
func NewConfigGenerateCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Interactively generate a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureSupportedSystems(cmd.Context(), deps); err != nil {
				return err
			}

			opts := GenerateSurveyOptions{
				DiscoverHomeAssistant: discoverHomeAssistant,
				DetectHostname:        detectSystemHostname,
				GetInterfaces:         getSystemInterfaces,
				BootloaderOptions:     deps.BootloaderRegistry.SupportedBootloaders(),
				InitSystemOptions:     deps.InitRegistry.SupportedInitSystems(),
			}

			cfg, err := runGenerateSurvey(opts)
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
