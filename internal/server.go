package internal

import (
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
	"os"
)

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func Serve() {
	logger := zap.Must(zap.NewDevelopment(zap.IncreaseLevel(zap.InfoLevel)))
	defer Sync(logger)
	sugar := logger.Sugar()

	router := mux.NewRouter()
	router.Use(LoggingMiddleware(logger))
	router.HandleFunc("/health", health)
	http.Handle("/", router)
	port := GetEnvOrDefault("PORT", "8080")
	sugar.Infof("Listening on port %s", port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		sugar.Errorf("Unable to start listener: %v", err)
		os.Exit(1)
	}
}
