package main

import (
	"fmt"
	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/spf13/cobra"
)

func newPushAvailableOSesCmd(blReg *bootloader.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "push-available-oses",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, ok := blReg.Get(loadedConfig.Host.Bootloader)
			if !ok {
				return fmt.Errorf("bootloader plugin %q not found or not registered", loadedConfig.Host.Bootloader)
			}

			opts, err := bl.Parse(loadedConfig.Host.BootloaderConfigPath)
			if err != nil {
				return fmt.Errorf("error parsing bootloader config: %w", err)
			}

			haClient := homeassistant.NewClient(loadedConfig.HomeAssistant)
			payload := homeassistant.HAPayload{
				MACAddress: loadedConfig.Host.MACAddress,
				Hostname:   loadedConfig.Host.Hostname,
				Bootloader: loadedConfig.Host.Bootloader,
				OSList:     opts.AvailableOSes,
			}

			return haClient.PushAvailableOSes(payload)
		},
	}
}
