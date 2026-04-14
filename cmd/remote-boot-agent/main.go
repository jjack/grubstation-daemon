package main

import (
	"fmt"
	"os"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/bootloader/grub"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/jjack/remote-boot-agent/internal/initsystem/systemd"
	"github.com/spf13/cobra"
)

func setDefaults(cfg *config.Config, blReg *bootloader.Registry, initReg *initsystem.Registry) {
	if cfg.Host.Bootloader == "" {
		cfg.Host.Bootloader = blReg.Detect()
	}
	if cfg.Host.InitSystem == "" {
		cfg.Host.InitSystem = initReg.Detect()
	}
}

func buildCommands(blReg *bootloader.Registry, initReg *initsystem.Registry) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "remote-boot-agent",
		Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
	}
	config.InitFlags(rootCmd.PersistentFlags())

	var getSelectedOSCmd = &cobra.Command{
		Use:   "get-selected-os",
		Short: "Output the currently selected OS from Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cmd.Flags())
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}
			setDefaults(cfg, blReg, initReg)

			haClient := homeassistant.NewClient(cfg.HomeAssistant)
			osName, err := haClient.GetSelectedOS(cfg.Host.MACAddress)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", osName)
			return nil
		},
	}

	var getAvailableOSesCmd = &cobra.Command{
		Use:   "get-available-oses",
		Short: "Output the list of available OSes from the bootloader",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cmd.Flags())
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}
			setDefaults(cfg, blReg, initReg)

			bl, ok := blReg.Get(cfg.Host.Bootloader)
			if !ok {
				return fmt.Errorf("bootloader plugin %q not found or not registered", cfg.Host.Bootloader)
			}

			opts, err := bl.Parse(cfg)
			if err != nil {
				return fmt.Errorf("error parsing bootloader config: %w", err)
			}

			for _, osName := range opts.AvailableOSes {
				fmt.Printf("%s\n", osName)
			}
			return nil
		},
	}

	var pushAvailableOSesCmd = &cobra.Command{
		Use:   "push-available-oses",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cmd.Flags())
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}
			setDefaults(cfg, blReg, initReg)

			bl, ok := blReg.Get(cfg.Host.Bootloader)
			if !ok {
				return fmt.Errorf("bootloader plugin %q not found or not registered", cfg.Host.Bootloader)
			}

			opts, err := bl.Parse(cfg)
			if err != nil {
				return fmt.Errorf("error parsing bootloader config: %w", err)
			}

			haClient := homeassistant.NewClient(cfg.HomeAssistant)
			payload := homeassistant.HAPayload{
				MACAddress: cfg.Host.MACAddress,
				Hostname:   cfg.Host.Hostname,
				Bootloader: cfg.Host.Bootloader,
				OSList:     opts.AvailableOSes,
			}

			if err := haClient.PushAvailableOSes(payload); err != nil {
				return err
			}

			return nil
		},
	}

	rootCmd.AddCommand(getSelectedOSCmd)
	rootCmd.AddCommand(getAvailableOSesCmd)
	rootCmd.AddCommand(pushAvailableOSesCmd)

	return rootCmd
}

func main() {
	blReg := bootloader.NewRegistry(grub.New())
	initReg := initsystem.NewRegistry(systemd.New())

	rootCmd := buildCommands(blReg, initReg)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
