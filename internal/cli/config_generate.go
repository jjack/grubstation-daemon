package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/jjack/remote-boot-agent/internal/system"
	"github.com/spf13/cobra"
)

var (
	surveyAskOne                = survey.AskOne
	systemGetBroadcastAddresses = system.GetBroadcastAddresses
	discoverHomeAssistant       = homeassistant.Discover
	detectSystemHostname        = system.DetectHostname
	getSystemInterfaces         = system.GetInterfaceOptions
	saveConfigFile              = config.Save
	runGenerateSurvey           = generateConfigInteractive
)

type haDiscoveryResult struct {
	url string
	err error
}

func surveyValidator(valFunc func(string) error) survey.Validator {
	return func(val interface{}) error {
		if str, ok := val.(string); ok {
			return valFunc(str)
		}
		return nil
	}
}

func askHostConfig() (config.HostConfig, error) {
	hostname, err := detectSystemHostname()
	if err != nil {
		return config.HostConfig{}, err
	}

	var finalHostname string
	err = surveyAskOne(&survey.Input{
		Message: "Hostname (how Home Assistant will refer to your machine):",
		Default: hostname,
	}, &finalHostname, survey.WithValidator(surveyValidator(config.ValidateHostname)))
	if err != nil {
		return config.HostConfig{}, err
	}

	interfaceOptions, err := getSystemInterfaces()
	if err != nil {
		return config.HostConfig{}, err
	}

	var ifaceOptions []string
	ifaceMap := make(map[string]string)
	for _, opt := range interfaceOptions {
		ifaceOptions = append(ifaceOptions, opt.Label)
		ifaceMap[opt.Label] = opt.Value
	}

	var selectedIfaceLabel string
	err = surveyAskOne(&survey.Select{
		Message: "Select Physical WOL Interface",
		Options: ifaceOptions,
	}, &selectedIfaceLabel)
	if err != nil {
		return config.HostConfig{}, err
	}

	macAddress := ifaceMap[selectedIfaceLabel]
	if err := config.ValidateMACAddress(macAddress); err != nil {
		return config.HostConfig{}, err
	}

	broadcastAddrs, _ := systemGetBroadcastAddresses(macAddress)
	var chosenBroadcast string
	if len(broadcastAddrs) > 0 {
		if len(broadcastAddrs) == 1 {
			chosenBroadcast = broadcastAddrs[0]
		} else {
			err = surveyAskOne(&survey.Select{
				Message: "Multiple WOL Subnet/Broadcast Addresses were discovered. Please select one:",
				Options: broadcastAddrs,
			}, &chosenBroadcast)
			if err != nil {
				return config.HostConfig{}, err
			}
		}
	}

	var finalBroadcast string
	err = surveyAskOne(&survey.Input{
		Message: "WOL Broadcast Address:",
		Default: chosenBroadcast,
	}, &finalBroadcast)
	if err != nil {
		return config.HostConfig{}, err
	}

	var wolPortStr string
	err = surveyAskOne(&survey.Input{
		Message: "Wake-on-LAN Port (leave blank for default):",
	}, &wolPortStr, survey.WithValidator(surveyValidator(config.ValidateBroadcastPort)))
	if err != nil {
		return config.HostConfig{}, err
	}

	wolPort, _ := strconv.Atoi(wolPortStr)
	if wolPort == 0 {
		wolPort = 9
	}

	return config.HostConfig{
		Hostname:         finalHostname,
		MACAddress:       macAddress,
		BroadcastAddress: finalBroadcast,
		BroadcastPort:    wolPort,
	}, nil
}

func askBootloaderConfig(ctx context.Context, registry *bootloader.Registry) (config.BootloaderConfig, error) {
	var blName string
	err := surveyAskOne(&survey.Select{
		Message: "Bootloader:",
		Options: registry.SupportedBootloaders(),
	}, &blName)
	if err != nil {
		return config.BootloaderConfig{}, err
	}

	var blPath string
	bl := registry.Get(blName)
	if bl != nil {
		blPath, _ = bl.DiscoverConfigPath(ctx)
	}

	if blPath != "" {
		err = surveyAskOne(&survey.Input{
			Message: "Bootloader Config Path:",
			Default: blPath,
		}, &blPath)
		if err != nil {
			return config.BootloaderConfig{}, err
		}
	}

	return config.BootloaderConfig{Name: blName, ConfigPath: blPath}, nil
}

func askInitSystemConfig(registry *initsystem.Registry) (config.InitSystemConfig, error) {
	var initSysName string
	err := surveyAskOne(&survey.Select{
		Message: "Init System:",
		Options: registry.SupportedInitSystems(),
	}, &initSysName)
	if err != nil {
		return config.InitSystemConfig{}, err
	}

	return config.InitSystemConfig{Name: initSysName}, nil
}

func askHomeAssistantConfig(ctx context.Context, discoveryChan <-chan haDiscoveryResult) (config.HomeAssistantConfig, error) {
	var discoveryResult haDiscoveryResult
	select {
	case discoveryResult = <-discoveryChan:
	case <-ctx.Done():
		return config.HomeAssistantConfig{}, ctx.Err()
	}

	var finalHassURL string
	err := surveyAskOne(&survey.Input{
		Message: "Home Assistant URL:",
		Default: discoveryResult.url,
	}, &finalHassURL, survey.WithValidator(surveyValidator(config.ValidateURL)))
	if err != nil {
		return config.HomeAssistantConfig{}, err
	}

	var webhookID string
	err = surveyAskOne(&survey.Input{
		Message: "Home Assistant Webhook ID:",
	}, &webhookID, survey.WithValidator(surveyValidator(config.ValidateWebhookID)))
	if err != nil {
		return config.HomeAssistantConfig{}, err
	}

	return config.HomeAssistantConfig{URL: finalHassURL, WebhookID: webhookID}, nil
}

func generateConfigInteractive(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
	haDiscoveryResultChan := make(chan haDiscoveryResult, 1)
	go func() {
		url, err := discoverHomeAssistant(ctx)
		haDiscoveryResultChan <- haDiscoveryResult{url: url, err: err}
	}()

	hostCfg, err := askHostConfig()
	if err != nil {
		return nil, err
	}

	blCfg, err := askBootloaderConfig(ctx, deps.BootloaderRegistry)
	if err != nil {
		return nil, err
	}

	initCfg, err := askInitSystemConfig(deps.InitRegistry)
	if err != nil {
		return nil, err
	}

	haCfg, err := askHomeAssistantConfig(ctx, haDiscoveryResultChan)
	if err != nil {
		return nil, err
	}

	return &config.Config{
		Host:          hostCfg,
		Bootloader:    blCfg,
		InitSystem:    initCfg,
		HomeAssistant: haCfg,
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

			cfg, err := runGenerateSurvey(cmd.Context(), deps)
			if err != nil {
				return err
			}

			fmt.Println("\nGenerated config (keys may be in a different order than shown here):")
			fmt.Printf("---\n")
			fmt.Printf("host:\n  hostname: %s\n  mac_address: %s\n  broadcast_address: %s\n  broadcast_port: %d\n", cfg.Host.Hostname, cfg.Host.MACAddress, cfg.Host.BroadcastAddress, cfg.Host.BroadcastPort)
			fmt.Printf("homeassistant:\n  url: %s\n  webhook_id: %s\n", cfg.HomeAssistant.URL, cfg.HomeAssistant.WebhookID)
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
