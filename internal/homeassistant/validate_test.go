package homeassistant

import (
	"strings"
	"testing"
)

func TestValidateWebhookID(t *testing.T) {
	tests := []struct {
		name      string
		webhookID string
		wantErr   bool
	}{
		{"valid", "my_webhook_123", false},
		{"empty", "", true},
		{"invalid characters", "my-webhook-123", true},
		{"too long", strings.Repeat("a", 256), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateWebhookID(tt.webhookID); (err != nil) != tt.wantErr {
				t.Errorf("ValidateWebhookID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		hassURL string
		wantErr bool
	}{
		{"valid http", "http://homeassistant.local:8123", false},
		{"valid https", "https://homeassistant.local:8123", false},
		{"empty", "", true},
		{"invalid format", "not-a-url", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateURL(tt.hassURL); (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
