/*
Package httpsrv implements minimalist "framework" to manage http server lifetime.

Setting up the server and managing it's lifetime is repetitive and it is easy to
introduce subtle bugs. This library aims to solve these problems while being
router agnostic and "errgroup pattern" friendly.

This package has no third-party dependencies.

Latest version requires Go 1.20 or newer, to use it with older Go versions use
version v0.1.2 of the package.
*/
package httpsrv
