package httpsrv_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ainvaltin/httpsrv"
)

func Example() {
	// create/init dependencies in the main func and pass them to
	// the run method (which is thus testable by passing in mocks).
	// typical dependencies are general configuration (ie which address/port
	// the service binds to), logger, service discovery implementation etc
	cfg := &srvConf{}
	err := run(context.Background(), cfg)
	fmt.Println("service exited:", err)
}

// when service contains multiple subprocesses these would be started here as errgroup members.
// in this simple example we do not use errgroup.
func run(ctx context.Context, cfg Configuration) error {
	// context to manage server's lifetime - when interrupt signal is sent the
	// ctx will be cancelled and server will shut down
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	go func() {
		<-ctx.Done()
		stop()
	}()

	s := &service{cfg: cfg}
	return httpsrv.Run(ctx, cfg.HttpServer(), httpsrv.Listener(cfg.Listener()), httpsrv.Endpoints(s.endpoints()))
}

type service struct {
	// dependencies like logger, service/DB discovery etc
	cfg ServiceCfg
}

// endpoints defines all the http endpoints the service exposes.
func (s *service) endpoints() http.Handler {
	// any mux which implements http.Handler can be used, ie gin, echo, gorilla...
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "hello, world")
	})

	return mux
}

type ServiceCfg interface {
	// methods returning config relevant to the service
}

type Configuration interface {
	ServiceCfg
	Listener() net.Listener
	HttpServer() http.Server
}

type srvConf struct {
	// from where to load config etc
}

func (c *srvConf) Listener() net.Listener {
	// in "live conf" we return nil as the address to listen to is set in
	// http.Server returned by HttpServer() - this method is needed to make
	// it easy for tests to listen test specific port when using mocked config.
	return nil
}

func (c *srvConf) HttpServer() http.Server {
	return http.Server{
		Addr:              "127.0.0.1:8080", // could read it from env etc
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		// design choice: could come in as a parameter and then we wouldn't use
		// the Endpoints parameter with Run
		//Handler:         h,
	}
}
