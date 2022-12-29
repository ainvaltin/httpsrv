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
function but sometimes it is more convinient to set them separately - ie the [http.Server] is returned
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
TLS allows to start the server using [net/http.Server.ServeTLS].
Alternatively the server's TLSConfig field can be assigned when passing it to [Run].
*/
func TLS(certFile, keyFile string) ServerParam {
	return serverParam{func(cfg *serverConf) { cfg.certFile, cfg.keyFile = certFile, keyFile }}
}

/*
LogError can be used to set the function which is used by the library (not the http server!) to log
errors (ie used when failing to start or stop the server - the first error is returned by the [Run]
and consecutive ones are logged by this func).

By default errors are logged to [os.Stderr].
*/
func LogError(f func(string, ...any)) ServerParam {
	return serverParam{
		func(cfg *serverConf) {
			if f != nil {
				cfg.logErr = f
			}
		},
	}
}
