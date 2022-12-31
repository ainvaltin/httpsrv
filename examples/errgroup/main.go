// example project using "errgroup pattern" and httpsrv library
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ainvaltin/httpsrv"
)

func main() {
	// create/init dependencies in the main func and pass them to
	// the run method (which is thus testable by passing in mocks).
	// typical dependencies are general configuration (ie which address/port
	// the service binds to), logger, service discovery implementation etc
	// see the main_test.go for how run can be tested.
	err := run(context.Background(), &srvConf{})
	fmt.Println("service exited:", err)
}

// run starts all the subprocesses of the service
func run(ctx context.Context, cfg Configuration) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return httpsrv.ListenForQuitSignal(ctx) })

	g.Go(func() error {
		s := &service{cfg: cfg}
		return httpsrv.Run(ctx, cfg.HttpServer(s.endpoints()), httpsrv.Listener(cfg.Listener()))
	})

	g.Go(func() error {
		return cron(ctx, cfg.MaxTicks())
	})

	return g.Wait()
}

/*
cron is a example of possible subprocess which has it's lifetime controlled by context.
*/
func cron(ctx context.Context, maxTickc int) error {
	cnt := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			fmt.Println("tick")
			if cnt++; cnt == maxTickc {
				return fmt.Errorf("ticked %d times, that's enough", cnt)
			}
		}
	}
}

type service struct {
	// dependencies like logger, DB discovery etc which are used by http handlers
	cfg ServiceCfg
}

func (s *service) endpoints() http.Handler {
	// any mux which implements http.Handler can be used, ie gin, echo, gorilla...
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.helloWorld)

	return mux
}

func (s *service) helloWorld(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

// the http service specific configuration interface
type ServiceCfg interface {
	DB() (*sql.DB, error)
}

type Configuration interface {
	ServiceCfg
	MaxTicks() int
	Listener() net.Listener
	HttpServer(http.Handler) http.Server
}

type srvConf struct {
	// from where to load config etc

	// we use the same conf struct for the tests too thats why we need this field
	// see the main_test.go for how it is used
	l net.Listener
}

func (c *srvConf) Listener() net.Listener {
	// in "live conf" we return nil as the address to listen to is set in
	// http.Server returned by HttpServer() - this method is needed to make
	// it easy for tests to listen test specific port.
	return c.l
}

func (c *srvConf) HttpServer(h http.Handler) http.Server {
	return http.Server{
		Addr:              "127.0.0.1:8080", // could read it from env etc
		Handler:           h,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

func (c *srvConf) MaxTicks() int {
	// in real life code would probably want to log error etc
	if v, err := strconv.Atoi(os.Getenv("MAX_TICKS")); err == nil {
		return v
	}
	return 3
}

func (c *srvConf) DB() (*sql.DB, error) {
	return nil, fmt.Errorf("not implemented")
}
