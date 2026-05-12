package cli

import (
	"context"

	"github.com/jjack/grubstation-daemon/internal/daemon"
	"github.com/jjack/grubstation-daemon/internal/reporter"
	"github.com/spf13/cobra"
)

type daemonRunner interface {
	Run(ctx context.Context) error
}

var newDaemon = func(cfg daemon.Config, pushHandler func(ctx context.Context, token string) error) daemonRunner {
	return daemon.New(cfg, pushHandler)
}

func NewDaemonCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the persistent agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			var pushHandler func(ctx context.Context, token string) error
			if deps.Config.Daemon.ReportBootOptions {
				mgr, _ := deps.Manager(cmd.Context())
				mgrName := ""
				if mgr != nil {
					mgrName = mgr.Name()
				}
				rep := reporter.New(deps.Config, deps.Grub, mgrName)
				pushHandler = rep.PushBootOptions
			}
			d := newDaemon(daemon.Config{
				Port:              deps.Config.Daemon.Port,
				ReportBootOptions: deps.Config.Daemon.ReportBootOptions,
				APIKey:            deps.Config.Daemon.APIKey,
			}, pushHandler)
			return d.Run(cmd.Context())
		},
	}
}
