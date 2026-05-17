package survey

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/homeassistant"
	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

type SystemResolver interface {
	DiscoverHomeAssistant(ctx context.Context) ([]homeassistant.ServiceInstance, error)
	DetectSystemHostname() (string, error)
	GetWOLInterfaces() ([]net.Interface, error)
	GetIPInfo(inf net.Interface) ([]string, map[string]string)
	GetFQDN(hostname string) string
	DiscoverGrubConfig(ctx context.Context) (string, error)
}

type SurveyDeps interface {
	GetSystemResolver() SystemResolver
	IsInstalled(ctx context.Context) (bool, error)
}

var (
	RunGenerateSurvey func(context.Context, SurveyDeps, bool, int) (*config.Config, bool, error) = generateConfigInteractive

	ErrAborted = errors.New("setup aborted")
)

const (
	ModeDaemonBoth     = "Daemon (Remote shutdown + Report boot options)"
	ModeDaemonShutdown = "Daemon (Remote shutdown only)"
	ModeHookOnly       = "Shutdown hook (Report boot options only)"
	ModeDryRun         = "Dry Run (Preview configuration only)"
)

func generateConfigInteractive(ctx context.Context, deps SurveyDeps, isReinstall bool, currentPort int) (*config.Config, bool, error) {
	resolver := deps.GetSystemResolver()

	// 0. Overwrite Confirmation
	installed, err := deps.IsInstalled(ctx)
	if err == nil && installed {
		overwrite := tap.Confirm(ctx, tap.ConfirmOptions{
			Message:      "GrubStation is already installed as a service. Do you want to re-run setup and overwrite the existing configuration?",
			InitialValue: false,
		})
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		if !overwrite {
			return nil, false, ErrAborted
		}
	}

	// 1. Initial Discovery
	hostname, err := resolver.DetectSystemHostname()
	if err != nil {
		return nil, false, err
	}
	interfaces, err := resolver.GetWOLInterfaces()
	if err != nil {
		return nil, false, err
	}

	grubConfigPath, _ := resolver.DiscoverGrubConfig(ctx)

	// Background HA Discovery
	type haResult struct {
		instances []homeassistant.ServiceInstance
		err       error
	}
	haChan := make(chan haResult, 1)
	go func() {
		instances, err := resolver.DiscoverHomeAssistant(ctx)
		haChan <- haResult{instances, err}
	}()

	// 2. Installation Mode
	var modeOpts []tap.SelectOption[string]
	if grubConfigPath != "" {
		modeOpts = []tap.SelectOption[string]{
			{Value: ModeDaemonBoth, Label: ModeDaemonBoth},
			{Value: ModeDaemonShutdown, Label: ModeDaemonShutdown},
			{Value: ModeHookOnly, Label: ModeHookOnly},
			{Value: ModeDryRun, Label: ModeDryRun},
		}
	} else {
		modeOpts = []tap.SelectOption[string]{
			{Value: ModeDaemonShutdown, Label: ModeDaemonShutdown},
			{Value: ModeDryRun, Label: ModeDryRun},
		}
	}

	mode := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Installation Mode",
		Options: modeOpts,
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	isDryRun := mode == ModeDryRun
	reportsBoot := mode == ModeDaemonBoth || mode == ModeHookOnly || mode == ModeDryRun
	runsDaemon := mode == ModeDaemonBoth || mode == ModeDaemonShutdown || mode == ModeDryRun

	// 3. Network Interface
	ifaceIdx := tap.Select(ctx, tap.SelectOptions[int]{
		Message: "Available Network Interface",
		Options: buildIfaceOptions(resolver, interfaces),
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	selectedIface := interfaces[ifaceIdx]

	// 4. Host Address
	ips, broadcasts := resolver.GetIPInfo(selectedIface)
	fqdn := resolver.GetFQDN(hostname)
	hostAddress := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Host Address (Used for ping checks and communication with the daemon)",
		Options: buildHostSelectOptions(hostname, fqdn, ips),
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	// 5. Daemon Port
	var AgentPort int
	if runsDaemon {
		defaultValue := strconv.Itoa(config.DefaultAgentPort)
		if currentPort > 0 {
			defaultValue = strconv.Itoa(currentPort)
		}
		portStr := tap.Text(ctx, tap.TextOptions{
			Message:      fmt.Sprintf("Daemon Port (default: %d)", config.DefaultAgentPort),
			DefaultValue: defaultValue,
			InitialValue: defaultValue,
			Validate: func(s string) error {
				return validatePort(s, isReinstall, currentPort)
			},
		})
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		AgentPort, _ = strconv.Atoi(portStr)
	}

	// 6. WOL Address
	WolBroadcastAddress := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "WOL Broadcast Address (you may need to choose subnet broadcast for cross-VLAN setups)",
		Options: buildWolSelectOptions(hostAddress, ips, broadcasts),
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	var grubWaitTime int
	if reportsBoot {
		defaultWait := strconv.Itoa(config.DefaultGrubWaitSeconds)
		waitStr := tap.Text(ctx, tap.TextOptions{
			Message:      "GRUB Network Wait (seconds to wait for network before getting next boot option from Home Assistant)",
			DefaultValue: defaultWait,
			InitialValue: defaultWait,
			Validate: func(s string) error {
				return config.ValidateGrubWaitTime(s)
			},
		})
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		grubWaitTime, _ = strconv.Atoi(waitStr)
	} else {
		grubConfigPath = ""
	}

	// 8. Home Assistant URL
	var discovered []homeassistant.ServiceInstance
	select {
	case res := <-haChan:
		discovered = res.instances
	default:
		s := tap.NewSpinner(tap.SpinnerOptions{})
		s.Start("Discovering Home Assistant...")
		res := <-haChan
		s.Stop("Home Assistant discovery complete", 0)
		discovered = res.instances
	}

	var haURL string
	var grubURL string

	totalURLs := 0
	for _, inst := range discovered {
		totalURLs += len(inst.URLs)
	}

	if totalURLs == 1 {
		// Single URL fallback
		haURL = discovered[0].URLs[0]
	} else if len(discovered) > 0 {
		// 1. Instance Selection
		var instOpts []tap.SelectOption[int]
		for i, inst := range discovered {
			label := fmt.Sprintf("%s (%s)", inst.Name, strings.Join(inst.URLs, ", "))
			instOpts = append(instOpts, tap.SelectOption[int]{Value: i, Label: label})
		}
		instOpts = append(instOpts, tap.SelectOption[int]{Value: -1, Label: "Other (Enter manually)"})

		instIdx := tap.Select(ctx, tap.SelectOptions[int]{
			Message: "Home Assistant instances discovered. Please select one:",
			Options: instOpts,
		})
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}

		if instIdx != -1 {
			selectedInst := discovered[instIdx]

			// 2. Agent URL Selection
			var agentOpts []tap.SelectOption[string]
			for _, u := range selectedInst.URLs {
				label := u
				if strings.HasPrefix(u, "https://") {
					label += " (HTTPS is Preferred)"
				}
				agentOpts = append(agentOpts, tap.SelectOption[string]{Value: u, Label: label})
			}

			haURL = tap.Select(ctx, tap.SelectOptions[string]{
				Message: fmt.Sprintf("Select URL for the Agent (%s):", selectedInst.Name),
				Options: agentOpts,
			})
			if ctx.Err() != nil {
				return nil, false, ctx.Err()
			}

			// 3. GRUB URL Selection (only if Agent is HTTPS)
			if strings.HasPrefix(haURL, "https://") {
				var grubOpts []tap.SelectOption[string]
				for _, u := range selectedInst.URLs {
					if !strings.HasPrefix(u, "https://") {
						grubOpts = append(grubOpts, tap.SelectOption[string]{Value: u, Label: u})
					}
				}

				if len(grubOpts) > 0 {
					grubURL = tap.Select(ctx, tap.SelectOptions[string]{
						Message: "Select HTTP URL for GRUB (HTTPS not readily supported in GRUB):",
						Options: grubOpts,
					})
					if ctx.Err() != nil {
						return nil, false, ctx.Err()
					}
				}
			}
		}
	}

	if haURL == "" {
		haURL = tap.Text(ctx, tap.TextOptions{
			Message: "Home Assistant URL",
			Validate: func(s string) error {
				if err := config.ValidateURL(s); err != nil {
					return err
				}

				// Optional: Allow skipping the check via environment variable
				if os.Getenv("GRUBSTATION_SKIP_URL_CHECK") == "true" {
					return nil
				}

				// Perform a quick connection check
				client := &http.Client{
					Timeout: 3 * time.Second,
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, s, nil)
				if err != nil {
					return fmt.Errorf("invalid request: %v", err)
				}

				resp, err := client.Do(req)
				if err != nil {
					return fmt.Errorf("could not connect to URL: %v", err)
				}
				defer resp.Body.Close()

				// We don't necessarily care about the status code (it might be 401/404 for HA without auth),
				// just that the server is reachable.
				return nil
			},
		})
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
	}

	// 9. HA Webhook ID
	haWebhook := tap.Password(ctx, tap.PasswordOptions{
		Message: "Home Assistant Webhook ID (generated by the integration)",
		Validate: func(s string) error {
			return config.ValidateWebhookID(s)
		},
	})
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	return &config.Config{
		Host: config.HostConfig{
			Address:    hostAddress,
			MACAddress: selectedIface.HardwareAddr.String(),
		},
		WakeOnLan: &config.WakeOnLanConfig{
			Address: WolBroadcastAddress,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       haURL,
			WebhookID: haWebhook,
		},
		Daemon: config.DaemonConfig{
			Port:              AgentPort,
			ReportBootOptions: reportsBoot,
		},
		Grub: &config.GrubConfig{
			WaitTimeSeconds: grubWaitTime,
			ConfigPath:      grubConfigPath,
			URL:             grubURL,
		},
	}, isDryRun, nil
}

func buildIfaceOptions(resolver SystemResolver, interfaces []net.Interface) []tap.SelectOption[int] {
	var opts []tap.SelectOption[int]
	for i, inf := range interfaces {
		ips, _ := resolver.GetIPInfo(inf)
		desc := fmt.Sprintf("(%s) [%s]", inf.HardwareAddr.String(), strings.Join(ips, ", "))
		opts = append(opts, tap.SelectOption[int]{
			Value: i,
			Label: inf.Name,
			Hint:  desc,
		})
	}
	return opts
}

func buildHostSelectOptions(hostname, fqdn string, ips []string) []tap.SelectOption[string] {
	var opts []tap.SelectOption[string]
	if fqdn != "" && fqdn != hostname {
		opts = append(opts, tap.SelectOption[string]{Value: fqdn, Label: fqdn, Hint: "FQDN"})
	}
	opts = append(opts, tap.SelectOption[string]{Value: hostname, Label: hostname, Hint: "Hostname"})
	for _, ip := range ips {
		opts = append(opts, tap.SelectOption[string]{Value: ip, Label: ip, Hint: "IP Address [Ensure This is Static!]"})
	}
	return opts
}

func buildWolSelectOptions(hostAddress string, ips []string, ipBroadcasts map[string]string) []tap.SelectOption[string] {
	opts := []tap.SelectOption[string]{
		{Value: config.DefaultWolBroadcastAddress, Label: fmt.Sprintf("%s (Default)", config.DefaultWolBroadcastAddress)},
	}

	selectedIP := net.ParseIP(hostAddress)
	isSelectedIPv4 := selectedIP != nil && selectedIP.To4() != nil

	seenBroadcasts := make(map[string]bool)
	for _, ip := range ips {
		bc, ok := ipBroadcasts[ip]
		if !ok {
			continue
		}
		isIPv4 := net.ParseIP(ip).To4() != nil
		if isSelectedIPv4 != isIPv4 {
			continue
		}
		if !seenBroadcasts[bc] {
			seenBroadcasts[bc] = true
			opts = append(opts, tap.SelectOption[string]{
				Value: bc,
				Label: fmt.Sprintf("%s (Subnet broadcast for %s)", bc, ip),
			})
		}
	}
	return opts
}

func PrintConfigSummary(cmd *cobra.Command, cfg *config.Config, cfgPath string) {
	maskWebhook := true
	out, err := cfg.ToYAML(maskWebhook)
	if err != nil {
		tap.Message(fmt.Sprintf("Error generating summary: %v", err))
		return
	}

	tap.Message(fmt.Sprintf("Configuration saved to %s", cfgPath))
	out = fmt.Sprintf("\n---\n%s", out)
	tap.Box(out, " Configuration Preview ", tap.BoxOptions{
		ContentPadding: 2,
	})
}

func validatePort(s string, isReinstall bool, currentPort int) error {
	if err := config.ValidatePort(s); err != nil {
		return err
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	if isReinstall && port == currentPort {
		return nil
	}

	if os.Getenv("GRUBSTATION_SKIP_PORT_CHECK") == "true" {
		return nil
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is in use or unavailable: %v", port, err)
	}
	_ = listener.Close()
	return nil
}
