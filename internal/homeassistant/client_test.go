package homeassistant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Push(t *testing.T) {
	var receivedPayload PushPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "api/webhook/test-webhook") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-webhook", nil)
	payload := PushPayload{
		MACAddress:  "aa:bb:cc:dd",
		Hostname:    "test-host",
		Bootloader:  "grub",
		BootOptions: []string{"Ubuntu", "Windows"},
	}

	err := client.Push(context.Background(), payload)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if receivedPayload.MACAddress != "aa:bb:cc:dd" {
		t.Errorf("expected MAC aa:bb:cc:dd, got %s", receivedPayload.MACAddress)
	}
	if len(receivedPayload.BootOptions) != 2 {
		t.Errorf("expected 2 OSs, got %d", len(receivedPayload.BootOptions))
	}
}

func TestClient_Push_InvalidURL(t *testing.T) {
	client := NewClient(":\x00invalid%url", "test", nil)
	err := client.Push(context.Background(), PushPayload{})
	if err == nil {
		t.Fatal("expected error on invalid URL, got nil")
	}
}

func TestClient_Push_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-webhook", nil)
	err := client.Push(context.Background(), PushPayload{})
	if err == nil {
		t.Fatal("expected error on server 500, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_View(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "api/remote_boot_manager/aa:bb") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Ubuntu"))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-webhook", nil)
	bootOption, err := client.View(context.Background(), "grub", "aa:bb")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if bootOption != "Ubuntu" {
		t.Errorf("expected Ubuntu, got %s", bootOption)
	}
}

func TestClient_View_InvalidURL(t *testing.T) {
	client := NewClient(":\x00invalid%url", "test", nil)
	_, err := client.View(context.Background(), "grub", "aa:bb")
	if err == nil {
		t.Fatal("expected error on invalid URL, got nil")
	}
}

func TestClient_View_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-webhook", nil)
	_, err := client.View(context.Background(), "grub", "aa:bb")
	if err == nil {
		t.Fatal("expected error on server 404, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// This tests HTTP Client errors in Do() for Push
func TestClient_Push_HttpClientError(t *testing.T) {
	// Create client with invalid base url matching protocol scheme error
	client := NewClient("http://127.0.0.1:0", "test", nil)
	err := client.Push(context.Background(), PushPayload{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// This tests HTTP Client errors in Do() for View
func TestClient_View_HttpClientError(t *testing.T) {
	// Create client with invalid base url matching protocol scheme error
	client := NewClient("http://127.0.0.1:0", "test", nil)
	_, err := client.View(context.Background(), "grub", "my-mac")
	if err == nil {
		t.Fatal("expected error")
	}
}
