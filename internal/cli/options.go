package cli

import (
	"github.com/spf13/cobra"
)

func NewOptionsCmd(deps *CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Manage boot options",
	}

	cmd.AddCommand(NewListCmd(deps))
	cmd.AddCommand(NewPushCmd(deps))

	return cmd
}
