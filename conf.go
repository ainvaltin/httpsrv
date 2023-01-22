package httpsrv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type serverConf struct {
	srv *http.Server
	l   net.Listener

	shutdownTO time.Duration // timeout for graceful shutdown

	dieOnPanic bool

	certFile, keyFile string // serve TLS if assigned

	// function to log errors during setup/shutdown
	logErr func(format string, args ...any)
}

var (
	errUnassignedAddr    = errors.New("address to listen to is not assigned - to fix use either Listener parameter or set the Addr field of the http.Server parameter of Run")
	errUnassignedHandler = errors.New("misconfigured http server, no handlers attached - to fix use either Endpoints parameter or set the Handler field of the http.Server parameter of Run")
)

func (cfg *serverConf) validate() error {
	if cfg.srv.Handler == nil {
		return errUnassignedHandler
	}

	if cfg.srv.Addr == "" && cfg.l == nil {
		return errUnassignedAddr
	}

	return nil
}

func (cfg *serverConf) listener() (net.Listener, error) {
	if cfg.l != nil {
		return cfg.l, nil
	}

	var err error
	if cfg.l, err = net.Listen("tcp", cfg.srv.Addr); err != nil {
		return nil, fmt.Errorf("failed to create listener on %q: %w", cfg.srv.Addr, err)
	}
	return cfg.l, nil
}

func (cfg *serverConf) startFunc() func() error {
	l, err := cfg.listener()
	if err != nil {
		return func() error { return err }
	}

	hasTLSConfig := cfg.srv.TLSConfig != nil && (len(cfg.srv.TLSConfig.Certificates) > 0 || cfg.srv.TLSConfig.GetCertificate != nil)
	if cfg.keyFile != "" || cfg.certFile != "" || hasTLSConfig {
		return func() error { return cfg.srv.ServeTLS(l, cfg.certFile, cfg.keyFile) }
	}
	return func() error { return cfg.srv.Serve(l) }
}

func (cfg *serverConf) stopFunc() func() error {
	if cfg.shutdownTO <= 0 {
		return func() error { return cfg.srv.Close() }
	}

	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTO)
		defer cancel()
		return cfg.srv.Shutdown(ctx)
	}
}
