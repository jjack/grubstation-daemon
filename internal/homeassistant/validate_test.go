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
		{"valid", "0b1eb94bad526a8f845df65e19bf1dfb165a17917f77d981acb4f5774dbb7953", false},
		{"empty", "", true},
		{"invalid characters", "my!webhook", true},
		{"too long", strings.Repeat("a", 256), true},
		{"too short", strings.Repeat("a", 32), true},
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
		{"invalid https", "https://homeassistant.local:8123", true},
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
