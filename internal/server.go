package internal

import (
	"fmt"
	"github.com/gorilla/handlers"
	"go.uber.org/zap"
	"net/http"
	"os"
)

func handleFunc(pattern string, handler http.HandlerFunc) {
	http.Handle(pattern, handlers.LoggingHandler(os.Stdout, handler))
}

func Serve() {
	logger := zap.Must(zap.NewDevelopment())
	defer Sync(logger)
	sugar := logger.Sugar()
	handleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("OK"))
		if err != nil {
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	port := GetEnvOrDefault("PORT", "8080")
	sugar.Infof("Listening on port %s", port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		sugar.Errorf("Unable to start listener: %v", err)
		os.Exit(1)
	}
}
