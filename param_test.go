package httpsrv

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func Test_ServerParam(t *testing.T) {
	t.Parallel()

	// check that correct config field is assigned

	t.Run("Listener", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer ln.Close()

		cfg := serverConf{}
		Listener(ln).apply(&cfg)
		if cfg.l == nil {
			t.Fatal("expected that the cfg.l is assigned")
		}
		if cfg.l != ln {
			t.Error("config has different listener assigned")
		}
	})

	t.Run("Endpoints", func(t *testing.T) {
		cfg := serverConf{srv: &http.Server{}}
		Endpoints(http.NotFoundHandler()).apply(&cfg)
		if cfg.srv.Handler == nil {
			t.Fatal("expected that the cfg.srv.Handler is assigned")
		}
	})

	t.Run("ShutdownTimeout", func(t *testing.T) {
		cfg := serverConf{}
		ShutdownTimeout(time.Second).apply(&cfg)
		if cfg.shutdownTO != time.Second {
			t.Errorf("unexpected timeout value %s", cfg.shutdownTO)
		}
	})

	t.Run("TLS", func(t *testing.T) {
		cfg := serverConf{}
		TLS("cert", "key").apply(&cfg)
		if cfg.certFile != "cert" {
			t.Errorf("unexpected certFile value: %s", cfg.certFile)
		}
		if cfg.keyFile != "key" {
			t.Errorf("unexpected keyFile value: %s", cfg.keyFile)
		}
	})

	logErrFunc := func() (func(string, ...any), *bytes.Buffer) {
		buf := bytes.NewBuffer(nil)
		return func(format string, a ...any) { fmt.Fprintln(buf, fmt.Sprintf(format, a...)) }, buf
	}

	t.Run("LogError", func(t *testing.T) {
		logF, buf := logErrFunc()
		cfg := serverConf{}
		LogError(logF).apply(&cfg)
		if cfg.logErr == nil {
			t.Fatal("unexpectedly log func is not assigned")
		}
		cfg.logErr("test message")
		if s := buf.String(); s != "test message\n" {
			t.Errorf("unexpected message was logged:\n%s\n", s)
		}
	})

	t.Run("LogError nil value is ignored", func(t *testing.T) {
		logF, buf := logErrFunc()
		cfg := serverConf{logErr: logF}
		LogError(nil).apply(&cfg)
		if cfg.logErr == nil {
			t.Fatal("unexpectedly log func is not assigned")
		}
		cfg.logErr("test message")
		if s := buf.String(); s != "test message\n" {
			t.Errorf("unexpected message was logged:\n%s\n", s)
		}
	})
}
