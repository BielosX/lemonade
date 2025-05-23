package internal

import (
	"go.uber.org/zap"
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
