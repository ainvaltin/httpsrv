package httpsrv

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func Test_serverConf_listener(t *testing.T) {
	t.Parallel()

	t.Run("when listener is assigned it is returned", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer ln.Close()

		cfg := &serverConf{l: ln, srv: &http.Server{}}
		l, err := cfg.listener()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l != ln {
			t.Fatal("unexpectedly different listener was returned")
		}
	})

	t.Run("both Addr and listener are unassigned", func(t *testing.T) {
		// random port is opened when no Addr is provided
		cfg := &serverConf{srv: &http.Server{}}
		l, err := cfg.listener()
		if err != nil {
			t.Error("unexpected error", err)
		}
		if l == nil {
			t.Error("unexpectedly nil listener was returned")
		} else {
			l.Close()
		}
	})

	t.Run("when both Addr and listener are assigned then listener is returned", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer ln.Close()
		addr := ln.Addr().String()

		cfg := &serverConf{l: ln, srv: &http.Server{Addr: "127.0.0.1:0"}}
		l, err := cfg.listener()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l != nil {
			if l != ln {
				l.Close()
				t.Fatal("unexpectedly different listener was returned")
			}
			if s := l.Addr().String(); s != addr {
				t.Errorf("the address of the listener has changed from %q to %q", addr, s)
			}
		}
	})

	t.Run("multiple calls do return the same listener", func(t *testing.T) {
		cfg := &serverConf{srv: &http.Server{Addr: "127.0.0.1:0"}}
		l1, err := cfg.listener()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		addr := l1.Addr().String()

		l2, err := cfg.listener()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l1 != l2 {
			t.Fatal("unexpectedly different listener was returned")
		}
		if s := l2.Addr().String(); s != addr {
			t.Errorf("the address of the listener has changed from %q to %q", addr, s)
		}
	})

	t.Run("try to open the same Addr twice", func(t *testing.T) {
		// first attempt should succeed
		cfg := &serverConf{srv: &http.Server{Addr: "127.0.0.1:0"}}
		l, err := cfg.listener()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if l == nil {
			t.Fatal("unexpectedly nil listener was returned")
		}
		defer l.Close()

		// second attempt with the same addr should fail
		cfg2 := &serverConf{srv: &http.Server{Addr: l.Addr().String()}}
		l2, err := cfg2.listener()
		if err != nil {
			expErrMsg := fmt.Sprintf("failed to create listener on %q: listen tcp %[1]s: bind: address already in use", l.Addr().String())
			if err.Error() != expErrMsg {
				t.Fatalf("unexpected error: %v", err)
			}
		} else {
			t.Error("unexpectedly no error was returned")
		}
		if l2 != nil {
			l2.Close()
			t.Fatal("unexpectedly non-nil listener was returned for the second time too")
		}
	})
}

func Test_serverConf_startFunc(t *testing.T) {
	t.Parallel()

	t.Run("listener call returns error", func(t *testing.T) {
		// to make the listener() method to return error we make it to
		// open listener on already in use port
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer ln.Close()

		cfg := &serverConf{srv: &http.Server{Addr: ln.Addr().String()}}
		sf := cfg.startFunc()

		sfErr := make(chan error, 1)
		go func() {
			sfErr <- sf()
		}()

		select {
		case <-time.After(time.Second):
			t.Error("func returned by startFunc didn't finish within timeout")
		case err := <-sfErr:
			expMsg := fmt.Sprintf("failed to create listener on %[1]q: listen tcp %[1]s: bind: address already in use", ln.Addr().String())
			if err == nil || err.Error() != expMsg {
				t.Errorf("got unexpected error: %v", err)
			}
		}
	})
}

func Test_serverConf_validate(t *testing.T) {
	t.Parallel()

	t.Run("handlers not assigned", func(t *testing.T) {
		cfg := &serverConf{srv: &http.Server{Addr: "127.0.0.1:0"}}
		err := cfg.validate()
		if err != nil {
			if err != errUnassignedHandler {
				t.Errorf("unexpected error: %v", err)
			}
		} else {
			t.Error("expected non-nil error")
		}
	})

	t.Run("both Addr and listener are unassigned", func(t *testing.T) {
		cfg := &serverConf{srv: &http.Server{Handler: http.NotFoundHandler()}}
		err := cfg.validate()
		if err != nil {
			if err != errUnassignedAddr {
				t.Errorf("unexpected error: %v", err)
			}
		} else {
			t.Error("expected non-nil error")
		}
	})

	t.Run("Addr is assigned", func(t *testing.T) {
		cfg := &serverConf{srv: &http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler()}}
		if err := cfg.validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("listener is assigned", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer ln.Close()

		cfg := &serverConf{l: ln, srv: &http.Server{Handler: http.NotFoundHandler()}}
		if err := cfg.validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
