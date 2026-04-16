package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func GetOSList(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "display",
		Short: "Output the list of available OSes from the bootloader",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := ResolveBootloader(cli.Config)
			if err != nil {
				return err
			}

			fmt.Printf("Bootloader: %s\n", bl.Name())

			osList, err := bl.GetOSList(cli.Config.Bootloader.ConfigPath)
			if err != nil {
				return fmt.Errorf("failed to get OS list from bootloader %s: %w", bl.Name(), err)
			}

			fmt.Println("Available Operating Systems:")
			if len(osList) == 0 {
				fmt.Println("  (None found)")
			} else {
				for _, osName := range osList {
					fmt.Printf("  - %s\n", osName)
				}
			}

			return nil

		},
	}
}
