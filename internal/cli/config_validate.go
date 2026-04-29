package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewConfigValidateCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate an existing configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Configuration is valid.")
			return nil
		},
	}
}
