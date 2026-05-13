package cli

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func NewServiceCmd(deps *CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the grubstation service",
	}

	cmd.AddCommand(NewServiceUninstallCmd(deps))
	cmd.AddCommand(NewServiceStartCmd(deps))
	cmd.AddCommand(NewServiceStopCmd(deps))
	cmd.AddCommand(NewServiceStatusCmd(deps))

	return cmd
}

func NewServiceUninstallCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the grubstation service and GRUB hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := deps.Manager(cmd.Context())
			if err != nil {
				return err
			}

			cmd.Printf("Uninstalling service: %s\n", mgr.Name())
			if err := mgr.Uninstall(cmd.Context()); err != nil {
				return fmt.Errorf("failed to uninstall manager: %w", err)
			}

			if deps.Config.Daemon.ReportBootOptions {
				cmd.Printf("Removing GRUB hooks...\n")
				if err := deps.Grub.Uninstall(cmd.Context()); err != nil {
					return fmt.Errorf("failed to uninstall grub: %w", err)
				}
			}

			cmd.Println("Uninstallation completed successfully.")
			return nil
		},
	}
}

func NewServiceStartCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the grubstation service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := deps.Manager(cmd.Context())
			if err != nil {
				return err
			}
			return mgr.Start(cmd.Context())
		},
	}
}

func NewServiceStopCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the grubstation service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := deps.Manager(cmd.Context())
			if err != nil {
				return err
			}
			return mgr.Stop(cmd.Context())
		},
	}
}

func NewServiceStatusCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the status of the grubstation service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := deps.Manager(cmd.Context())
			if err != nil {
				return err
			}

			if mgr.IsActive(cmd.Context()) {
				cmd.Printf("Service %s is active\n", mgr.Name())
			} else {
				cmd.Printf("Service %s is inactive\n", mgr.Name())
			}

			// Also check health endpoint
			client := &http.Client{Timeout: 2 * time.Second}
			url := fmt.Sprintf("http://localhost:%d/healthcheck", deps.Config.Daemon.Port)
			resp, err := client.Get(url)
			if err != nil {
				cmd.Printf("Daemon health check failed: %v (daemon might not be running or port is blocked)\n", err)
				return nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				cmd.Printf("Daemon health: %s", string(body))
			} else {
				cmd.Printf("Daemon health check returned non-OK status: %d\n", resp.StatusCode)
			}

			return nil
		},
	}
}
