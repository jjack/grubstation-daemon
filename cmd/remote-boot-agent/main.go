package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jjack/remote-boot-agent/pkg/bootloader"
	_ "github.com/jjack/remote-boot-agent/pkg/bootloader/grub"
	"github.com/jjack/remote-boot-agent/pkg/config"
	"github.com/jjack/remote-boot-agent/pkg/initsystem"
	_ "github.com/jjack/remote-boot-agent/pkg/initsystem/systemd"
	"github.com/spf13/cobra"
)

// ensureAutoDetect resolves bootloader and initsystem if they are not explicitly provided
func ensureAutoDetect(cfg *config.Config) {
	if cfg.BootloaderName == "" {
		log.Println("Bootloader not specified, attempting auto-detection...")
		cfg.BootloaderName = bootloader.Detect()
		if cfg.BootloaderName == "" {
			log.Println("Warning: Could not auto-detect a registered bootloader.")
		} else {
			log.Printf("Auto-detected bootloader: %s\n", cfg.BootloaderName)
		}
	}
	if cfg.InitSystemName == "" {
		log.Println("Init system not specified, attempting auto-detection...")
		cfg.InitSystemName = initsystem.Detect()
		if cfg.InitSystemName == "" {
			log.Println("Warning: Could not auto-detect a registered init system.")
		} else {
			log.Printf("Auto-detected init system: %s\n", cfg.InitSystemName)
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "remote-boot-agent",
	Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cmd.Flags())
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
		ensureAutoDetect(cfg)
		
		fmt.Printf("Starting remote-boot-agent (bootloader=%v, init=%v)...\n", cfg.BootloaderName, cfg.InitSystemName)
		fmt.Printf("Device Info: hostname=%v, mac=%v\n", cfg.Hostname, cfg.MACAddress)
		log.Println("Done.")
	},
}

var getSelectedOSCmd = &cobra.Command{
	Use:   "get-selected-os",
	Short: "Output the currently selected OS",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cmd.Flags())
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
		ensureAutoDetect(cfg)
		
		fmt.Printf("Action: Getting selected OS (bootloader=%s)...\n", cfg.BootloaderName)
	},
}

var getAvailableOSesCmd = &cobra.Command{
	Use:   "get-available-oses",
	Short: "Output the list of available OSes",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cmd.Flags())
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
		ensureAutoDetect(cfg)

		fmt.Printf("Action: Getting available OSes (bootloader=%s)...\n", cfg.BootloaderName)
		bl, ok := bootloader.Get(cfg.BootloaderName)
		if !ok {
			log.Fatalf("Bootloader plugin %q not found or not registered", cfg.BootloaderName)
		}

		opts, err := bl.Parse(cfg)
		if err != nil {
			log.Fatalf("Error parsing bootloader config: %v", err)
		}

		fmt.Printf("Available OSes (via %s):\n", cfg.BootloaderName)
		for _, osName := range opts.AvailableOSes {
			fmt.Printf("  - %s\n", osName)
		}

	},
}


var pushAvailableOSesCmd = &cobra.Command{
	Use:   "push-available-oses",
	Short: "Push the list of available OSes to Home Assistant",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cmd.Flags())
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
		ensureAutoDetect(cfg)
		
		fmt.Printf("Action: Pushing available OSes (bootloader=%s)...\n", cfg.BootloaderName)
	},
}

func init() {
	config.InitFlags(rootCmd.PersistentFlags())
	
	rootCmd.AddCommand(getSelectedOSCmd)
	rootCmd.AddCommand(getAvailableOSesCmd)
	rootCmd.AddCommand(pushAvailableOSesCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
