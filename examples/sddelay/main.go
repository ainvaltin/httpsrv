// example project demonstrating how to delay the shutdown of a http service
// (ie to signal bad health for some time before the service dies - this is
// useful in environments where some other service monitors the health endpoint
// of the service and when it signals bad health it is deregistered from
// load balancer / router so that no more traffic is sent to the instance thus
// allowing "cool down" before shutdown).
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ainvaltin/httpsrv"
)

func main() {
	cfg := &srvConf{
		shutdownDelay: 5 * time.Second,
	}
	if err := cfg.validate(); err != nil {
		fmt.Println("invalid configuration:", err)
		os.Exit(1)
	}

	monCtx, monCancel := context.WithCancel(context.Background())
	go func() {
		err := monitorStatus(monCtx, fmt.Sprintf("http://%s/health", cfg.Listener().Addr().String()))
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Println("monitor failed:", err)
		}
	}()

	fmt.Printf("starting http server on %s... press Ctrl+C to stop it (will take %s)\n", cfg.Listener().Addr().String(), cfg.shutdownDelay)
	err := run(context.Background(), cfg)
	monCancel()
	fmt.Println(time.Now().Format(" 15:04:05"), "service exited:", err)
}

func run(ctx context.Context, cfg Configuration) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return httpsrv.ListenForQuitSignal(ctx) })

	g.Go(func() error {
		s := service{}
		sh := &statusHandler{status: http.StatusOK}
		return httpsrv.Run(
			delayedCancel(ctx, cfg.ShutdownDelay(), sh.SignalShutdown),
			cfg.HttpServer(s.endpoints(sh)),
			httpsrv.Listener(cfg.Listener()),
		)
	})

	return g.Wait()
}

/*
delayedCancel returns context which is cancelled with given "delay" after input context "ctx"
is cancelled. Before starting the wait it calls "onDelay" callback (synchronously so the wait
actually starts after the callback completes).
*/
func delayedCancel(ctx context.Context, delay time.Duration, onDelay func()) context.Context {
	rCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-ctx.Done()
		onDelay()
		time.Sleep(delay)
	}()
	return rCtx
}

type statusHandler struct {
	status int32
}

func (sh *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(int(atomic.LoadInt32(&sh.status)))
}

func (sh *statusHandler) SignalShutdown() {
	atomic.StoreInt32(&sh.status, http.StatusServiceUnavailable)
}

type service struct {
}

func (s *service) endpoints(statusHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", statusHandler.ServeHTTP)

	return mux
}

type Configuration interface {
	Listener() net.Listener
	HttpServer(http.Handler) http.Server
	ShutdownDelay() time.Duration
}

type srvConf struct {
	shutdownDelay time.Duration

	srvLn net.Listener
}

func (c *srvConf) validate() error {
	var err error
	c.srvLn, err = net.Listen("tcp", c.addr())
	if err != nil {
		return fmt.Errorf("failed to create listener for the server: %w", err)
	}

	if s := os.Getenv("SD_DELAY"); s != "" {
		if c.shutdownDelay, err = time.ParseDuration(s); err != nil {
			return fmt.Errorf("invalid value for shutdown delay: %w", err)
		}
	}
	if c.shutdownDelay <= 0 {
		return fmt.Errorf("shutdown delay must be greater than zero, got %s", c.shutdownDelay)
	}

	return nil
}

func (c *srvConf) addr() string {
	if s := os.Getenv("HTTP_ADDR"); s != "" {
		return s
	}
	return "127.0.0.1:0"
}

func (c *srvConf) Listener() net.Listener { return c.srvLn }

func (c *srvConf) ShutdownDelay() time.Duration { return c.shutdownDelay }

func (c *srvConf) HttpServer(h http.Handler) http.Server {
	return http.Server{
		Handler:           h,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

func monitorStatus(ctx context.Context, addr string) error {
	hc := http.Client{Timeout: 1 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}

		rsp, err := hc.Do(req)
		if err != nil {
			fmt.Println(time.Now().Format(" 15:04:05"), "request failed:", err)
			continue
		}
		fmt.Println(time.Now().Format(" 15:04:05"), "server reports status", rsp.Status)
	}
}
