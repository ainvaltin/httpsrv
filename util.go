package httpsrv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

/*
ErrReceivedQuitSignal is returned by [ListenForQuitSignal] when it receives one of the
quit signals it listened for.
*/
var ErrReceivedQuitSignal = errors.New("received quit signal")

/*
ListenForQuitSignal is meant to be used with [errgroup] - as one group member this func causes the
group context to be cancelled when quit signal is sent.
Benefit using it over [signal.NotifyContext] is that signal.NotifyContext returns [context.Cancelled]
no matter whether the signal was sent or parent ctx was cancelled, ListenForQuitSignal returns
[ErrReceivedQuitSignal] for the former case (use [errors.Is] to check for it as it might be wrapped
inside another error describing the signal).
If differentiation between these two cases is not a concern then following func can be used instead:

	g.Go(func() error {
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
		<-ctx.Done()
		return ctx.Err()
	})

If no signals (the sig parameter) is provided then it listens for [os.Interrupt] and [syscall.SIGTERM] ie
following is equivalent to the previous example:

	g.Go(func() error { return httpsrv.ListenForQuitSignal(ctx) })

[errgroup]: https://pkg.go.dev/golang.org/x/sync/errgroup
*/
func ListenForQuitSignal(ctx context.Context, sig ...os.Signal) error {
	if len(sig) == 0 {
		sig = append(sig, os.Interrupt, syscall.SIGTERM)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, sig...)
	defer signal.Stop(c)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case s := <-c:
		return fmt.Errorf("%s: %w", s, ErrReceivedQuitSignal)
	}
}
