# Changelog

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