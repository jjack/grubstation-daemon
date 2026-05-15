//go:build !windows

package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestUnixSocketPush(t *testing.T) {
	oldPath := SocketPath
	newPath := filepath.Join(t.TempDir(), "test.sock")
	SocketPath = newPath
	defer func() { SocketPath = oldPath }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pushed := false
	d := New(Config{}, Metadata{}, nil, func(ctx context.Context) error {
		pushed = true
		return nil
	})

	go d.listenUnixSocket(ctx, "test-token")
	time.Sleep(100 * time.Millisecond) // Wait for the socket server to spin up

	if err := RequestPushViaSocket(ctx); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !pushed {
		t.Errorf("expected push handler to be called")
	}
}

func TestUnixSocket_ListenError(t *testing.T) {
	// We use a path with a non-existent parent directory to guarantee a listen error.
	// Note: using t.TempDir() directly is risky because os.Remove(path) inside
	// listenUnixSocket will delete an empty directory, allowing net.Listen to succeed.
	oldPath := SocketPath
	SocketPath = "/non/existent/path/socket.sock"
	defer func() { SocketPath = oldPath }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := New(Config{}, Metadata{}, nil, nil)

	// Call in a goroutine with a channel to signal completion
	done := make(chan struct{})
	go func() {
		d.listenUnixSocket(ctx, "token")
		close(done)
	}()

	select {
	case <-done:
		// Success: the function returned as expected after failing to listen
	case <-time.After(1 * time.Second):
		t.Error("listenUnixSocket did not return on listen error")
	}
}

func TestUnixSocket_PushHandlerError(t *testing.T) {
	oldPath := SocketPath
	newPath := filepath.Join(t.TempDir(), "test-err.sock")
	SocketPath = newPath
	defer func() { SocketPath = oldPath }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := New(Config{}, Metadata{}, nil, func(ctx context.Context) error {
		return errors.New("push failed")
	})
	go d.listenUnixSocket(ctx, "token")
	time.Sleep(50 * time.Millisecond)

	err := RequestPushViaSocket(ctx)
	if err == nil || !strings.Contains(err.Error(), "push failed") {
		t.Errorf("expected push failed error, got %v", err)
	}
}

func TestUnixSocket_NoPushHandler(t *testing.T) {
	oldPath := SocketPath
	newPath := filepath.Join(t.TempDir(), "test-none.sock")
	SocketPath = newPath
	defer func() { SocketPath = oldPath }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := New(Config{}, Metadata{}, nil, nil)
	go d.listenUnixSocket(ctx, "token")
	time.Sleep(50 * time.Millisecond)

	err := RequestPushViaSocket(ctx)
	if err == nil || !strings.Contains(err.Error(), "UpdateHandler not configured") {
		t.Errorf("expected not configured error, got %v", err)
	}
}

func TestRequestPushViaSocket_NoSocket(t *testing.T) {
	SocketPath = "/tmp/non-existent-socket-12345"
	err := RequestPushViaSocket(context.Background())
	if err == nil {
		t.Error("expected error dialing non-existent socket")
	}
}

func TestDaemon_Run(t *testing.T) {
	d := New(Config{}, Metadata{}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()
	// Wait for Run to start
	time.Sleep(50 * time.Millisecond)

	// Send signal
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "http: Server closed") {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after SIGTERM")
	}
}

func TestUnixSocket_WriteError(t *testing.T) {
	oldPath := SocketPath
	newPath := filepath.Join(t.TempDir(), "test-write-err.sock")
	SocketPath = newPath
	defer func() { SocketPath = oldPath }()

	l, _ := net.Listen("unix", SocketPath)
	defer func() { _ = l.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Close immediately to cause write error on client side or similar
	go func() {
		conn, _ := l.Accept()
		if conn != nil {
			_ = conn.Close()
		}
	}()

	// This is harder to trigger exactly for conn.Write, but we can try to trigger RequestPushViaSocket errors
	// Actually, let's just test RequestPushViaSocket with a closed connection
	err := RequestPushViaSocket(ctx)
	if err == nil {
		t.Error("expected error from RequestPushViaSocket on closed connection")
	}
}

func TestRequestPushViaSocket_InvalidResponse(t *testing.T) {
	oldPath := SocketPath
	newPath := filepath.Join(t.TempDir(), "test-invalid-resp.sock")
	SocketPath = newPath
	defer func() { SocketPath = oldPath }()

	l, _ := net.Listen("unix", SocketPath)
	defer func() { _ = l.Close() }()

	go func() {
		conn, _ := l.Accept()
		if conn != nil {
			_, _ = fmt.Fprintf(conn, "INVALID\n")
			_ = conn.Close()
		}
	}()

	err := RequestPushViaSocket(context.Background())
	if err == nil || !strings.Contains(err.Error(), "daemon returned error: INVALID") {
		t.Errorf("expected invalid response error, got %v", err)
	}
}

func TestRequestPushViaSocket_NoResponse(t *testing.T) {
	oldPath := SocketPath
	newPath := filepath.Join(t.TempDir(), "test-no-resp.sock")
	SocketPath = newPath
	defer func() { SocketPath = oldPath }()

	l, _ := net.Listen("unix", SocketPath)
	defer func() { _ = l.Close() }()

	go func() {
		conn, _ := l.Accept()
		if conn != nil {
			_ = conn.Close()
		}
	}()

	err := RequestPushViaSocket(context.Background())
	if err == nil {
		t.Error("expected error from RequestPushViaSocket on no response")
	}
}
