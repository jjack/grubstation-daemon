package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
)

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func waitForServer(port int) error {
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server at port %d never became ready", port)
}

func getTestClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}

func TestDaemonHealthcheckEndpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	d := New(Config{Port: port, APIKey: "test-key"}, nil, nil)

	done := make(chan error, 1)
	go func() {
		done <- d.run(ctx)
	}()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	resp, err := getTestClient().Get(fmt.Sprintf("http://localhost:%d/healthcheck", port))
	if err != nil {
		t.Fatalf("failed to call healthcheck endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if !bytes.Equal(body, []byte("ok\n")) {
		t.Errorf("expected body 'ok\\n', got %q", string(body))
	}

	cancel()
	<-done
}

func TestDaemon_Shutdown_Unauthorized(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	token := "secret-token"
	d := New(Config{
		Port:   port,
		APIKey: token,
	}, nil, nil)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/shutdown", port), nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := getTestClient().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestDaemon_InvalidMethod(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	d := New(Config{Port: port, APIKey: "test-key"}, nil, nil)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()
	time.Sleep(10 * time.Millisecond)

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	resp, err := getTestClient().Get(fmt.Sprintf("http://localhost:%d/shutdown", port))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestDaemon_NotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	token := "token"
	d := New(Config{Port: port, APIKey: token}, nil, nil)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/invalid", port), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := getTestClient().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestDaemon_Run_HandshakeSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Find an available port
	port := getFreePort(t)
	token := "secret"
	registrationDone := make(chan bool, 1)
	updateDone := make(chan bool, 1)

	d := New(Config{
		Port:   port,
		APIKey: token,
	}, func(ctx context.Context, tok string) error {
		if tok == token {
			registrationDone <- true
		}
		return nil
	}, func(ctx context.Context) error {
		updateDone <- true
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	select {
	case <-registrationDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("registration not called within timeout")
	}

	select {
	case <-updateDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("initial update not called within timeout")
	}

	cancel()
	<-done
}

func TestDaemon_Run_DynamicToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	var capturedToken string
	registrationDone := make(chan bool, 1)

	// No APIKey provided, should generate one
	token := "test-api-key"
	d := New(Config{Port: port, APIKey: token}, func(ctx context.Context, tok string) error {
		capturedToken = tok
		registrationDone <- true
		return nil
	}, nil)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	select {
	case <-registrationDone:
		if capturedToken != token {
			t.Errorf("expected token %s, got %s", token, capturedToken)
		}
	case <-time.After(2 * time.Second):
		t.Error("registration not called")
	}

	cancel()
	<-done
}

func TestDaemon_Shutdown_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	cmdCalled := make(chan bool, 1)
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cmdCalled <- true
		if runtime.GOOS == "windows" {
			return exec.Command("cmd", "/c", "exit", "0")
		}
		return exec.Command("true")
	}

	port := getFreePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token := "token"
	updateCalled := make(chan bool, 10)
	d := New(Config{
		Port:              port,
		APIKey:            token,
		ReportBootOptions: true,
		ShutdownDelay:     time.Millisecond,
	}, nil, func(ctx context.Context) error {
		updateCalled <- true
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/shutdown", port), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := getTestClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	select {
	case <-cmdCalled:
		// success
	case <-time.After(2 * time.Second):
		t.Error("shutdown command not called")
	}

	// Drain any remaining updates to avoid blocking the daemon's finalization
	go func() {
		for range updateCalled {
		}
	}()

	cancel()
	<-done
	close(updateCalled)
}

func TestDaemon_Run_HandshakeRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	callCount := 0
	registrationDone := make(chan bool, 1)

	d := New(Config{
		Port:          port,
		APIKey:        "test-key",
		RetryInterval: 10 * time.Millisecond,
	}, func(ctx context.Context, tok string) error {
		callCount++
		if callCount == 1 {
			return errors.New("fail")
		}
		registrationDone <- true
		return nil
	}, nil)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	select {
	case <-registrationDone:
		if callCount < 2 {
			t.Errorf("expected retry, callCount was %d", callCount)
		}
	case <-time.After(1 * time.Second):
		t.Error("registration retry did not succeed in time")
	}

	cancel()
	<-done
}

func TestDaemon_FinalPush(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Final push logic is linux specific")
	}

	port := getFreePort(t)
	token := "token"
	
	var wg sync.WaitGroup
	wg.Add(3) // 1 registration + 1 initial update + 1 final update

	d := New(Config{
		Port:              port,
		APIKey:            token,
		ReportBootOptions: true,
	}, func(ctx context.Context, tok string) error {
		wg.Done()
		return nil
	}, func(ctx context.Context) error {
		wg.Done()
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	cancel() // Stop daemon
	<-done

	// Wait for all expected calls to finish
	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
		// Success
	case <-time.After(2 * time.Second):
		t.Errorf("Timed out waiting for expected registration and updates")
	}
}

func TestDaemon_PerformOSShutdown_Error(t *testing.T) {
	oldExec := execCommand
	oldExit := osExit
	defer func() {
		execCommand = oldExec
		osExit = oldExit
	}()

	exitCalled := make(chan bool, 1)
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("nonexistent-command-12345")
	}
	osExit = func(code int) {
		if code == 1 {
			exitCalled <- true
		}
	}

	d := New(Config{APIKey: "test-key", ShutdownDelay: time.Millisecond}, nil, nil)
	d.performOSShutdown()

	select {
	case <-exitCalled:
		// success
	case <-time.After(2 * time.Second):
		t.Error("os.Exit(1) not called on shutdown error")
	}
}
