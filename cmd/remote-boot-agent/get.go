package main

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/spf13/cobra"
)

func newGetSelectedOSCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Output the currently selected OS from Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			haClient := homeassistant.NewClient(loadedConfig.HomeAssistant)
			osName, err := haClient.GetSelectedOS(loadedConfig.Host.MACAddress)
			if err != nil {
				return err
			}
			fmt.Printf("Selected OS from Home Assistant: %s\n", osName)
			return nil
		},
	}
}
