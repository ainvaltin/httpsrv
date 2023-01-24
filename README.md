[![Go Reference](https://pkg.go.dev/badge/github.com/ainvaltin/httpsrv.svg)](https://pkg.go.dev/github.com/ainvaltin/httpsrv)

# httpsrv

Package `httpsrv` implements minimalist "framework" to manage http server lifetime.

Setting up server and managing it's lifetime is repetitive and it is easy to
introduce subtle bugs. This library aims to solve these problems while being
router agnostic and "errgroup pattern" friendly.

To a seasoned Go developer this might seem like too little functionality to warrant
a package (a little copying is better than a little dependency) but IME people coming
from other languages feel uncomfortable when there is no framework to set up a service :)
And when maintaining multiple services _it is_ nice when they all follow the same
basic setup pattern.
Also, when considering the tests too the amount of code to copy from project to
project is not that small anymore...

Simplest example of using this package:
```go
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, "Hello, World!")
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		stop()
	}()

	err := httpsrv.Run(ctx, http.Server{Addr: "127.0.0.1:8080", Handler: mux})
	fmt.Println("server exited:", err)
}
```

## errgroup pattern

Simply put "errgroup pattern" is a organization of the service code so that all
subprocesses of the service are launched as members of the same
[errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) and their lifetime is
controlled by the context of the group. This means that when one of the group members
exits with error the group's context gets cancelled and all other group members
get signal to (gracefully) exit too. Group reports the first error as the reason
service stopped.

Rules for a func starting a subprocess are:
 - every group member lifetime is controlled by group's context - when it gets cancelled
 the subprocess should exit (gracefully) ASAP;
 - subprocess always returns non-nil error (most of the time it would be the `ctx.Err()`
 ie the subprocess exits because the context controlling it's lifetime has been cancelled).

See the [example project](./examples/errgroup/) for more.

## Possible improvements
- support user defined panic handlers so that if none of the handlers mark panic as "handled"
then service will be shut down (IOW configurable `ShutdownOnPanic`);
- Drop the `LogError` parameter and use "wrapping multiple errors" instead (support coming with Go 1.20).
- `ListenForQuitSignal` and `WaitWithTimeout` should probably live in a separate package as these
functions are not `httpsrv` specific but general "errgroup pattern" helpers.
