package httpsrv

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
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

	t.Run("no Addr assigned", func(t *testing.T) {
		cfg := &serverConf{srv: &http.Server{}}
		l, err := cfg.listener()
		if err == nil {
			t.Fatal("expected error, got nil")
		} else if !errors.Is(err, errUnassignedAddr) {
			t.Errorf("unexpected error: %v", err)
		}

		if l != nil {
			defer l.Close()
			t.Fatalf("unexpectedly non-nil listener was returned: %s", l.Addr().String())
		}
	})

	t.Run("when both Addr and listener are assigned listener is returned", func(t *testing.T) {
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
		if l != ln {
			t.Fatal("unexpectedly different listener was returned")
		}
		if s := l.Addr().String(); s != addr {
			t.Errorf("the address of the listener has changed from %q to %q", addr, s)
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

	t.Run("try to open the same addr twice", func(t *testing.T) {
		// first attemp should succeed
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
