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

	certFile, keyFile string // serve TLS if assigned

	// function to log errors during setup/shutdown
	logErr func(format string, args ...any)
}

var (
	errUnassignedAddr    = errors.New("address to listen to is not assigned - to fix use either Listener parameter or set the Addr field of the http.Server parameter of Run")
	errUnassignedHandler = errors.New("misconfigured http server, no handlers attached - to fix use either Endpoints parameter or set the Handler field of the http.Server parameter of Run")
)

func (cfg *serverConf) listener() (net.Listener, error) {
	if cfg.l != nil {
		return cfg.l, nil
	}

	if cfg.srv.Addr == "" {
		return nil, errUnassignedAddr
	}

	var err error
	if cfg.l, err = net.Listen("tcp", cfg.srv.Addr); err != nil {
		return nil, fmt.Errorf("failed to create listener on %q: %w", cfg.srv.Addr, err)
	}
	return cfg.l, nil
}

func (cfg *serverConf) startFunc() func() error {
	s := cfg.srv

	if s.Handler == nil {
		return func() error { return errUnassignedHandler }
	}

	l, err := cfg.listener()
	if err != nil {
		return func() error { return err }
	}

	if cfg.keyFile != "" || cfg.certFile != "" || (s.TLSConfig != nil && (len(s.TLSConfig.Certificates) > 0 || s.TLSConfig.GetCertificate != nil)) {
		return func() error { return s.ServeTLS(l, cfg.certFile, cfg.keyFile) }
	}
	return func() error { return s.Serve(l) }
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
