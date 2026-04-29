package cli

import (
	"errors"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/jjack/remote-boot-agent/internal/system"
)

func TestSurveyValidator(t *testing.T) {
	valFunc := func(v string) error {
		if v == "fail" {
			return errors.New("validation failed")
		}
		return nil
	}

	validator := surveyValidator(valFunc)

	if err := validator("fail"); err == nil || err.Error() != "validation failed" {
		t.Errorf("expected validation to fail, got %v", err)
	}
	if err := validator("success"); err != nil {
		t.Errorf("expected validation to succeed, got %v", err)
	}
	if err := validator(123); err != nil {
		t.Errorf("expected non-string to return nil, got %v", err)
	}
}

func buildMockSurveyAskOne(triggerErrorOn string) func(survey.Prompt, interface{}, ...survey.AskOpt) error {
	return func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		var msg string
		switch pt := p.(type) {
		case *survey.Input:
			msg = pt.Message
		case *survey.Select:
			msg = pt.Message
		}

		if triggerErrorOn != "" && msg == triggerErrorOn {
			return errors.New("simulated survey error")
		}

		switch pt := p.(type) {
		case *survey.Input:
			switch pt.Message {
			case "Hostname (how Home Assistant will refer to your machine):":
				*(response.(*string)) = "my-host"
			case "WOL Broadcast Address:":
				*(response.(*string)) = "192.168.1.255"
			case "Wake-on-LAN Port (leave blank for default):":
				*(response.(*string)) = "" // test fallback
			case "Bootloader Config Path:":
				*(response.(*string)) = "/boot/grub/grub.cfg"
			case "Home Assistant URL:":
				*(response.(*string)) = "http://hass.local:8123"
			case "Home Assistant Webhook ID:":
				*(response.(*string)) = "webhook123"
			}
		case *survey.Select:
			switch pt.Message {
			case "Select Physical WOL Interface":
				*(response.(*string)) = "eth0"
			case "Multiple WOL Subnet/Broadcast Addresses were discovered. Please select one:":
				*(response.(*string)) = "192.168.1.255"
			case "Bootloader:":
				*(response.(*string)) = "grub"
			case "Init System:":
				*(response.(*string)) = "systemd"
			}
		}
		return nil
	}
}

func TestGenerateConfigForm_Success(t *testing.T) {
	oldSurveyAskOne := surveyAskOne
	oldSystemGetBroadcastAddresses := systemGetBroadcastAddresses
	defer func() {
		surveyAskOne = oldSurveyAskOne
		systemGetBroadcastAddresses = oldSystemGetBroadcastAddresses
	}()

	surveyAskOne = buildMockSurveyAskOne("")

	systemGetBroadcastAddresses = func(mac string) ([]string, error) {
		return []string{"192.168.1.255", "10.0.0.255"}, nil // Trigger multiple broadcasts path
	}

	opts := GenerateFormOptions{
		DiscoverHomeAssistant: func() (string, error) { return "http://hass.local:8123", nil },
		DetectHostname:        func() (string, error) { return "detected-host", nil },
		GetInterfaces: func() ([]system.InterfaceInfo, error) {
			return []system.InterfaceInfo{
				{Label: "eth0", Value: "00:11:22:33:44:55"},
			}, nil
		},
		BootloaderOptions:     []string{"grub"},
		DefaultBootloader:     "grub",
		DefaultBootloaderPath: "/boot/grub/grub.cfg",
		InitSystemOptions:     []string{"systemd"},
		DefaultInitSystem:     "systemd",
	}

	cfg, err := GenerateConfigForm(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host.Hostname != "my-host" {
		t.Errorf("expected hostname my-host, got %s", cfg.Host.Hostname)
	}
	if cfg.Host.BroadcastAddress != "192.168.1.255" {
		t.Errorf("expected BroadcastAddress 192.168.1.255, got %s", cfg.Host.BroadcastAddress)
	}
	if cfg.Host.BroadcastPort != 9 {
		t.Errorf("expected BroadcastPort 9 (fallback), got %d", cfg.Host.BroadcastPort)
	}
	if cfg.HomeAssistant.URL != "http://hass.local:8123" {
		t.Errorf("expected URL http://hass.local:8123, got %s", cfg.HomeAssistant.URL)
	}
}

func TestGenerateConfigForm_AskOneErrors(t *testing.T) {
	oldSurveyAskOne := surveyAskOne
	oldSystemGetBroadcastAddresses := systemGetBroadcastAddresses
	defer func() {
		surveyAskOne = oldSurveyAskOne
		systemGetBroadcastAddresses = oldSystemGetBroadcastAddresses
	}()

	systemGetBroadcastAddresses = func(mac string) ([]string, error) { return []string{"192.168.1.255"}, nil }

	baseOpts := GenerateFormOptions{
		DiscoverHomeAssistant: func() (string, error) { return "http://hass.local:8123", nil },
		DetectHostname:        func() (string, error) { return "detected-host", nil },
		GetInterfaces: func() ([]system.InterfaceInfo, error) {
			return []system.InterfaceInfo{{Label: "eth0", Value: "00:11:22:33:44:55"}}, nil
		},
	}

	errorSteps := []string{
		"Hostname (how Home Assistant will refer to your machine):",
		"Select Physical WOL Interface",
		"WOL Broadcast Address:",
		"Wake-on-LAN Port (leave blank for default):",
		"Bootloader:",
		"Bootloader Config Path:",
		"Init System:",
		"Home Assistant URL:",
		"Home Assistant Webhook ID:",
	}

	for _, step := range errorSteps {
		t.Run("Error at "+step, func(t *testing.T) {
			surveyAskOne = buildMockSurveyAskOne(step)
			_, err := GenerateConfigForm(baseOpts)
			if err == nil || err.Error() != "simulated survey error" {
				t.Fatalf("expected simulated survey error at step %q, got %v", step, err)
			}
		})
	}

	t.Run("Multiple Subnet Selection Error", func(t *testing.T) {
		surveyAskOne = buildMockSurveyAskOne("Multiple WOL Subnet/Broadcast Addresses were discovered. Please select one:")
		systemGetBroadcastAddresses = func(mac string) ([]string, error) { return []string{"192.168.1.255", "10.0.0.255"}, nil }
		_, err := GenerateConfigForm(baseOpts)
		if err == nil || err.Error() != "simulated survey error" {
			t.Errorf("expected simulated survey error, got %v", err)
		}
	})
}

func TestGenerateConfigForm_OptErrors(t *testing.T) {
	t.Run("Invalid MAC Address", func(t *testing.T) {
		oldSurveyAskOne := surveyAskOne
		surveyAskOne = buildMockSurveyAskOne("")
		defer func() { surveyAskOne = oldSurveyAskOne }()

		opts := GenerateFormOptions{
			DetectHostname: func() (string, error) { return "host", nil },
			GetInterfaces: func() ([]system.InterfaceInfo, error) {
				return []system.InterfaceInfo{{Label: "eth0", Value: "invalid-mac"}}, nil
			},
		}
		_, err := GenerateConfigForm(opts)
		if err == nil {
			t.Errorf("expected mac validation error, got nil")
		}
	})
}
