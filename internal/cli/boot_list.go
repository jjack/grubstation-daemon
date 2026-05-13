package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewBootListCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Output the list of available boot options from GRUB",
		RunE: func(cmd *cobra.Command, args []string) error {
			bootOptions, err := deps.Grub.GetBootOptions(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get boot options from grub: %w", err)
			}

			cmd.Println("Available Boot Options:")
			if len(bootOptions) == 0 {
				cmd.Println("  (None found)")
			} else {
				for _, bootOption := range bootOptions {
					cmd.Printf("  - %s\n", bootOption)
				}
			}

			return nil
		},
	}
}
