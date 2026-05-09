package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func waitForServer(port int) error {
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			conn.Close()
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
	d := New(Config{ListenPort: port}, nil)

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var status map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status["status"] != "ok" {
		t.Errorf("expected status ok, got %s", status["status"])
	}
	if _, ok := status["version"]; ok {
		t.Errorf("expected version to be removed from status endpoint")
	}
	if _, ok := status["os"]; ok {
		t.Errorf("expected os to be removed from status endpoint")
	}

	cancel()
	<-done
}

func TestGenerateToken(t *testing.T) {
	t1, err := generateToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	t2, err := generateToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if t1 == t2 {
		t.Errorf("expected unique tokens, got same: %s", t1)
	}
	if len(t1) != 32 { // 16 bytes hex encoded
		t.Errorf("expected 32 chars, got %d", len(t1))
	}
}

func TestDaemon_Shutdown_Unauthorized(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	token := "secret-token"
	d := New(Config{
		ListenPort: port,
		APIKey:     token,
	}, nil)

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
	defer resp.Body.Close()

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
	d := New(Config{ListenPort: port}, nil)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	resp, err := getTestClient().Get(fmt.Sprintf("http://localhost:%d/shutdown", port))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

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
	d := New(Config{ListenPort: port, APIKey: token}, nil)

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
	defer resp.Body.Close()

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
	handshakeDone := make(chan bool, 1)

	d := New(Config{
		ListenPort: port,
		APIKey:     token,
	}, func(ctx context.Context, tok string) error {
		if tok == token {
			handshakeDone <- true
		}
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	select {
	case <-handshakeDone:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("handshake not called within timeout")
	}

	cancel()
	<-done
}

func TestDaemon_Run_DynamicToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	var capturedToken string
	handshakeDone := make(chan bool, 1)

	// No APIKey provided, should generate one
	d := New(Config{ListenPort: port}, func(ctx context.Context, tok string) error {
		capturedToken = tok
		handshakeDone <- true
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	select {
	case <-handshakeDone:
		if len(capturedToken) != 32 {
			t.Errorf("expected 32-char hex token, got %s", capturedToken)
		}
	case <-time.After(2 * time.Second):
		t.Error("handshake not called")
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
	pushCalled := make(chan bool, 10)
	d := New(Config{
		ListenPort:        port,
		APIKey:            token,
		ReportBootOptions: true,
		ShutdownDelay:     time.Millisecond,
	}, func(ctx context.Context, tok string) error {
		pushCalled <- true
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
	resp.Body.Close()

	select {
	case <-cmdCalled:
		// success
	case <-time.After(2 * time.Second):
		t.Error("shutdown command not called")
	}

	// Drain any remaining pushes to avoid blocking the daemon's finalization
	go func() {
		for range pushCalled {
		}
	}()

	cancel()
	<-done
	close(pushCalled)
}

func TestDaemon_Run_HandshakeRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	callCount := 0
	handshakeDone := make(chan bool, 1)

	d := New(Config{
		ListenPort:    port,
		RetryInterval: 10 * time.Millisecond,
	}, func(ctx context.Context, tok string) error {
		callCount++
		if callCount == 1 {
			return errors.New("fail")
		}
		handshakeDone <- true
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	select {
	case <-handshakeDone:
		if callCount < 2 {
			t.Errorf("expected retry, callCount was %d", callCount)
		}
	case <-time.After(1 * time.Second):
		t.Error("handshake retry did not succeed in time")
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
	pushCalled := make(chan bool, 10)
	d := New(Config{
		ListenPort:        port,
		APIKey:            token,
		ReportBootOptions: true,
	}, func(ctx context.Context, tok string) error {
		pushCalled <- true
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	// Start a goroutine to drain the push channel so it doesn't block the daemon's finalization
	pushCount := 0
	doneDraining := make(chan bool)
	go func() {
		for range pushCalled {
			pushCount++
		}
		doneDraining <- true
	}()

	cancel() // Stop daemon
	<-done
	close(pushCalled)
	<-doneDraining

	if pushCount < 2 {
		t.Errorf("expected at least 2 pushes (handshake + final), got %d", pushCount)
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

	d := New(Config{ShutdownDelay: time.Millisecond}, nil)
	d.performOSShutdown()

	select {
	case <-exitCalled:
		// success
	case <-time.After(2 * time.Second):
		t.Error("os.Exit(1) not called on shutdown error")
	}
}
