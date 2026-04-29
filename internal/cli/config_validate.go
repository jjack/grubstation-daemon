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
			if err := deps.Config.Validate(); err != nil {
				return fmt.Errorf("configuration is invalid: %w", err)
			}
			cmd.Println("Configuration is valid.")
			return nil
		},
	}
}
