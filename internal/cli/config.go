package cli

import (
	"github.com/spf13/cobra"
)

func NewConfigCmd(deps *CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the remote-boot-agent configuration",
	}

	cmd.AddCommand(NewConfigValidateCmd(deps))

	return cmd
}
