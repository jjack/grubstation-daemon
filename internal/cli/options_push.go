package cli

import (
	"log/slog"

	"github.com/jjack/grubstation-cli/internal/daemon"
	"github.com/jjack/grubstation-cli/internal/reporter"
	"github.com/spf13/cobra"
)

func NewPushCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemon.RequestPushViaSocket(cmd.Context()); err == nil {
				cmd.Println("Successfully pushed boot options to Home Assistant (via running daemon)")
				return nil
			} else {
				slog.Debug("Could not push via daemon socket, falling back to direct push", "error", err)
			}

			svcMgr, _ := deps.ServiceManager(cmd.Context())
			svcName := ""
			if svcMgr != nil {
				svcName = svcMgr.Name()
			}
			rep := reporter.New(deps.Config, deps.Grub, svcName)
			if err := rep.PushBootOptions(cmd.Context(), ""); err != nil {
				return err
			}
			cmd.Println("Successfully pushed boot options to Home Assistant")
			return nil
		},
	}
}
