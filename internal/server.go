package internal

import (
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"net/http"
	"os"
)

func Serve() {
	logger := zap.Must(zap.NewDevelopment(zap.IncreaseLevel(zap.InfoLevel)))
	defer Sync(logger)
	sugar := logger.Sugar()

	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		writer := &zapio.Writer{Log: logger, Level: logger.Level()}
		return handlers.LoggingHandler(writer, next)
	})
	router.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("OK"))
		if err != nil {
			return
		}
	})
	http.Handle("/", router)
	port := GetEnvOrDefault("PORT", "8080")
	sugar.Infof("Listening on port %s", port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		sugar.Errorf("Unable to start listener: %v", err)
		os.Exit(1)
	}
}
