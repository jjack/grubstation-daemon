package main

import (
	"github.com/charmbracelet/huh"
	"github.com/jjack/remote-boot-agent/internal/config"
	hass "github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/jjack/remote-boot-agent/internal/system"
)

func GenerateConfigForm(
	hostname string,
	hassURL string,
	interfaceOptions []system.InterfaceInfo,
) (cfg *config.Config, err error) {
	macAddress := ""
	finalHassURL := hassURL
	webhookID := ""
	finalHostname := hostname

	var ifaceOpts []huh.Option[string]
	for _, opt := range interfaceOptions {
		ifaceOpts = append(ifaceOpts, huh.NewOption(opt.Label, opt.Value))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Hostname").
				Description("This is how Home Assistant will refer to your machine.\nPress enter to accept or enter a custom hostname").
				Placeholder("my-computer").
				Value(&finalHostname).
				Validate(func(v string) error {
					return config.ValidateHostname(v)
				}),

			huh.NewSelect[string]().
				Title("WOL Interface").
				Options(ifaceOpts...).
				Value(&macAddress).
				Validate(func(v string) error {
					return config.ValidateMACAddress(v)
				}),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Home Assistant URL").
				Description("Press enter to accept (if found) or enter a custom Home Assistant URL").
				Placeholder(hassURL).
				Value(&finalHassURL).
				Validate(func(v string) error {
					return hass.ValidateURL(v)
				}),
			huh.NewInput().
				Title("Home Assistant Webhook ID").
				Placeholder("").
				Value(&webhookID).
				Validate(func(v string) error {
					return hass.ValidateWebhookID(v)
				}),
		),
	)

	err = form.Run()
	if err != nil {
		return nil, err
	}

	cfg = &config.Config{
		Host: config.HostConfig{
			MACAddress: macAddress,
			Hostname:   finalHostname,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       finalHassURL,
			WebhookID: webhookID,
		},
	}
	return cfg, nil
}
