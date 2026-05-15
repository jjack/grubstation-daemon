//go:build linux

package cli

import (
	"github.com/spf13/cobra"
)

func NewBootCmd(deps *CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "boot",
		Short: "Manage boot options",
	}

	cmd.AddCommand(NewBootListCmd(deps))
	cmd.AddCommand(NewBootPushCmd(deps))

	return cmd
}
