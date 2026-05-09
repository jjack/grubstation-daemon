package cli

import (
	"context"

	"github.com/jjack/grubstation-cli/internal/daemon"
	"github.com/jjack/grubstation-cli/internal/reporter"
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
			svcMgr, _ := deps.ServiceManager(cmd.Context())
			svcName := ""
			if svcMgr != nil {
				svcName = svcMgr.Name()
			}
			rep := reporter.New(deps.Config, deps.Grub, svcName)
			d := newDaemon(daemon.Config{
				ListenPort:        deps.Config.Daemon.ListenPort,
				ReportBootOptions: deps.Config.Daemon.ReportBootOptions,
				APIKey:            deps.Config.Daemon.APIKey,
			}, rep.PushBootOptions)
			return d.Run(cmd.Context())
		},
	}
}
