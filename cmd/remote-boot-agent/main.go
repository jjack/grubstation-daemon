package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jjack/remote-boot-agent/pkg/bootloader"
	_ "github.com/jjack/remote-boot-agent/pkg/bootloader/grub"
	"github.com/jjack/remote-boot-agent/pkg/config"
	"github.com/jjack/remote-boot-agent/pkg/initsystem"
	_ "github.com/jjack/remote-boot-agent/pkg/initsystem/systemd"
	"github.com/spf13/cobra"
)

// ensureAutoDetect resolves bootloader and initsystem if they are not explicitly provided
func ensureAutoDetect(cfg *config.Config) {
	if cfg.Host.Bootloader == "" {
		log.Println("Bootloader not specified, attempting auto-detection...")
		cfg.Host.Bootloader = bootloader.Detect()
		if cfg.Host.Bootloader == "" {
			log.Println("Warning: Could not auto-detect a registered bootloader.")
		} else {
			log.Printf("Auto-detected bootloader: %s\n", cfg.Host.Bootloader)
		}
	}
	if cfg.Host.InitSystem == "" {
		log.Println("Init system not specified, attempting auto-detection...")
		cfg.Host.InitSystem = initsystem.Detect()
		if cfg.Host.InitSystem == "" {
			log.Println("Warning: Could not auto-detect a registered init system.")
		} else {
			log.Printf("Auto-detected init system: %s\n", cfg.Host.InitSystem)
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
		
		fmt.Printf("Starting remote-boot-agent (bootloader=%v, init=%v)...\n", cfg.Host.Bootloader, cfg.Host.InitSystem)
		fmt.Printf("Device Info: hostname=%v, mac=%v\n", cfg.Host.Hostname, cfg.Host.MACAddress)
		log.Println("Done.")
	},
}

var getSelectedOSCmd = &cobra.Command{
	Use:   "get-selected-os",
	Short: "Output the currently selected OS from Home Assistant",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cmd.Flags())
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
		ensureAutoDetect(cfg)
		
		if cfg.HomeAssistant.BaseURL == "" {
			log.Fatalf("Error: Home Assistant Base URL is not configured. Please provide it via config file or flags.")
		}

		endpoint := fmt.Sprintf("%s/api/remote_boot_manager/%s", strings.TrimRight(cfg.HomeAssistant.BaseURL, "/"), cfg.Host.MACAddress)
		fmt.Printf("Action: Getting info from %s...\n", endpoint)

		resp, err := http.Get(endpoint)
		if err != nil {
			log.Fatalf("Error communicating with Home Assistant: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var buf bytes.Buffer
			buf.ReadFrom(resp.Body)
			fmt.Printf("Response:\n%s\n", buf.String())
		} else {
			log.Fatalf("Received non-success status code from Home Assistant: %d", resp.StatusCode)
		}

	},
}

var getAvailableOSesCmd = &cobra.Command{
	Use:   "get-available-oses",
	Short: "Output the list of available OSes from the bootloader",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cmd.Flags())
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
		ensureAutoDetect(cfg)

		fmt.Printf("Action: Getting available OSes (bootloader=%s)...\n", cfg.Host.Bootloader)
		bl, ok := bootloader.Get(cfg.Host.Bootloader)
		if !ok {
			log.Fatalf("Bootloader plugin %q not found or not registered", cfg.Host.Bootloader)
		}

		opts, err := bl.Parse(cfg)
		if err != nil {
			log.Fatalf("Error parsing bootloader config: %v", err)
		}

		fmt.Printf("Available OSes (via %s):\n", cfg.Host.Bootloader)
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
		
		if cfg.HomeAssistant.BaseURL == "" {
			log.Fatalf("Error: Home Assistant Base URL is not configured. Please provide it via config file or flags.")
		}

		bl, ok := bootloader.Get(cfg.Host.Bootloader)
		if !ok {
			log.Fatalf("Bootloader plugin %q not found or not registered", cfg.Host.Bootloader)
		}

		opts, err := bl.Parse(cfg)
		if err != nil {
			log.Fatalf("Error parsing bootloader data: %v", err)
		}

		// Webhook endpoint (we can configure the webhook ID explicitly later if needed)
		webhookURL := fmt.Sprintf("%s/api/webhook/remote_boot_manager_ingest", strings.TrimRight(cfg.HomeAssistant.BaseURL, "/"))
		fmt.Printf("Action: Pushing available OSes (bootloader=%s) to %s...\n", cfg.Host.Bootloader, webhookURL)

		payload := map[string]interface{}{
			"mac_address": cfg.Host.MACAddress,
			"hostname":    cfg.Host.Hostname,
			"bootloader":  cfg.Host.Bootloader,
			"os_list":     opts.AvailableOSes,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			log.Fatalf("Error marshaling payload: %v", err)
		}

		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Fatalf("Error posting to Home Assistant: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			fmt.Println("Successfully pushed available OSes to Home Assistant.")
		} else {
			log.Fatalf("Received non-success status code from Home Assistant: %d", resp.StatusCode)
		}
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
