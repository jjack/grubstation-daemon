package cli

import (
	"context"
	"runtime"

	"github.com/jjack/grubstation/internal/daemon"
	"github.com/jjack/grubstation/internal/reporter"
	"github.com/jjack/grubstation/internal/version"
	"github.com/spf13/cobra"
)

type daemonRunner interface {
	Run(ctx context.Context) error
}

var newDaemon = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) daemonRunner {
	return daemon.New(cfg, meta, regHandler, updateHandler)
}

func NewDaemonCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the persistent agent daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			var regHandler func(ctx context.Context, token string) error
			var updateHandler func(ctx context.Context) error

			mgr, _ := deps.Manager(cmd.Context())
			mgrName := ""
			if activeMgr := mgr; activeMgr != nil {
				mgrName = activeMgr.Name()
			}

			if deps.Config.Daemon.ReportBootOptions {
				rep := reporter.New(deps.Config, deps.Grub, mgrName)
				regHandler = rep.RegisterDaemon
				updateHandler = rep.PushBootOptions
			}
			d := newDaemon(daemon.Config{
				Port:              deps.Config.Daemon.Port,
				ReportBootOptions: deps.Config.Daemon.ReportBootOptions,
				APIKey:            deps.Config.Daemon.APIKey,
			}, daemon.Metadata{
				OS:             runtime.GOOS,
				Version:        version.Version,
				ServiceManager: mgrName,
			}, regHandler, updateHandler)
			return d.Run(cmd.Context())
		},
	}
}
