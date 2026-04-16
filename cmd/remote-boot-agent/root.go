package main

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/spf13/cobra"
)

var loadedConfig *config.Config

func setDefaults(cfg *config.Config, blReg *bootloader.Registry) {
	if cfg.Host.Bootloader == "" {
		cfg.Host.Bootloader = blReg.Detect()
	}
}

func newRootCmd(blReg *bootloader.Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote-boot-agent",
		Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cmd.Flags())
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}
			setDefaults(cfg, blReg)
			loadedConfig = cfg
			return nil
		},
	}

	cmd.AddCommand(newGetSelectedOSCmd())
	cmd.AddCommand(newDisplayAvailableOSesCmd(blReg))
	cmd.AddCommand(newPushAvailableOSesCmd(blReg))

	return cmd
}
