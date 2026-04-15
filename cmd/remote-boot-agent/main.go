package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/bootloader/grub"
	"github.com/jjack/remote-boot-agent/internal/config"
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

			endpoint := fmt.Sprintf("%s/api/remote_boot_manager/%s", strings.TrimRight(cfg.HomeAssistant.BaseURL, "/"), cfg.Host.MACAddress)
			resp, err := http.Get(endpoint)
			if err != nil {
				return fmt.Errorf("error communicating with Home Assistant: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var buf bytes.Buffer
				buf.ReadFrom(resp.Body)
				fmt.Printf("%s\n", buf.String())
				return nil
			}
			return fmt.Errorf("received HTTP %d from Home Assistant", resp.StatusCode)
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

			webhookURL := fmt.Sprintf("%s/api/webhook/remote_boot_manager_ingest", strings.TrimRight(cfg.HomeAssistant.BaseURL, "/"))

			type HAPayload struct {
				MACAddress string   `json:"mac_address"`
				Hostname   string   `json:"hostname"`
				Bootloader string   `json:"bootloader"`
				OSList     []string `json:"os_list"`
			}
			payload := HAPayload{
				MACAddress: cfg.Host.MACAddress,
				Hostname:   cfg.Host.Hostname,
				Bootloader: cfg.Host.Bootloader,
				OSList:     opts.AvailableOSes,
			}

			jsonData, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("error marshaling payload: %w", err)
			}

			resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				return fmt.Errorf("error posting to Home Assistant: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			return fmt.Errorf("received HTTP %d from Home Assistant", resp.StatusCode)
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
