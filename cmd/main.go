package main

import (
	"flag"
	"fmt"
	"github.com/BielosX/lemonade/internal"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"math"
	"os"
)

func main() {
	var port int
	var logLevel string
	var maxWsConnections uint64
	var wsReadBufferSize int
	var wsWriteBufferSize int
	flag.Uint64Var(&maxWsConnections, "max-ws-connections", 256, "Max number of concurrent WebSocket connections")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.StringVar(&logLevel, "log-level", zap.InfoLevel.String(), "Log level")
	flag.IntVar(&wsReadBufferSize, "ws-read-buffer-size", 1024*64, "WebSocket read buffer size")
	flag.IntVar(&wsWriteBufferSize, "ws-write-buffer-size", 1024*64, "WebSocket write buffer size")
	flag.Parse()
	if !(port >= 1 && port <= math.MaxUint16) {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid port: %d\n", port)
		os.Exit(1)
	}
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid log level: %s\n", logLevel)
		os.Exit(1)
	}
	config := zap.NewDevelopmentConfig()
	config.Level.SetLevel(level)
	logger := zap.Must(config.Build())
	defer internal.Sync(logger)
	server := internal.NewServer(uint16(port), logger, maxWsConnections, wsReadBufferSize, wsWriteBufferSize)
	server.Serve()
}
