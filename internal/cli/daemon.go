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

var newDaemon = func(cfg daemon.Config, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) daemonRunner {
	return daemon.New(cfg, regHandler, updateHandler)
}

func NewDaemonCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the persistent agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			var regHandler func(ctx context.Context, token string) error
			var updateHandler func(ctx context.Context) error

			if deps.Config.Daemon.ReportBootOptions {
				mgr, _ := deps.Manager(cmd.Context())
				mgrName := ""
				if activeMgr := mgr; activeMgr != nil {
					mgrName = activeMgr.Name()
				}
				rep := reporter.New(deps.Config, deps.Grub, mgrName)
				regHandler = rep.RegisterDaemon
				updateHandler = rep.PushBootOptions
			}
			d := newDaemon(daemon.Config{
				Port:              deps.Config.Daemon.Port,
				ReportBootOptions: deps.Config.Daemon.ReportBootOptions,
				APIKey:            deps.Config.Daemon.APIKey,
			}, regHandler, updateHandler)
			return d.Run(cmd.Context())
		},
	}
}
