package internal

import (
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"net/http"
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
