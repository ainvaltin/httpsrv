package httpsrv

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
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
			expectError(t, err, errUnassignedAddr)
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
			expectError(t, err, errUnassignedHandler)
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
			expectError(t, err, `http server exited with error: open foo.bar: no such file or directory`)
		}
	})

	listenerAndGetFunc := func(t *testing.T) (net.Listener, func(path string) (*http.Response, error)) {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}

		c := http.Client{Timeout: 3 * time.Second}
		f := func(path string) (*http.Response, error) {
			return c.Get(fmt.Sprintf("http://%s/%s", ln.Addr().String(), path))
		}

		return ln, f
	}

	t.Run("shutdown timeout is respected", func(t *testing.T) {
		ln, doGet := listenerAndGetFunc(t)
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
			)
		}()

		// make request to the server which causes it to exit
		cliErr := make(chan error, 1)
		go func() {
			defer close(cliErr)
			if rsp, err := doGet(""); err != nil {
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
			expectError(t, err, context.Canceled)
			// request exceeded shutdown timeout, server should log error
			expectError(t, err, context.DeadlineExceeded)
			expectError(t, err, `stopping http server: context deadline exceeded`)
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
	})

	queryServer := func(doGet func(path string) (*http.Response, error), path string) error {
		cliErr := make(chan error, 1)
		go func() {
			defer close(cliErr)
			if rsp, err := doGet(path); err != nil {
				cliErr <- err
			} else if rsp == nil {
				cliErr <- fmt.Errorf("unexpectedly response is nil")
			} else {
				cliErr <- fmt.Errorf("got response from server: %v", rsp.Status)
			}
		}()

		select {
		case <-time.After(3 * time.Second):
			return fmt.Errorf("client request didn't return within timeout")
		case err := <-cliErr:
			return err
		}
	}

	t.Run("no panic handler, service ignores panic", func(t *testing.T) {
		ln, doGet := listenerAndGetFunc(t)
		defer ln.Close()

		logBuf := &strings.Builder{}

		ctx, cancel := context.WithCancel(context.Background())
		srvErr := make(chan error, 1)
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/panic", func(w http.ResponseWriter, req *http.Request) { panic("oh-my-foobar") })
			mux.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) { fmt.Fprintf(w, "hello, world") })
			srvErr <- Run(ctx,
				http.Server{
					WriteTimeout: 5 * time.Second,
					Handler:      mux,
					ErrorLog:     log.New(logBuf, "", log.LstdFlags),
				},
				ShutdownTimeout(time.Second),
				Listener(ln),
			)
		}()

		err := queryServer(doGet, "panic")
		expectError(t, err, fmt.Sprintf("Get \"http://%s/panic\": EOF", ln.Addr().String()))
		// server should be up and able to respond
		err = queryServer(doGet, "hello")
		expectError(t, err, "got response from server: 200 OK")

		// stop the server
		cancel()
		select {
		case <-time.After(3 * time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-srvErr:
			// we stopped the server by cancelling the context so that's the error we expect
			expectError(t, err, context.Canceled)
		}

		// the error message logged contains connection specific port so when checking for the
		// expected message (http: panic serving 127.0.0.1:32854: oh-my-foobar) we ignore the port
		if s := logBuf.String(); !strings.Contains(s, "http: panic serving 127.0.0.1:") || !strings.Contains(s, ": oh-my-foobar") {
			t.Error("the log doesn't contain the expected error\n", s)
		}
	})

	t.Run("using panic handler, http.ErrAbortHandler doesn't stop the server", func(t *testing.T) {
		ln, doGet := listenerAndGetFunc(t)
		defer ln.Close()

		logBuf := &strings.Builder{}

		ctx, cancel := context.WithCancel(context.Background())
		srvErr := make(chan error, 1)
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/panic", func(w http.ResponseWriter, req *http.Request) { panic(http.ErrAbortHandler) })
			mux.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) { fmt.Fprintf(w, "hello, world") })
			srvErr <- Run(ctx,
				http.Server{
					WriteTimeout: 5 * time.Second,
					Handler:      mux,
					ErrorLog:     log.New(logBuf, "", log.LstdFlags),
				},
				ShutdownOnPanic(),
				ShutdownTimeout(time.Second),
				Listener(ln),
			)
		}()

		err := queryServer(doGet, "panic")
		expectError(t, err, "got response from server: 200 OK")
		err = queryServer(doGet, "hello")
		expectError(t, err, "got response from server: 200 OK")

		// stop the server
		cancel()
		select {
		case <-time.After(3 * time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-srvErr:
			// we stopped the server by cancelling the context so that's the error we expect
			expectError(t, err, context.Canceled)
		}

		// there should be nothing in the errorlog
		if s := logBuf.String(); s != "" {
			t.Error("unexpectedly there is something in the error log:\n", s)
		}
	})

	t.Run("using panic handler, random panic stops the server", func(t *testing.T) {
		ln, doGet := listenerAndGetFunc(t)
		defer ln.Close()

		logBuf := &strings.Builder{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		srvErr := make(chan error, 1)
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/panic", func(w http.ResponseWriter, req *http.Request) { panic("foobar") })
			mux.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) { fmt.Fprintf(w, "hello, world") })
			srvErr <- Run(ctx,
				http.Server{
					WriteTimeout: 5 * time.Second,
					Handler:      mux,
					ErrorLog:     log.New(logBuf, "", log.LstdFlags),
				},
				ShutdownOnPanic(),
				ShutdownTimeout(time.Second),
				Listener(ln),
			)
		}()

		err := queryServer(doGet, "panic")
		expectError(t, err, fmt.Sprintf("Get \"http://%s/panic\": EOF", ln.Addr().String()))

		// we expect that panic triggered by the request caused server to stop
		select {
		case <-time.After(3 * time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-srvErr:
			expectError(t, err, "unhandled panic: foobar")
		}

		if s := logBuf.String(); s != "" {
			t.Error("unexpectedly there is something in the error log:\n", s)
		}
	})
}

func Test_runServer(t *testing.T) {
	t.Parallel()

	t.Run("failure to start the server", func(t *testing.T) {
		stopCalled := false
		err := runServer(context.Background(),
			// the start func should block until stop signal is sent, we return error immediately
			func() error { return fmt.Errorf("failed to start") },
			func() error { stopCalled = true; return nil },
			nil,
		)
		expectError(t, err, "http server exited with error: failed to start")

		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})

	t.Run("start func returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stopCalled := false
		expErr := fmt.Errorf("error from start")

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				// server starts but after stop signal is sent it returns error
				func() error { <-ctx.Done(); return expErr },
				func() error { stopCalled = true; return nil },
				nil,
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			// we stopped the server by cancelling the context so that's the error we expect
			expectError(t, err, context.Canceled)
			// "http server exited with error: error from start"
			expectError(t, err, expErr)
		}

		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})

	t.Run("stop func returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stopCalled := false
		expErr := fmt.Errorf("error from stop")

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				func() error { <-ctx.Done(); return http.ErrServerClosed },
				func() error { stopCalled = true; return expErr },
				nil,
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			expectError(t, err, context.Canceled)
			// "stopping http server: error from stop"
			expectError(t, err, expErr)
		}

		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})

	t.Run("both start and stop func return error on shutdown", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		startErr := fmt.Errorf("error from start")
		stopErr := fmt.Errorf("error from stop")

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				func() error { <-ctx.Done(); return startErr },
				func() error { return stopErr },
				nil,
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			expectError(t, err, context.Canceled)
			// checking for these errors also checks that both start and stop func were called
			// "http server exited with error: error from start"
			expectError(t, err, startErr)
			// "stopping http server: error from stop"
			expectError(t, err, startErr)
		}
	})

	t.Run("shutdown signal sent to chan", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stopCalled := false

		done := make(chan error, 1)
		shutdownCh := make(chan error)
		go func() {
			done <- runServer(ctx,
				// http.ErrServerClosed is not reported as this is "normal case"
				func() error { <-ctx.Done(); return http.ErrServerClosed },
				func() error { stopCalled = true; return nil },
				shutdownCh,
			)
		}()

		sdErr := fmt.Errorf("shutdown signal in chan")
		shutdownCh <- sdErr
		cancel() // so that startFunc returns

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			expectError(t, err, sdErr)
		}

		if stopCalled {
			t.Error("unexpectedly the stop func was called")
		}
	})

	t.Run("no errors to log", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stopCalled := false

		done := make(chan error, 1)
		go func() {
			done <- runServer(ctx,
				// http.ErrServerClosed is not reported as this is "normal exit error"
				func() error { <-ctx.Done(); return http.ErrServerClosed },
				func() error { stopCalled = true; return nil },
				make(chan error),
			)
		}()

		cancel()

		select {
		case <-time.After(time.Second):
			t.Error("runServer didn't return within timeout")
		case err := <-done:
			expectError(t, err, context.Canceled)
		}

		if !stopCalled {
			t.Error("unexpectedly the stop func hasn't been called")
		}
	})
}

func expectError(t *testing.T, err error, expect any) {
	t.Helper()

	if err == nil {
		t.Errorf("expected error\n%v\ngot nil", expect)
		return
	}

	switch v := expect.(type) {
	case error:
		if !errors.Is(err, v) {
			t.Errorf("expected error\n%v\ngot\n%v", expect, err)
		}
	case string:
		errs := []error{err}
		if ue, ok := err.(interface{ Unwrap() []error }); ok {
			errs = ue.Unwrap()
		}
		for _, e := range errs {
			if e.Error() == v {
				return
			}
		}
		t.Errorf("expected error\n%v\ngot\n%s", expect, err)
	default:
		t.Errorf("unexpected type %T for comparison with %v", expect, err)
	}
}
