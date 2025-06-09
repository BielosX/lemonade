package internal

import (
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"net"
	"net/http"
	"strings"
)

func Sync(logger *zap.Logger) {
	_ = logger.Sync()
}

func Close(connection *websocket.Conn) {
	_ = connection.Close()
}

func LoggingMiddleware(logger *zap.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		writer := &zapio.Writer{Log: logger, Level: logger.Level()}
		return handlers.LoggingHandler(writer, next)
	}
}

func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedFor := r.Header.Get("X-Forwarded-For")
		host, port, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil && (host == "::1" || host == "[::1]") {
			r.RemoteAddr = net.JoinHostPort("localhost", port)
		}
		if forwardedFor != "" {
			r.RemoteAddr = strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		}
		next.ServeHTTP(w, r)
	})
}

func WriteWithStatus(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
