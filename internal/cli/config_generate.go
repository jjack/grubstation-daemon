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
	saveConfigFile        = config.Save
	runGenerateSurvey     = generateConfigInteractive

	runBasicForm    = defaultRunBasicForm
	runAdvancedForm = defaultRunAdvancedForm
)

const (
	DefaultBroadcastAddress = "255.255.255.255"
	DefaultBroadcastPort    = "9"
	OptionCustomHost        = "Custom / Manual Entry"
)

type haDiscoveryResult struct {
	url string
	err error
}

type basicFormResults struct {
	EntityType string
	Name       string
	HAURL      string
	HAWebhook  string
	Bootloader string
	InitSystem string
	IfaceName  string
}

func defaultRunBasicForm(
	hostname string,
	haURL string,
	ifaceOpts []huh.Option[string],
	blOpts []string,
	initOpts []string,
) (basicFormResults, error) {
	res := basicFormResults{
		EntityType: string(config.EntityTypeButton),
		Name:       hostname,
		HAURL:      haURL,
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Home Assistant Entity Type").
				Description("Buttons cannot track on/off states, switches can.").
				Options(
					huh.NewOption("Button", string(config.EntityTypeButton)),
					huh.NewOption("Switch", string(config.EntityTypeSwitch)),
				).
				Value(&res.EntityType),
			huh.NewInput().Title("Name (how HA will refer to your machine):").Value(&res.Name),
		),
		huh.NewGroup(
			huh.NewInput().Title("Home Assistant URL:").Value(&res.HAURL).Validate(config.ValidateURL),
			huh.NewInput().Title("Home Assistant Generated Webhook ID:").Value(&res.HAWebhook).Validate(config.ValidateWebhookID),
		),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Physical Interface for WOL Packets:").Options(ifaceOpts...).Value(&res.IfaceName),
			huh.NewSelect[string]().Title("Init System:").Options(huh.NewOptions(initOpts...)...).Value(&res.InitSystem),
			huh.NewSelect[string]().Title("Bootloader:").Options(huh.NewOptions(blOpts...)...).Value(&res.Bootloader),
		),
	).Run()

	return res, err
}

type advancedFormResults struct {
	HostAddress    string
	Broadcast      string
	WOLPort        string
	BootloaderPath string
}

func defaultRunAdvancedForm(
	isSwitch bool,
	hostOpts []huh.Option[string],
	defaultHost string,
	defaultBroadcast string,
	defaultBLPath string,
) (advancedFormResults, error) {
	res := advancedFormResults{
		HostAddress:    defaultHost,
		Broadcast:      defaultBroadcast,
		WOLPort:        "9",
		BootloaderPath: defaultBLPath,
	}
	var customHost string

	var groups []*huh.Group

	if isSwitch {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Server address for ping checks").
				Description("Warning: If you choose an IP, it must be static").
				Options(hostOpts...).
				Value(&res.HostAddress),
		))
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title("Enter custom server address:").
				Value(&customHost).
				Validate(config.ValidateHost),
		).WithHideFunc(func() bool { return res.HostAddress != OptionCustomHost }))
	}

	groups = append(groups, huh.NewGroup(
		huh.NewInput().
			Title("WOL Broadcast Address:").
			Value(&res.Broadcast).
			Validate(config.ValidateBroadcastAddress),
		huh.NewInput().
			Title("Wake-on-LAN Port:").
			Description("Leave default (9) unless you know what you're doing").
			Value(&res.WOLPort).
			Validate(config.ValidateBroadcastPort),
	))

	groups = append(groups, huh.NewGroup(
		huh.NewInput().
			Title("Bootloader Config Path:").
			Value(&res.BootloaderPath).
			Validate(config.ValidateBootloaderConfigPath),
	))

	err := huh.NewForm(groups...).Run()

	if res.HostAddress == OptionCustomHost {
		res.HostAddress = customHost
	}

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

	var ifaceOpts []huh.Option[string]
	ifaceMap := make(map[string]net.Interface)
	for _, inf := range wolInterfaces {
		ifaceMap[inf.Name] = inf
		ips, _ := getIPv4Info(inf)
		desc := fmt.Sprintf("(%s) [%s]", inf.HardwareAddr.String(), strings.Join(ips, ", "))
		ifaceOpts = append(ifaceOpts, huh.NewOption(fmt.Sprintf("%s %s", inf.Name, desc), inf.Name))
	}

	blOpts := deps.BootloaderRegistry.SupportedBootloaders()
	initOpts := deps.InitRegistry.SupportedInitSystems()

	// Wait for HA Discovery
	var haURL string
	select {
	case res := <-haDiscoveryResultChan:
		haURL = res.url
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 3. Run Basic Form
	basicRes, err := runBasicForm(hostname, haURL, ifaceOpts, blOpts, initOpts)
	if err != nil {
		return nil, err
	}

	// 4. Process basic results
	selectedIface := ifaceMap[basicRes.IfaceName]
	macAddress := selectedIface.HardwareAddr.String()
	if err := config.ValidateMACAddress(macAddress); err != nil {
		return nil, err
	}

	ips, ipBroadcasts := getIPv4Info(selectedIface)

	hostOpts := []huh.Option[string]{
		huh.NewOption(hostname, hostname),
	}
	for _, ip := range ips {
		hostOpts = append(hostOpts, huh.NewOption(ip, ip))
	}
	hostOpts = append(hostOpts, huh.NewOption(OptionCustomHost, OptionCustomHost))

	defaultBroadcast := DefaultBroadcastAddress
	for _, bc := range ipBroadcasts {
		defaultBroadcast = bc
		break
	}

	bl := deps.BootloaderRegistry.Get(basicRes.Bootloader)
	var blPath string
	if bl != nil {
		blPath, _ = bl.DiscoverConfigPath(ctx)
	}

	// 5. Run Advanced Form
	isSwitch := basicRes.EntityType == string(config.EntityTypeSwitch)
	advRes, err := runAdvancedForm(isSwitch, hostOpts, hostname, defaultBroadcast, blPath)
	if err != nil {
		return nil, err
	}

	wolPort, _ := strconv.Atoi(advRes.WOLPort)

	return &config.Config{
		Server: config.ServerConfig{
			Name:             basicRes.Name,
			Host:             advRes.HostAddress,
			MACAddress:       macAddress,
			BroadcastAddress: advRes.Broadcast,
			BroadcastPort:    wolPort,
		},
		Bootloader: config.BootloaderConfig{
			Name:       basicRes.Bootloader,
			ConfigPath: advRes.BootloaderPath,
		},
		InitSystem: config.InitSystemConfig{
			Name: basicRes.InitSystem,
		},
		HomeAssistant: config.HomeAssistantConfig{
			EntityType: config.EntityType(basicRes.EntityType),
			URL:        basicRes.HAURL,
			WebhookID:  basicRes.HAWebhook,
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

			fmt.Println("\nGenerated config (keys may be in a different order than shown here):")
			fmt.Printf("---\n")
			fmt.Printf("host:\n  name: %s\n  host: %s\n  mac_address: %s\n  broadcast_address: %s\n  broadcast_port: %d\n", cfg.Server.Name, cfg.Server.Host, cfg.Server.MACAddress, cfg.Server.BroadcastAddress, cfg.Server.BroadcastPort)
			fmt.Printf("homeassistant:\n  url: %s\n  webhook_id: %s\n  entity_type: %s\n", cfg.HomeAssistant.URL, cfg.HomeAssistant.WebhookID, cfg.HomeAssistant.EntityType)
			fmt.Printf("bootloader:\n  name: %s\n  config_path: %s\n", cfg.Bootloader.Name, cfg.Bootloader.ConfigPath)
			fmt.Printf("initsystem:\n  name: %s\n", cfg.InitSystem.Name)

			cfgPath, err := cmd.Flags().GetString("path")
			if err != nil {
				cfgPath = "./config.yaml"
			}

			return saveConfigFile(cfg, cfgPath)
		},
	}

	cmd.Flags().String("path", "./config.yaml", "Path to save the generated config file")
	return cmd
}
