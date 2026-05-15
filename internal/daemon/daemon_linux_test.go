//go:build linux

package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestDaemon_FinalPush(t *testing.T) {
	port := getFreePort(t)
	token := "token"

	var wg sync.WaitGroup
	wg.Add(3) // 1 registration + 1 initial update + 1 final update

	d := New(Config{
		Port:              port,
		APIKey:            token,
		ReportBootOptions: true,
	}, Metadata{}, func(ctx context.Context, tok string) error {
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
		t.Fatal(err)
	}

	cancel()
	<-done

	wg.Wait()
}

func TestDaemon_FinalPush_UpdateError(t *testing.T) {
	port := getFreePort(t)
	d := New(Config{
		Port:              port,
		APIKey:            "token",
		ReportBootOptions: true,
	}, Metadata{}, nil, func(ctx context.Context) error {
		return errors.New("final push fail")
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		t.Fatal(err)
	}

	cancel()
	<-done
}
