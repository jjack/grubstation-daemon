package cli

import (
	"fmt"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/spf13/cobra"
)

func NewListCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Output the list of available boot options from the bootloader",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := deps.Bootloader(cmd.Context())
			if err != nil {
				return err
			}

			fmt.Printf("Bootloader: %s\n", bl.Name())

			bootOptions, err := bl.GetBootOptions(cmd.Context(), bootloader.Config{
				ConfigPath: deps.Config.Bootloader.ConfigPath,
			})
			if err != nil {
				return fmt.Errorf("failed to get boot options from bootloader %s: %w", bl.Name(), err)
			}

			fmt.Println("Available Boot Options:")
			if len(bootOptions) == 0 {
				fmt.Println("  (None found)")
			} else {
				for _, bootOption := range bootOptions {
					fmt.Printf("  - %s\n", bootOption)
				}
			}

			return nil
		},
	}
}
