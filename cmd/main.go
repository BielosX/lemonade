package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/BielosX/lemonade/internal"
	"go.uber.org/zap"
)

func main() {
	serverConfig := internal.ServerConfig{}
	flag.UintVar(
		&serverConfig.MaxPlayerNameLength,
		"max-player-name-len",
		15,
		"Max player name length",
	)
	flag.UintVar(
		&serverConfig.MinPlayerNameLength,
		"min-player-name-len",
		5,
		"Min player name length",
	)
	flag.UintVar(&serverConfig.MaxGameNameLength, "max-game-name-len", 15, "Max game name length")
	flag.UintVar(&serverConfig.MinGameNameLength, "min-game-name-len", 5, "Min game name length")
	flag.Uint64Var(
		&serverConfig.MaxWsConnections,
		"max-ws-connections",
		256,
		"Max number of concurrent WebSocket connections",
	)
	flag.IntVar(&serverConfig.Port, "port", 8080, "Port to listen on")
	flag.StringVar(&serverConfig.LogLevel, "log-level", zap.InfoLevel.String(), "Log level")
	flag.IntVar(
		&serverConfig.WsReadBufferSize,
		"ws-read-buffer-size",
		1024*64,
		"WebSocket read buffer size",
	)
	flag.IntVar(
		&serverConfig.WsWriteBufferSize,
		"ws-write-buffer-size",
		1024*64,
		"WebSocket write buffer size",
	)
	flag.Parse()
	if serverConfig.Port < 1 || serverConfig.Port > math.MaxUint16 {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid port: %d\n", serverConfig.Port)
		os.Exit(1)
	}
	if serverConfig.MinGameNameLength == 0 ||
		serverConfig.MinGameNameLength > serverConfig.MaxGameNameLength {
		_, _ = fmt.Fprintf(
			os.Stderr,
			"MinGameNameLen should be greater than 0 and less or equal MaxGameNameLen",
		)
		os.Exit(1)
	}
	server, err := internal.NewServer(serverConfig)
	if err != nil {
		os.Exit(1)
	}
	defer server.Shutdown()
	server.Serve()
}
