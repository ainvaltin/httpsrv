package httpsrv_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/ainvaltin/httpsrv"
)

// example of a simple service which only has a http server (ie no need to use errgroup)
func ExampleRun() {
	// any mux which implements http.Handler can be used, ie gin, echo, gorilla...
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "hello, world")
	})

	// context to manage server's lifetime - when interrupt signal is sent the
	// ctx will be cancelled and server stops
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := httpsrv.Run(ctx, http.Server{Addr: "127.0.0.1:8080", Handler: mux})
	fmt.Println("server exited:", err)
}

// Listener parameter is useful for tests where server is running on a random port
// which the test needs to know in order to make request to it.
func ExampleListener() {
	// this would be a parameter of a Test func ie "func TestXXX(t *testing.T)"
	var t testing.T
	// open listener on random free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	// start the http server on the port
	ctx, cancel := context.WithCancel(context.Background())
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- httpsrv.Run(ctx,
			http.Server{Handler: http.NotFoundHandler()},
			httpsrv.Listener(ln),
		)
	}()

	// make a request to the server
	c := &http.Client{Timeout: time.Second}
	rsp, err := c.Get(fmt.Sprintf("http://%s", ln.Addr().String()))
	if err != nil {
		t.Errorf("GET request returned unexpected error: %v", err)
	}
	if rsp == nil {
		t.Error("unexpectedly GET request returned nil response")
	}

	// stop the server
	cancel()

	select {
	case <-time.After(time.Second):
		t.Fatal("server didn't stop within timeout")
	case err := <-srvErr:
		if err == nil {
			t.Fatal("unexpectedly Run returned nil error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected %q, got %q", context.Canceled, err)
		}
	}
}
