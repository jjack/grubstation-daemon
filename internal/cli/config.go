package cli

import (
	"github.com/spf13/cobra"
)

func NewConfigCmd(deps *CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the grubstation configuration",
	}

	cmd.AddCommand(NewConfigValidateCmd(deps))
	cmd.AddCommand(NewConfigInitCmd(deps))

	return cmd
}
