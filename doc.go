/*
Package httpsrv implements minimalist "framework" to manage http server lifetime.

Setting up the server and managing it's lifetime is repetitive and it is easy to introduce subtle bugs.
This library aims to solve these problems while being router agnostic and "errgroup pattern" friendly.

This package has no thirdparty dependencies.
*/
package httpsrv
