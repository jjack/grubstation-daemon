package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/jjack/remote-boot-agent/internal/system"
	"github.com/spf13/cobra"
)

var (
	discoverHomeAssistant = homeassistant.Discover
	detectSystemHostname  = system.DetectHostname
	getWOLInterfaces      = system.GetWOLInterfaces
	getIPv4Info           = system.GetIPv4Info
	getFQDN               = system.GetFQDN
	saveConfigFile        = config.Save
	runGenerateSurvey     = generateConfigInteractive

	runHostInfoForm   = defaultRunHostInfoForm
	runWOLForm        = defaultRunWOLForm
	runBootloaderForm = defaultRunBootloaderForm
	runInitSystemForm = defaultRunInitSystemForm
	runHAForm         = defaultRunHAForm
)

const (
	OptionCustomHost = "Custom / Manual Entry"
)

type haDiscoveryResult struct {
	url string
	err error
}

func buildIfaceOptions(wolInterfaces []net.Interface) ([]huh.Option[string], map[string]net.Interface) {
	var ifaceOpts []huh.Option[string]
	ifaceMap := make(map[string]net.Interface)
	for _, inf := range wolInterfaces {
		ifaceMap[inf.Name] = inf
		ips, _ := getIPv4Info(inf)
		desc := fmt.Sprintf("(%s) [%s]", inf.HardwareAddr.String(), strings.Join(ips, ", "))
		ifaceOpts = append(ifaceOpts, huh.NewOption(fmt.Sprintf("%s %s", inf.Name, desc), inf.Name))
	}
	return ifaceOpts, ifaceMap
}

func buildHostOptions(hostname, fqdn string, ips []string) []huh.Option[string] {
	hostOpts := []huh.Option[string]{
		huh.NewOption(hostname, hostname),
	}
	if fqdn != "" && fqdn != hostname {
		hostOpts = append(hostOpts, huh.NewOption(fmt.Sprintf("%s (FQDN)", fqdn), fqdn))
	}
	for _, ip := range ips {
		hostOpts = append(hostOpts, huh.NewOption(ip, ip))
	}
	hostOpts = append(hostOpts, huh.NewOption(OptionCustomHost, OptionCustomHost))
	return hostOpts
}

func buildBroadcastOptions(hostAddress string, ips []string, ipBroadcasts map[string]string) []huh.Option[string] {
	broadcastOpts := []huh.Option[string]{
		huh.NewOption("Default Broadcast (255.255.255.255)", config.DefaultBroadcastAddress),
	}
	selectedIP := net.ParseIP(hostAddress)
	isSelectedIPv4 := selectedIP != nil && selectedIP.To4() != nil
	seenBroadcasts := make(map[string]bool)
	for _, ip := range ips {
		bc := ipBroadcasts[ip]
		isIPv4 := net.ParseIP(ip).To4() != nil
		if isSelectedIPv4 && !isIPv4 {
			continue
		}
		if !seenBroadcasts[bc] {
			seenBroadcasts[bc] = true
			broadcastOpts = append(broadcastOpts, huh.NewOption(fmt.Sprintf("Subnet Broadcast (%s)", bc), bc))
		}
	}
	broadcastOpts = append(broadcastOpts, huh.NewOption("Custom / Manual Entry", "custom"))
	return broadcastOpts
}

type initSystemResults struct {
	Name string
}

func defaultRunInitSystemForm(initOpts []string) (initSystemResults, error) {
	res := initSystemResults{}
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Autodetected Supported Init Systems:").Options(huh.NewOptions(initOpts...)...).Value(&res.Name),
		).Title("Init System Configuration"),
	).Run()
	return res, err
}

type bootloaderResults struct {
	Name       string
	ConfigPath string
}

func defaultRunBootloaderForm(blOpts []string, deps *CommandDeps, ctx context.Context) (bootloaderResults, error) {
	res := bootloaderResults{}
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Autodetected Supported Bootloaders:").Options(huh.NewOptions(blOpts...)...).Value(&res.Name),
		).Title("Bootloader Configuration"),
	).Run()
	if err != nil {
		return res, err
	}

	bl := deps.BootloaderRegistry.Get(res.Name)
	if bl != nil {
		res.ConfigPath, _ = bl.DiscoverConfigPath(ctx)
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Bootloader Config Path:").
				Value(&res.ConfigPath).
				Validate(config.ValidateBootloaderConfigPath),
		).Title("Bootloader Configuration"),
	).Run()
	return res, err
}

type hostInfoResults struct {
	Name        string
	IfaceName   string
	HostAddress string
	MACAddress  string
}

func defaultRunHostInfoForm(ifaceOpts []huh.Option[string], ifaceMap map[string]net.Interface, hostname string) (hostInfoResults, []huh.Option[string], error) {
	res := hostInfoResults{
		Name: hostname,
	}
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Select a Physical Interface for the WOL Packets:").Options(ifaceOpts...).Value(&res.IfaceName),
		).Title("Host Information"),
	).Run()
	if err != nil {
		return res, nil, err
	}

	selectedIface := ifaceMap[res.IfaceName]
	res.MACAddress = selectedIface.HardwareAddr.String()

	ips, ipBroadcasts := getIPv4Info(selectedIface)

	fqdn := getFQDN(hostname)
	hostOpts := buildHostOptions(hostname, fqdn, ips)

	var customHost string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Machine Name").Description("What to call this machine in Home Assistant").Value(&res.Name).Validate(config.ValidateName),
			huh.NewSelect[string]().
				Title("Ping Target").
				Description("(If you select an IP, it should be static)").
				Options(hostOpts...).
				Value(&res.HostAddress),
		).Title("Host Information"),
		huh.NewGroup(
			huh.NewInput().
				Title("Enter custom address:").
				Value(&customHost).
				Validate(config.ValidateHost),
		).Title("Host Information").WithHideFunc(func() bool { return res.HostAddress != OptionCustomHost }),
	).Run()

	if res.HostAddress == OptionCustomHost {
		res.HostAddress = customHost
	}

	broadcastOpts := buildBroadcastOptions(res.HostAddress, ips, ipBroadcasts)

	return res, broadcastOpts, err
}

type wolResults struct {
	Broadcast string
	WOLPort   string
}

func defaultRunWOLForm(broadcastOpts []huh.Option[string]) (wolResults, error) {
	res := wolResults{
		Broadcast: broadcastOpts[0].Value,
		WOLPort:   strconv.Itoa(config.DefaultBroadcastPort),
	}
	var customBroadcast string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Broadcast Address:").
				Description("(select Subnet Broadcast if HA is on a different VLAN)").
				Options(broadcastOpts...).
				Value(&res.Broadcast),
		).Title("Wake on Lan Configuration"),
		huh.NewGroup(
			huh.NewInput().
				Title("Enter custom broadcast address:").
				Value(&customBroadcast).
				Validate(config.ValidateBroadcastAddress),
		).Title("Wake on Lan Configuration").WithHideFunc(func() bool { return res.Broadcast != "custom" }),
		huh.NewGroup(
			huh.NewInput().
				Title("Wake-on-LAN Port:").
				Description("Leave default (9) unless you know what you're doing").
				Value(&res.WOLPort).
				Validate(config.ValidateBroadcastPort),
		).Title("Wake on Lan Configuration"),
	).Run()

	if res.Broadcast == "custom" {
		res.Broadcast = customBroadcast
	}
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

func generateConfigInteractive(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
	// 1. Fetch async HA Discovery
	haDiscoveryResultChan := make(chan haDiscoveryResult, 1)
	go func() {
		url, err := discoverHomeAssistant(ctx)
		haDiscoveryResultChan <- haDiscoveryResult{url: url, err: err}
	}()

	// 2. Fetch basic system info
	hostname, err := detectSystemHostname()
	if err != nil {
		return nil, err
	}

	wolInterfaces, err := getWOLInterfaces()
	if err != nil {
		return nil, err
	}

	ifaceOpts, ifaceMap := buildIfaceOptions(wolInterfaces)

	blOpts := deps.BootloaderRegistry.SupportedBootloaders()
	initOpts := deps.InitRegistry.SupportedInitSystems()

	initRes, err := runInitSystemForm(initOpts)
	if err != nil {
		return nil, err
	}

	blRes, err := runBootloaderForm(blOpts, deps, ctx)
	if err != nil {
		return nil, err
	}

	hostRes, broadcastOpts, err := runHostInfoForm(ifaceOpts, ifaceMap, hostname)
	if err != nil {
		return nil, err
	}

	if err := config.ValidateMACAddress(hostRes.MACAddress); err != nil {
		return nil, err
	}

	wolRes, err := runWOLForm(broadcastOpts)
	if err != nil {
		return nil, err
	}

	// Wait for HA Discovery
	var haURL string
	select {
	case res := <-haDiscoveryResultChan:
		haURL = res.url
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	haRes, err := runHAForm(haURL)
	if err != nil {
		return nil, err
	}

	wolPort, _ := strconv.Atoi(wolRes.WOLPort)

	return &config.Config{
		Host: config.HostConfig{
			Name:             hostRes.Name,
			Address:          hostRes.HostAddress,
			MACAddress:       hostRes.MACAddress,
			BroadcastAddress: wolRes.Broadcast,
			BroadcastPort:    wolPort,
		},
		Bootloader: config.BootloaderConfig{
			Name:       blRes.Name,
			ConfigPath: blRes.ConfigPath,
		},
		InitSystem: config.InitSystemConfig{
			Name: initRes.Name,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       haRes.URL,
			WebhookID: haRes.WebhookID,
		},
	}, nil
}

func ensureSupport(ctx context.Context, deps *CommandDeps) error {
	_, err := deps.BootloaderRegistry.Detect(ctx)
	if err != nil {
		if errors.Is(err, bootloader.ErrNotSupported) {
			supported := strings.Join(deps.BootloaderRegistry.SupportedBootloaders(), ", ")
			return fmt.Errorf("no supported bootloader detected. Please ensure you have one of the following installed: %s", supported)
		}
		return err
	}

	_, err = deps.InitRegistry.Detect(ctx)
	if err != nil {
		if errors.Is(err, initsystem.ErrNotSupported) {
			supported := strings.Join(deps.InitRegistry.SupportedInitSystems(), ", ")
			return fmt.Errorf("no supported init system detected. Please ensure you have one of the following installed: %s", supported)
		}
		return err
	}
	return nil
}

// NewConfigGenerateCmd walks the user through generating a config interactively
func NewConfigGenerateCmd(deps *CommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Interactively generate a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureSupport(cmd.Context(), deps); err != nil {
				return err
			}

			// Clear the terminal screen before starting the interactive prompts
			cmd.Print("\033[H\033[2J")

			cfg, err := runGenerateSurvey(cmd.Context(), deps)
			if err != nil {
				return err
			}

			cfgPath, err := cmd.Flags().GetString("path")
			if err != nil {
				cfgPath = "./config.yaml"
			}

			cmd.Printf("\nConfig file saved to %s\n", cfgPath)
			cmd.Println("(note: keys may be in a different order than shown here)")
			cmd.Printf("---\n")

			var broadcastStr string
			if cfg.Host.BroadcastAddress != "" && cfg.Host.BroadcastAddress != config.DefaultBroadcastAddress {
				broadcastStr += fmt.Sprintf("\n  broadcast_address: %s", cfg.Host.BroadcastAddress)
			}
			if cfg.Host.BroadcastPort != 0 && cfg.Host.BroadcastPort != config.DefaultBroadcastPort {
				broadcastStr += fmt.Sprintf("\n  broadcast_port: %d", cfg.Host.BroadcastPort)
			}

			safeWebhookID := cfg.HomeAssistant.WebhookID
			if len(safeWebhookID) > 4 {
				safeWebhookID = safeWebhookID[:4] + "..."
			}
			cmd.Printf("host:\n  name: %s\n  address: %s\n  mac: %s%s\n\n", cfg.Host.Name, cfg.Host.Address, cfg.Host.MACAddress, broadcastStr)
			cmd.Printf("homeassistant:\n  url: %s\n  webhook_id: %s\n\n", cfg.HomeAssistant.URL, safeWebhookID)
			cmd.Printf("bootloader:\n  name: %s\n  config_path: %s\n\n", cfg.Bootloader.Name, cfg.Bootloader.ConfigPath)
			cmd.Printf("initsystem:\n  name: %s\n", cfg.InitSystem.Name)

			return saveConfigFile(cfg, cfgPath)
		},
	}

	cmd.Flags().String("path", "./config.yaml", "Path to save the generated config file")
	return cmd
}
