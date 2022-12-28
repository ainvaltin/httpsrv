package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func Test_run(t *testing.T) {
	t.Parallel()

	// creates listener on random free port
	listenerForSrv := func(t *testing.T) net.Listener {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		return ln
	}

	t.Run("cancelling context stops the service", func(t *testing.T) {
		ln := listenerForSrv(t)
		defer ln.Close()

		ctx, cancel := context.WithCancel(context.Background())

		srvErr := make(chan error, 1)
		go func() {
			srvErr <- run(ctx, &srvConf{l: ln})
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Fatal("server didn't stop within timeout")
		case err := <-srvErr:
			if err == nil {
				t.Fatal("unexpectedly run returned nil error")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected %q, got %q", context.Canceled, err)
			}
		}
	})

	t.Run("http server is up", func(t *testing.T) {
		ln := listenerForSrv(t)
		defer ln.Close()

		ctx, cancel := context.WithCancel(context.Background())

		srvErr := make(chan error, 1)
		go func() {
			srvErr <- run(ctx, &srvConf{l: ln})
		}()

		c := &http.Client{Timeout: time.Second}
		rsp, err := c.Get(fmt.Sprintf("http://%s", ln.Addr().String()))
		if err != nil {
			t.Errorf("GET request returned unexpected error: %v", err)
		}
		if rsp == nil {
			t.Error("unexpectedly GET request returned nil response")
		}

		cancel()

		select {
		case <-time.After(time.Second):
			t.Fatal("server didn't stop within timeout")
		case err := <-srvErr:
			if err == nil {
				t.Fatal("unexpectedly run returned nil error")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected %q, got %q", context.Canceled, err)
			}
		}
	})
}
