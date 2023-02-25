package httpsrv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

func runServer(ctx context.Context, start, stop func() error, shutdown chan error) (rerr error) {
	var m sync.Mutex
	setReturnErr := func(err error) {
		m.Lock()
		defer m.Unlock()
		if rerr == nil {
			rerr = err
		} else if err != nil {
			rerr = errors.Join(rerr, err)
		}
	}

	serveQuit := make(chan struct{})
	go func() {
		defer close(serveQuit)
		if err := start(); err != http.ErrServerClosed {
			setReturnErr(fmt.Errorf("http server exited with error: %w", err))
		}
	}()

	select {
	case <-serveQuit:
	case <-ctx.Done():
		setReturnErr(ctx.Err())
	case err := <-shutdown:
		setReturnErr(err)
		<-serveQuit
		return
	}

	if err := stop(); err != nil {
		setReturnErr(fmt.Errorf("stopping http server: %w", err))
	}

	<-serveQuit
	return
}

/*
Run starts the http server "srv" and blocks until it exits. It always return non-nil error.
Server is stopped by cancelling the ctx.

The srv parameter must have Addr and Handler fields assigned unless [Listener] and [Endpoints]
parameters are used to provide respective values.
*/
func Run(ctx context.Context, srv http.Server, params ...ServerParam) error {
	cfg := serverConf{srv: &srv}
	for _, p := range params {
		p.apply(&cfg)
	}
	if err := cfg.validate(); err != nil {
		return err
	}

	var shutdown chan error
	if cfg.dieOnPanic {
		shutdown = installDieOnPanicHandler(cfg.srv)
	}

	return runServer(
		ctx,
		cfg.startFunc(),
		cfg.stopFunc(),
		shutdown,
	)
}

func installDieOnPanicHandler(srv *http.Server) chan error {
	done := make(chan error)
	next := srv.Handler
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok && err == http.ErrAbortHandler {
					return
				}
				done <- fmt.Errorf("unhandled panic: %v", r)
				srv.Close()
			}
		}()

		next.ServeHTTP(w, r)
	})
	return done
}
