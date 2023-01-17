package httpsrv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func Test_Run(t *testing.T) {
	t.Parallel()

	t.Run("address to listen to is not assigned", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- Run(ctx, http.Server{Handler: http.NotFoundHandler()})
		}()

		select {
		case <-time.After(time.Second):
			t.Error("Run didn't return within timeout")
		case err := <-done:
			// we expect the server not to start because the Addr is not set
			if err == nil {
				t.Error("unexpectedly got nil error")
			} else if !errors.Is(err, errUnassignedAddr) {
				t.Errorf("unexpected error: %v", err)
			}
		}
	})

	t.Run("handlers to serve is not assigned", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- Run(ctx, http.Server{Addr: "127.0.0.1:0"})
		}()

		select {
		case <-time.After(time.Second):
			t.Error("Run didn't return within timeout")
		case err := <-done:
			// we expect the server not to start because the Handler is not set
			if err == nil {
				t.Error("unexpectedly got nil error")
			} else if !errors.Is(err, errUnassignedHandler) {
				t.Errorf("unexpected error: %v", err)
			}
		}
	})

	t.Run("certificate file doesn't exist", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- Run(ctx, http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler()}, TLS("foo.bar", "bar.foo"))
		}()

		select {
		case <-time.After(time.Second):
			t.Error("Run didn't return within timeout")
		case err := <-done:
			// we expect the server not to start because the file(s) do not exist
			if err == nil {
				t.Error("unexpectedly got nil error")
			} else if err.Error() != `http server exited with error: open foo.bar: no such file or directory` {
				t.Errorf("unexpected error: %v", err)
			}
		}
	})

	listenerAndGetFunc := func(t *testing.T) (net.Listener, func() (*http.Response, error)) {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}

		c := http.Client{Timeout: 3 * time.Second}
		f := func() (*http.Response, error) {
			return c.Get(fmt.Sprintf("http://%s", ln.Addr().String()))
		}

		return ln, f
	}

	logErrFunc := func() (func(string, ...any), *bytes.Buffer) {
		buf := bytes.NewBuffer(nil)
		return func(format string, a ...any) { fmt.Fprintln(buf, fmt.Sprintf(format, a...)) }, buf
	}

	t.Run("shutdown timeout is respected", func(t *testing.T) {
		logF, logBuf := logErrFunc()
		ln, getf := listenerAndGetFunc(t)
		defer ln.Close()

		ctx, cancel := context.WithCancel(context.Background())
		srvErr := make(chan error, 1)
		go func() {
			srvErr <- Run(ctx,
				http.Server{
					WriteTimeout: 5 * time.Second,
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// stop the server but keep the handler busy longer than the shutdown timeout
						cancel()
						time.Sleep(2000 * time.Millisecond)
						w.Write([]byte("something"))
					}),
				},
				ShutdownTimeout(time.Second),
				Listener(ln),
				LogError(logF),
			)
		}()

		// make request to the server which causes it to exit
		cliErr := make(chan error, 1)
		go func() {
			defer close(cliErr)
			if rsp, err := getf(); err != nil {
				cliErr <- err
			} else if rsp == nil {
				cliErr <- fmt.Errorf("unexpectedly response is nil")
			} else {
				// response should indicate that the connection will be closed
				if rsp.StatusCode != 200 || !rsp.Close {
					cliErr <- fmt.Errorf("got unexpected response: %+v", rsp)
				}
			}
		}()

		select {
		case <-time.After(3 * time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-srvErr:
			// we stopped the server by cancelling the context so that's the error we expect
			if err != context.Canceled {
				t.Errorf("unexpected server error: %v", err)
			}
		}

		select {
		case <-time.After(3 * time.Second):
			t.Error("client request didn't return within timeout")
		case err := <-cliErr:
			// server should die while the request is still in flight but it seems that the request
			// still gets served (given server's WriteTimeout is not hit), ie no error expected here
			if err != nil {
				t.Errorf("unexpected client error: %v", err)
			}
		}

		// request exceeded shutdown timeout, server should log error
		if s := logBuf.String(); s != "stopping http server: context deadline exceeded\n" {
			t.Errorf("unexpected content in error log:\n%s\n", s)
		}
	})
}

func Test_runServer(t *testing.T) {
	t.Parallel()

	logErrFunc := func() (func(string, ...any), *bytes.Buffer) {
		buf := bytes.NewBuffer(nil)
		return func(format string, a ...any) { fmt.Fprintln(buf, fmt.Sprintf(format, a...)) }, buf
	}

	t.Run("failure to start the server", func(t *testing.T) {
		logF, buf := logErrFunc()
		stopCalled := false
		err := runServer(context.Background(),
			// the start func should block until stop signal is sent, we return error immediately
			func() error { return fmt.Errorf("failed to start") },
			func() error { stopCalled = true; return nil },
			logF,
		)
		if err == nil {
			t.Error("expected non-nil error")
		} else if err.Error() != "http server exited with error: failed to start" {
			t.Errorf("unexpected error: %v", err)
		}

		if s := buf.String(); s != "" {
			t.Errorf("unexpected content in error log:\n%s\n", s)
		}

		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})

	t.Run("start func returns error", func(t *testing.T) {
		logF, buf := logErrFunc()
		ctx, cancel := context.WithCancel(context.Background())
		stopCalled := false

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				// server starts but after stop signal is sent it returns error
				func() error { <-ctx.Done(); return fmt.Errorf("error from start") },
				func() error { stopCalled = true; return nil },
				logF,
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			// we stopped the server by cancelling the context so that's the error we expect
			if err != context.Canceled {
				t.Errorf("unexpected error: %v", err)
			}
		}

		if s := buf.String(); s != "http server exited with error: error from start\n" {
			t.Errorf("unexpected content in error log:\n%s\n", s)
		}
		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})

	t.Run("stop func returns error", func(t *testing.T) {
		logF, buf := logErrFunc()
		ctx, cancel := context.WithCancel(context.Background())
		stopCalled := false

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				func() error { <-ctx.Done(); return http.ErrServerClosed },
				func() error { stopCalled = true; return fmt.Errorf("error from stop") },
				logF,
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			if err != context.Canceled {
				t.Errorf("unexpected error: %v", err)
			}
		}

		if s := buf.String(); s != "stopping http server: error from stop\n" {
			t.Errorf("unexpected content in error log:\n%s\n", s)
		}
		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})

	t.Run("both start and stop func return error on shutdown", func(t *testing.T) {
		logF, buf := logErrFunc()
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				func() error { <-ctx.Done(); return fmt.Errorf("error from start") },
				func() error { return fmt.Errorf("error from stop") },
				logF,
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			if err != context.Canceled {
				t.Errorf("unexpected error: %v", err)
			}
		}

		// checking the log content also checks that both start and stop func were called
		if s := buf.String(); s != "stopping http server: error from stop\nhttp server exited with error: error from start\n" {
			t.Errorf("unexpected content in error log:\n%s\n", s)
		}
	})

	t.Run("no errors to log", func(t *testing.T) {
		logF, buf := logErrFunc()
		ctx, cancel := context.WithCancel(context.Background())
		stopCalled := false

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				// http.ErrServerClosed is not reported as this is "normal case"
				func() error { <-ctx.Done(); return http.ErrServerClosed },
				func() error { stopCalled = true; return nil },
				logF,
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			if err != context.Canceled {
				t.Errorf("unexpected error: %v", err)
			}
		}

		if s := buf.String(); s != "" {
			t.Errorf("unexpected content in error log:\n%s\n", s)
		}
		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})
}
