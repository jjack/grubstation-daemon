package survey

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"github.com/jjack/grub-os-reporter/internal/config"
	"github.com/spf13/cobra"
)

type SystemResolver interface {
	DiscoverHomeAssistant(ctx context.Context) (string, error)
	DetectSystemHostname() (string, error)
	GetWOLInterfaces() ([]net.Interface, error)
	GetIPv4Info(inf net.Interface) ([]string, map[string]string)
	GetFQDN(hostname string) string
	DiscoverGrubConfig(ctx context.Context) (string, error)
}

type SurveyDeps interface {
	GetSystemResolver() SystemResolver
}

var (
	RunGenerateSurvey = generateConfigInteractive

	runNetworkingIfaceForm = defaultRunNetworkingIfaceForm
	runAgentConfigForm     = defaultRunAgentConfigForm
	runHostInfoForm        = defaultRunHostInfoForm
	runWOLForm             = defaultRunWOLForm
	runHAForm              = defaultRunHAForm
)

const (
	OptionCustomHost = "Custom / Manual Entry"

	ModeDaemonBoth     = "Daemon (Remote shutdown + Report boot options)"
	ModeDaemonShutdown = "Daemon (Remote shutdown only)"
	ModeHookOnly       = "Shutdown hook (Report boot options only)"
)

type haDiscoveryResult struct {
	url string
	err error
}

func buildIfaceOptions(resolver SystemResolver, wolInterfaces []net.Interface) ([]huh.Option[string], map[string]net.Interface) {
	var ifaceOpts []huh.Option[string]
	ifaceMap := make(map[string]net.Interface)
	for _, inf := range wolInterfaces {
		ifaceMap[inf.Name] = inf
		ips, _ := resolver.GetIPv4Info(inf)
		desc := fmt.Sprintf("(%s) [%s]", inf.HardwareAddr.String(), strings.Join(ips, ", "))
		ifaceOpts = append(ifaceOpts, huh.NewOption(fmt.Sprintf("%s %s", inf.Name, desc), inf.Name))
	}
	return ifaceOpts, ifaceMap
}

func buildHostOptions(hostname, fqdn string, ips []string) []huh.Option[string] {
	hostOpts := []huh.Option[string]{}

	if fqdn != "" && fqdn != hostname {
		hostOpts = append(hostOpts, huh.NewOption(fmt.Sprintf("%s (FQDN)", fqdn), fqdn))
	}
	hostOpts = append(hostOpts, huh.NewOption(hostname, hostname))
	for _, ip := range ips {
		hostOpts = append(hostOpts, huh.NewOption(ip, ip))
	}
	hostOpts = append(hostOpts, huh.NewOption(OptionCustomHost, OptionCustomHost))
	return hostOpts
}

func buildWolOptions(hostAddress string, ips []string, ipBroadcasts map[string]string) []huh.Option[string] {
	wolOpts := []huh.Option[string]{
		huh.NewOption("Default WOL Address (255.255.255.255)", config.DefaultWolAddress),
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
		if isSelectedIPv4 && !isIPv4 {
			continue
		}
		if !seenBroadcasts[bc] {
			seenBroadcasts[bc] = true
			wolOpts = append(wolOpts, huh.NewOption(fmt.Sprintf("Subnet Broadcast (%s)", bc), bc))
		}
	}
	return wolOpts
}

func defaultRunNetworkingIfaceForm(ifaceOpts []huh.Option[string]) (string, error) {
	var ifaceName string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Select a Physical Network Interface:").Options(ifaceOpts...).Value(&ifaceName),
		).Title("Networking"),
	).Run()
	return ifaceName, err
}

type hostInfoResults struct {
	Name        string
	HostAddress string
}

func defaultRunHostInfoForm(hostOpts []huh.Option[string], hostname string) (hostInfoResults, error) {
	res := hostInfoResults{
		Name: hostname,
	}

	var customHost string
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().Title("Name").Description("What to call this machine in Home Assistant").Value(&res.Name).Validate(config.ValidateName),
			huh.NewSelect[string]().
				Title("Host Address").
				Description("Ping target and address for the shutdown daemon (If you select an IP, it should be static)").
				Options(hostOpts...).
				Value(&res.HostAddress),
		).Title("Host Information"),
		huh.NewGroup(
			huh.NewInput().
				Title("Enter custom address:").
				Value(&customHost).
				Validate(config.ValidateHost),
		).Title("Host Information").WithHideFunc(func() bool { return res.HostAddress != OptionCustomHost }),
	}

	err := huh.NewForm(groups...).Run()

	if res.HostAddress == OptionCustomHost {
		res.HostAddress = customHost
	}

	return res, err
}

type agentConfigResults struct {
	Mode                string
	DaemonPort          string
	GrubWaitTimeSeconds string
}

func validatePort(s string) error {
	if s == "" {
		return errors.New("port cannot be empty")
	}
	port, err := strconv.Atoi(s)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid port: must be a number between 1 and 65535")
	}
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is in use or unavailable: %v", port, err)
	}
	_ = l.Close()
	return nil
}

func defaultRunAgentConfigForm(hasGrub bool) (agentConfigResults, error) {
	res := agentConfigResults{
		DaemonPort:          "8081",
		GrubWaitTimeSeconds: "2",
	}

	var modeOpts []huh.Option[string]
	if hasGrub {
		modeOpts = []huh.Option[string]{
			huh.NewOption(ModeDaemonBoth, ModeDaemonBoth),
			huh.NewOption(ModeDaemonShutdown, ModeDaemonShutdown),
			huh.NewOption(ModeHookOnly, ModeHookOnly),
		}
		res.Mode = ModeDaemonBoth
	} else {
		modeOpts = []huh.Option[string]{
			huh.NewOption(ModeDaemonShutdown, ModeDaemonShutdown),
		}
		res.Mode = ModeDaemonShutdown
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Installation Mode").
				Options(modeOpts...).
				Value(&res.Mode),
		).Title("Agent Configuration"),
		huh.NewGroup(
			huh.NewInput().
				Title("Daemon Port").
				Description("Port for the daemon to listen on").
				Value(&res.DaemonPort).
				Validate(validatePort),
		).Title("Agent Configuration").WithHideFunc(func() bool { return res.Mode == ModeHookOnly }),
		huh.NewGroup(
			huh.NewInput().
				Title("GRUB Network Wait Time (seconds)").
				Description("How long to wait for network connectivity before falling back to default OS.\nRecommended: 1-2 seconds for most networks, up to 35 if you need to wait for Spanning Tree Protocol.").
				Placeholder("2").
				Value(&res.GrubWaitTimeSeconds).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("wait time cannot be empty")
					}
					val, err := strconv.Atoi(s)
					if err != nil {
						return errors.New("wait time must be a number")
					}
					if val < 1 || val > 300 {
						return errors.New("wait time must be between 1 and 300 seconds")
					}
					return nil
				}),
		).Title("Agent Configuration").WithHideFunc(func() bool { return res.Mode == ModeDaemonShutdown }),
	}

	err := huh.NewForm(groups...).Run()
	return res, err
}

type wolResults struct {
	Broadcast string
	WOLPort   string
}

func defaultRunWOLForm(broadcastOpts []huh.Option[string]) (wolResults, error) {
	res := wolResults{
		Broadcast: broadcastOpts[0].Value,
		WOLPort:   strconv.Itoa(config.DefaultWolPort),
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Broadcast Address:").
				Description("(select Subnet Broadcast if HA is on a different VLAN)").
				Options(broadcastOpts...).
				Value(&res.Broadcast),
		).Title("Networking"),
	).Run()

	return res, err
}

type haResults struct {
	URL       string
	WebhookID string
}

func defaultRunHAForm(defaultURL string) (haResults, error) {
	res := haResults{
		URL: defaultURL,
	}
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Home Assistant URL:").Value(&res.URL).Validate(config.ValidateURL),
			huh.NewInput().Title("Home Assistant Generated Webhook ID:").Value(&res.WebhookID).Validate(config.ValidateWebhookID),
		).Title("Home Assistant Configuration"),
	).Run()

	return res, err
}

func generateConfigInteractive(ctx context.Context, deps SurveyDeps) (*config.Config, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 1. Fetch async HA Discovery
	// Run mDNS discovery concurrently in the background so it doesn't block the initial interactive CLI prompts.
	haDiscoveryResultChan := make(chan haDiscoveryResult, 1)
	go func() {
		url, err := deps.GetSystemResolver().DiscoverHomeAssistant(ctx)
		haDiscoveryResultChan <- haDiscoveryResult{url: url, err: err}
	}()

	var grubConfigPath string
	hasGrub := false
	isLinux := runtime.GOOS == "linux"
	if isLinux {
		if path, err := deps.GetSystemResolver().DiscoverGrubConfig(ctx); err == nil {
			grubConfigPath = path
			hasGrub = true
		}
	}

	// 2. Fetch basic system info
	hostname, err := deps.GetSystemResolver().DetectSystemHostname()
	if err != nil {
		return nil, err
	}

	wolInterfaces, err := deps.GetSystemResolver().GetWOLInterfaces()
	if err != nil {
		return nil, err
	}

	ifaceOpts, ifaceMap := buildIfaceOptions(deps.GetSystemResolver(), wolInterfaces)

	ifaceName, err := runNetworkingIfaceForm(ifaceOpts)
	if err != nil {
		return nil, err
	}

	selectedIface := ifaceMap[ifaceName]
	macAddress := selectedIface.HardwareAddr.String()

	ips, ipBroadcasts := deps.GetSystemResolver().GetIPv4Info(selectedIface)
	fqdn := deps.GetSystemResolver().GetFQDN(hostname)
	hostOpts := buildHostOptions(hostname, fqdn, ips)

	hostRes, err := runHostInfoForm(hostOpts, hostname)
	if err != nil {
		return nil, err
	}

	if err := config.ValidateMACAddress(macAddress); err != nil {
		return nil, err
	}

	agentRes, err := runAgentConfigForm(hasGrub)
	if err != nil {
		return nil, err
	}

	reportBootOptions := agentRes.Mode == ModeDaemonBoth || agentRes.Mode == ModeHookOnly
	runDaemon := agentRes.Mode == ModeDaemonBoth || agentRes.Mode == ModeDaemonShutdown

	var daemonPort int
	if runDaemon {
		daemonPort, _ = strconv.Atoi(agentRes.DaemonPort)
	}

	var grubWaitTime int
	if reportBootOptions {
		grubWaitTime, _ = strconv.Atoi(agentRes.GrubWaitTimeSeconds)
	} else {
		grubConfigPath = ""
	}

	wolOpts := buildWolOptions(hostRes.HostAddress, ips, ipBroadcasts)

	var wolRes wolResults
	selectedIP := net.ParseIP(hostRes.HostAddress)
	isIPv6 := selectedIP != nil && selectedIP.To4() == nil

	// If we're on linux and the user has opted out of GRUB reporting, we assume they don't want
	// to configure WOL settings (broadcast address/port) and we use defaults to reduce friction.
	// Windows users (who don't have GRUB) likely still want to configure WOL for waking the machine.
	if isIPv6 {
		wolRes = wolResults{
			Broadcast: hostRes.HostAddress,
			WOLPort:   strconv.Itoa(config.DefaultWolPort),
		}
	} else if reportBootOptions || runtime.GOOS == "windows" {
		if len(wolOpts) == 1 {
			wolRes = wolResults{
				Broadcast: wolOpts[0].Value,
				WOLPort:   strconv.Itoa(config.DefaultWolPort),
			}
		} else {
			wolRes, err = runWOLForm(wolOpts)
			if err != nil {
				return nil, err
			}
		}
	} else {
		wolRes = wolResults{
			Broadcast: config.DefaultWolAddress,
			WOLPort:   strconv.Itoa(config.DefaultWolPort),
		}
	}

	// Wait for HA Discovery
	var haURL string
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-haDiscoveryResultChan:
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		haURL = res.url
	}

	haRes, err := runHAForm(haURL)
	if err != nil {
		return nil, err
	}

	wolPort, _ := strconv.Atoi(wolRes.WOLPort)

	return &config.Config{
		Host: config.HostConfig{
			Name:       hostRes.Name,
			Address:    hostRes.HostAddress,
			MACAddress: macAddress,
		},
		WakeOnLan: config.WakeOnLanConfig{
			Address: wolRes.Broadcast,
			Port:    wolPort,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       haRes.URL,
			WebhookID: haRes.WebhookID,
		},
		Daemon: config.DaemonConfig{
			ListenPort:        daemonPort,
			ReportBootOptions: reportBootOptions,
		},
		Grub: config.GrubConfig{
			WaitTimeSeconds: grubWaitTime,
			ConfigPath:      grubConfigPath,
		},
	}, nil
}

func PrintConfigSummary(cmd *cobra.Command, cfg *config.Config, cfgPath string) {
	cmd.Printf("\nConfig file saved to %s\n", cfgPath)
	cmd.Println("(note: keys may be in a different order than shown here)")
	cmd.Printf("---\n")

	var wolStr string
	if cfg.WakeOnLan.Address != "" && cfg.WakeOnLan.Address != config.DefaultWolAddress {
		wolStr += fmt.Sprintf("\n  address: %s", cfg.WakeOnLan.Address)
	}
	if cfg.WakeOnLan.Port != 0 && cfg.WakeOnLan.Port != config.DefaultWolPort {
		wolStr += fmt.Sprintf("\n  port: %d", cfg.WakeOnLan.Port)
	}

	safeWebhookID := cfg.HomeAssistant.WebhookID
	if len(safeWebhookID) > 4 {
		safeWebhookID = safeWebhookID[:4] + "..."
	}
	cmd.Printf("host:\n  name: %s\n  address: %s\n  mac: %s\n", cfg.Host.Name, cfg.Host.Address, cfg.Host.MACAddress)
	if wolStr != "" {
		cmd.Printf("wake_on_lan:%s\n", wolStr)
	}
	cmd.Println()
	cmd.Printf("homeassistant:\n  url: %s\n  webhook_id: %s\n\n", cfg.HomeAssistant.URL, safeWebhookID)
	cmd.Printf("daemon:\n  listen_port: %d\n", cfg.Daemon.ListenPort)
	if runtime.GOOS == "linux" {
		cmd.Printf("  report_boot_options: %v\n", cfg.Daemon.ReportBootOptions)
		if cfg.Daemon.ReportBootOptions {
			cmd.Printf("grub:\n  wait_time_seconds: %d\n", cfg.Grub.WaitTimeSeconds)
		}
	}
}
