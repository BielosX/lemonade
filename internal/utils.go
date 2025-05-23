package internal

import (
	"github.com/gorilla/handlers"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"net/http"
	"os"
)

func Sync(logger *zap.Logger) {
	_ = logger.Sync()
}

func GetEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		writer := &zapio.Writer{Log: logger, Level: logger.Level()}
		return handlers.LoggingHandler(writer, next)
	}
}
