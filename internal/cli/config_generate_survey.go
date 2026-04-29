package cli

import (
	"fmt"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/system"
)

func surveyValidator(valFunc func(string) error) survey.Validator {
	return func(val interface{}) error {
		if str, ok := val.(string); ok {
			return valFunc(str)
		}
		return nil
	}
}

var (
	surveyAskOne                = survey.AskOne
	systemGetBroadcastAddresses = system.GetBroadcastAddresses
)

type GenerateFormOptions struct {
	DiscoverHomeAssistant func() (string, error)
	DetectHostname        func() (string, error)
	GetInterfaces         func() ([]system.InterfaceInfo, error)
	BootloaderOptions     []string
	DefaultBootloader     string
	DefaultBootloaderPath string
	InitSystemOptions     []string
	DefaultInitSystem     string
}

func GenerateConfigForm(opts GenerateFormOptions) (cfg *config.Config, err error) {
	hostname, err := opts.DetectHostname()
	if err != nil {
		return nil, err
	}

	var finalHostname string
	err = surveyAskOne(&survey.Input{
		Message: "Hostname (how Home Assistant will refer to your machine):",
		Default: hostname,
	}, &finalHostname, survey.WithValidator(surveyValidator(config.ValidateHostname)))
	if err != nil {
		return nil, err
	}

	interfaceOptions, err := opts.GetInterfaces()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	macAddress := ifaceMap[selectedIfaceLabel]
	if err := config.ValidateMACAddress(macAddress); err != nil {
		return nil, err
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
				return nil, err
			}
		}
	}

	var finalBroadcast string
	err = surveyAskOne(&survey.Input{
		Message: "WOL Broadcast Address:",
		Default: chosenBroadcast,
	}, &finalBroadcast)
	if err != nil {
		return nil, err
	}

	var wolPortStr string
	err = surveyAskOne(&survey.Input{
		Message: "Wake-on-LAN Port (leave blank for default):",
	}, &wolPortStr, survey.WithValidator(surveyValidator(config.ValidateBroadcastPort)))
	if err != nil {
		return nil, err
	}

	var blName string
	err = surveyAskOne(&survey.Select{
		Message: "Bootloader:",
		Options: opts.BootloaderOptions,
		Default: opts.DefaultBootloader,
	}, &blName)
	if err != nil {
		return nil, err
	}

	var blPath string
	err = surveyAskOne(&survey.Input{
		Message: "Bootloader Config Path:",
		Default: opts.DefaultBootloaderPath,
	}, &blPath)
	if err != nil {
		return nil, err
	}

	var initSysName string
	err = surveyAskOne(&survey.Select{
		Message: "Init System:",
		Options: opts.InitSystemOptions,
		Default: opts.DefaultInitSystem,
	}, &initSysName)
	if err != nil {
		return nil, err
	}

	fmt.Println("\nScanning network for Home Assistant...")
	hassURL, _ := opts.DiscoverHomeAssistant()

	var finalHassURL string
	err = surveyAskOne(&survey.Input{
		Message: "Home Assistant URL:",
		Default: hassURL,
	}, &finalHassURL, survey.WithValidator(surveyValidator(config.ValidateURL)))
	if err != nil {
		return nil, err
	}

	var webhookID string
	err = surveyAskOne(&survey.Input{
		Message: "Home Assistant Webhook ID:",
	}, &webhookID, survey.WithValidator(surveyValidator(config.ValidateWebhookID)))
	if err != nil {
		return nil, err
	}

	wolPort, _ := strconv.Atoi(wolPortStr)
	if wolPort == 0 {
		wolPort = 9
	}

	cfg = &config.Config{
		Host: config.HostConfig{
			MACAddress:       macAddress,
			Hostname:         finalHostname,
			BroadcastAddress: finalBroadcast,
			BroadcastPort:    wolPort,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       finalHassURL,
			WebhookID: webhookID,
		},
		Bootloader: config.BootloaderConfig{
			Name:       blName,
			ConfigPath: blPath,
		},
		InitSystem: config.InitSystemConfig{
			Name: initSysName,
		},
	}
	return cfg, nil
}
