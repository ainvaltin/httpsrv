package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"
)

func Test_run(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	hc := http.Client{Timeout: 30 * time.Second}

	// getStatus returns the status reported by the /health endpoint.
	// returns 0 when connection is refused, ie the server is not up or is shutting down.
	getStatus := func(ctx context.Context) int {
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/health", ln.Addr().String()), nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		rsp, err := hc.Do(req)
		if err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) {
				return 0
			}
			t.Fatalf("request failed: %v", err)
		}
		return rsp.StatusCode
	}

	waitForStatus := func(ctx context.Context, status int, timeout time.Duration) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		for getStatus(ctx) != status {
			time.Sleep(10 * time.Millisecond)
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		return nil
	}

	cfg := &srvConf{
		srvLn:         ln,
		shutdownDelay: 3 * time.Second,
	}

	srvCtx, srvCtxCancel := context.WithCancel(context.Background())

	runErr := make(chan error, 1)
	go func() {
		runErr <- run(srvCtx, cfg)
	}()

	// wait until server is up and starts to signal good health
	if err := waitForStatus(srvCtx, http.StatusOK, time.Second); err != nil {
		t.Fatal("waiting for the server to become healthy:", err)
	}

	// cancel the server ctx - triggers server shutdown
	srvCtxCancel()

	ctx := context.Background()
	// wait until server starts to signal bad health
	if err := waitForStatus(ctx, http.StatusServiceUnavailable, time.Second); err != nil {
		t.Fatal("waiting for the server to become un-healthy:", err)
	}
	startWaitForShutdown := time.Now()

	// wait until server is not responding anymore
	if err := waitForStatus(ctx, 0, cfg.shutdownDelay+time.Second); err != nil {
		t.Fatal("waiting for the server to become un-available:", err)
	}
	// we starting our clock and server shuting down are not in perfect sync (test is slightly
	// behind) so sometimes the delay comes out as little shorter so round up when comparing
	tt := time.Since(startWaitForShutdown)
	if tt.Round(25*time.Millisecond) < cfg.shutdownDelay {
		t.Errorf("bad health was signalled for a shorter period than expected: %s", tt)
	}
	// allow some overhead on how long the shutdown takes
	if tt.Truncate(50*time.Millisecond) > cfg.shutdownDelay {
		t.Errorf("bad health was signalled longer than expected: %s", tt)
	}

	// server must have exited by now
	select {
	case <-time.After(500 * time.Millisecond):
		t.Fatal("test didn't finish within timeout")
	case err := <-runErr:
		if err == nil {
			t.Fatal("expected non-nil error from run")
		}
		if !errors.Is(err, context.Canceled) {
			t.Error("unexpected error returned by run:", err)
		}
	}
}
