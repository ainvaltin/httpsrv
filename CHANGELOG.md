# Changelog

## v0.3.1 (11.11.2023)
- dropped utility functions `WaitWithTimeout` and `ListenForQuitSignal` - these
are now part of `github.com/ainvaltin/wake` package;
- examples upgraded and updated.

## v0.3.0 (28.10.2023)
- **breaking change**: `Run` takes `*http.Server` (instead of `http.Server` ie pointer
instead of value) as a parameter because http.Server now contains atomic values which
may not be copied.
- deprecate utility functions `WaitWithTimeout` and `ListenForQuitSignal` - these
are moved to `github.com/ainvaltin/wake` package.

## v0.2.1 (03.03.2023)
- detect `http.ErrAbortHandler` even when it is wrapped inside another error.

## v0.2.0 (25.02.2023)
- **breaking change**: dropped `LogError` option in favour of `errors.Join`, **requires Go 1.20**!

## v0.1.2 (22.01.2023)
- new `ShutdownOnPanic` option.

## v0.1.1 (17.01.2023)
- new utility function `WaitWithTimeout`.

## v0.1.0 (01.01.2023)
- initial release.