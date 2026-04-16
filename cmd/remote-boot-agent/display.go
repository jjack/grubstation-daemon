package main

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/spf13/cobra"
)

func newDisplayAvailableOSesCmd(blReg *bootloader.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "display",
		Short: "Output the list of available OSes from the bootloader",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, ok := blReg.Get(loadedConfig.Host.Bootloader)
			if !ok {
				return fmt.Errorf("bootloader plugin %q not found or not registered", loadedConfig.Host.Bootloader)
			}

			opts, err := bl.Parse(loadedConfig.Host.BootloaderConfigPath)
			if err != nil {
				return fmt.Errorf("error parsing bootloader config: %w", err)
			}

			for _, osName := range opts.AvailableOSes {
				fmt.Printf("Available OS: %s\n", osName)
			}
			return nil
		},
	}
}
