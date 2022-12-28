package httpsrv

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
)

func runServer(ctx context.Context, start, stop func() error, logErr func(string, ...any)) (rerr error) {
	var m sync.Mutex
	setReturnErr := func(err error) {
		m.Lock()
		defer m.Unlock()
		if rerr == nil {
			rerr = err
		} else if err != nil {
			logErr(err.Error())
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := start(); err != http.ErrServerClosed {
			setReturnErr(fmt.Errorf("http server exited with error: %w", err))
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		setReturnErr(ctx.Err())
	}

	if err := stop(); err != nil {
		setReturnErr(fmt.Errorf("stopping http server: %w", err))
	}

	<-done
	return
}

/*
Run starts the http server "srv" and blocks until it exits. It always return non-nil error.
Server is stopped by cancelling the ctx.

The srv parameter must have Addr and Handler fields assigned unless [Listener] and [Endpoints]
parameters are used to provide respective values.
*/
func Run(ctx context.Context, srv http.Server, params ...ServerParam) error {
	cfg := serverConf{
		srv:    &srv,
		logErr: func(format string, a ...any) { fmt.Fprintln(os.Stderr, fmt.Sprintf(format, a...)) },
	}
	for _, p := range params {
		p.apply(&cfg)
	}

	return runServer(
		ctx,
		cfg.startFunc(),
		cfg.stopFunc(),
		cfg.logErr,
	)
}
