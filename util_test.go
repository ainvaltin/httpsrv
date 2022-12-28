package httpsrv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func Test_ListenForQuitSignal(t *testing.T) {
	t.Parallel()

	t.Run("parent context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(700 * time.Millisecond)
			cancel()
		}()

		done := make(chan error, 1)
		go func() {
			done <- ListenForQuitSignal(ctx)
		}()

		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected error %q, got %q", context.Canceled, err.Error())
			}
		case <-time.After(time.Second):
			t.Fatal("test didn't complete within timeout")
		}
	})

	t.Run("os.Interrupt", func(t *testing.T) {
		s, err := runTestCommand("TestSignalInterrupt")
		if err != nil {
			t.Fatalf("failed to run test: %v", err)
		}
		if s != `interrupt: received quit signal` {
			t.Errorf("unexpected return value:\n%s\n", s)
		}
	})

	t.Run("syscall.SIGTERM", func(t *testing.T) {
		s, err := runTestCommand("TestSignalSIGTERM")
		if err != nil {
			t.Fatalf("failed to run test: %v", err)
		}
		if s != `terminated: received quit signal` {
			t.Errorf("unexpected return value:\n%s\n", s)
		}
	})
}

func runTestCommand(testName string) (string, error) {
	cmd := exec.Command(os.Args[0], "-test.run="+testName)
	cmd.Env = []string{"GO_TEST_PROCESS=1"}
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run the command: %w", err)
	}

	return out.String(), nil
}

func sendSignalToItself(sig os.Signal) error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return fmt.Errorf("failed to find the process: %w", err)
	}

	if err := p.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal (%d) to the process: %w", sig, err)
	}

	return nil
}

func testListenForQuitSignal(sig os.Signal) {
	done := make(chan error, 1)
	go func() {
		done <- ListenForQuitSignal(context.Background())
	}()
	// delay to allow the goroutine to register the signal handler
	time.Sleep(500 * time.Millisecond)

	if err := sendSignalToItself(sig); err != nil {
		fmt.Fprint(os.Stdout, err.Error())
		os.Exit(1)
	}

	select {
	case err := <-done:
		if err == nil {
			fmt.Print("unexpectedly got nil error")
		} else if !errors.Is(err, ErrReceivedQuitSignal) {
			fmt.Printf("unexpected error: %v", err)
		} else {
			fmt.Fprint(os.Stdout, err.Error())
		}
	case <-time.After(2 * time.Second):
		fmt.Print("test didn't complete within timeout")
	}
	os.Exit(0)
}

func TestSignalInterrupt(t *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}

	testListenForQuitSignal(os.Interrupt)
}

func TestSignalSIGTERM(t *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}

	testListenForQuitSignal(syscall.SIGTERM)
}
