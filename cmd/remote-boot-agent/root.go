package main

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/system"
	"github.com/spf13/cobra"
)

type CLI struct {
	Config  *config.Config
	CfgFile string
	RootCmd *cobra.Command
}

func NewCLI() *CLI {
	cli := &CLI{}

	rootCmd := &cobra.Command{
		Use:   "remote-boot-agent",
		Short: "remote-boot-agent reads boot configurations and posts them to Home Assistant",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(cli.CfgFile)
			if err != nil {
				return err
			}

			if mac, _ := cmd.Flags().GetString("mac"); mac != "" {
				cfg.Host.MACAddress = mac
			}
			if hostname, _ := cmd.Flags().GetString("hostname"); hostname != "" {
				cfg.Host.Hostname = hostname
			}
			if bl, _ := cmd.Flags().GetString("bootloader"); bl != "" {
				cfg.Bootloader.Name = bl
			}

			if cfg.Host.MACAddress == "" {
				mac, err := system.DetectMACAddress()
				if err != nil {
					return err
				}
				cfg.Host.MACAddress = mac
			}

			if cfg.Host.Hostname == "" {
				host, err := system.DetectHostname()
				if err != nil {
					return err
				}
				cfg.Host.Hostname = host
			}

			cli.Config = cfg
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cli.CfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().String("mac", "", "MAC Address override")
	rootCmd.PersistentFlags().String("hostname", "", "Hostname override")
	rootCmd.PersistentFlags().String("bootloader", "", "Bootloader override (e.g., grub)")

	rootCmd.AddCommand(GetOSList(cli))
	rootCmd.AddCommand(PushOSes(cli))
	rootCmd.AddCommand(GetSelectedOS(cli))

	cli.RootCmd = rootCmd
	return cli
}

func (cli *CLI) Execute() error {
	return cli.RootCmd.Execute()
}

func ResolveBootloader(cfg *config.Config) (bootloader.Bootloader, error) {
	if cfg.Bootloader.Name != "" {
		bl := bootloader.Get(cfg.Bootloader.Name)
		if bl == nil {
			return nil, fmt.Errorf("specified bootloader %s not supported", cfg.Bootloader.Name)
		}
		return bl, nil
	}

	bl, err := bootloader.Detect()
	if err != nil {
		return nil, fmt.Errorf("bootloader detection failed: %w", err)
	}
	return bl, nil
}
