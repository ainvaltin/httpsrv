package httpsrv

import (
	"net"
	"net/http"
	"time"
)

type (
	// ServerParam is the parameter type for [Run] function.
	ServerParam interface {
		apply(cfg *serverConf)
	}

	serverParam struct{ set func(*serverConf) }
)

func (p serverParam) apply(cfg *serverConf) { p.set(cfg) }

/*
Listener allows to set the net listener the http server binds to.

Usually it is easier to just set the Addr field of the srv parameter when calling [Run].
This parameter is mostly useful for testing where server is bind to a random port which
the test need to know.

If both server.Addr is set and Listener is used then the listener set by Listener is used.
*/
func Listener(l net.Listener) ServerParam {
	return serverParam{func(cfg *serverConf) { cfg.l = l }}
}

/*
Endpoints sets the HTTP request multiplexer for the server (Server.Handler field).

Instead of using this parameter the Handler could be assigned in the Server parameter of the [Run]
function but sometimes it is more convenient to set them separately - ie the [http.Server] is returned
by configuration provider while http.Handler is implemented by a service struct.

Endpoints overrides Handler set by the srv parameter of [Run]!
*/
func Endpoints(h http.Handler) ServerParam {
	return serverParam{func(cfg *serverConf) { cfg.srv.Handler = h }}
}

/*
ShutdownTimeout sets timeout for graceful shutdown (ie context timeout for the [http.Server.Shutdown] call).
When not provided or duration is smaller than or equal to zero no graceful shutdown is attempted,
all connections are closed immediately (ie [http.Server.Close] is used to stop the server).

Keep in mind that in "managed environment" (ie Kubernetes) the instance (pod) could still be killed
by the orchestrator before this timeout is reached.
*/
func ShutdownTimeout(to time.Duration) ServerParam {
	return serverParam{func(cfg *serverConf) { cfg.shutdownTO = to }}
}

/*
ShutdownOnPanic instructs the http server to shut down when unhandled panic (except [http.ErrAbortHandler])
escapes some handler. The http server's Close method will be used to shut down the server immediately, ie
the [ShutdownTimeout] parameter is ignored.

By default http.Server just logs the panic and carries on but some argue that in case of
unhandled panic service should always die and new instance started - this option provides
easy way to implement that behavior.
*/
func ShutdownOnPanic() ServerParam {
	return serverParam{func(cfg *serverConf) { cfg.dieOnPanic = true }}
}

/*
TLS allows to start the server using [http.Server.ServeTLS].
Alternatively the server's [http.Server.TLSConfig] field can be assigned when passing it to [Run].
*/
func TLS(certFile, keyFile string) ServerParam {
	return serverParam{func(cfg *serverConf) { cfg.certFile, cfg.keyFile = certFile, keyFile }}
}
