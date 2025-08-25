package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync/atomic"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

type ServerConfig struct {
	Port                         int
	MaxWsConnections             uint64
	MaxGameNameLength            uint
	MinGameNameLength            uint
	MaxPlayerNameLength          uint
	MinPlayerNameLength          uint
	MaxClientWsMessagesPerSecond uint64
	WsReadBufferSize             int
	WsWriteBufferSize            int
	LogLevel                     string
}

type Server struct {
	logger            *zap.SugaredLogger
	upgrader          *websocket.Upgrader
	connectionCounter atomic.Int64
	gameNameRegex     *regexp.Regexp
	playerNameRegex   *regexp.Regexp
	config            ServerConfig
}

func health(w http.ResponseWriter, _ *http.Request) {
	WriteWithStatus(w, http.StatusOK, "OK")
}

type NewGameRequestResponse struct {
	Name string `json:"name,omitempty"`
}

func (s *Server) newGame(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := NewGameRequestResponse{}
	err := decoder.Decode(&request)
	if err != nil {
		WriteWithStatus(w, http.StatusBadRequest, "Unable to decode NewGameRequest")
		return
	}
	var gameName string
	if request.Name != "" && !s.gameNameRegex.MatchString(request.Name) {
		WriteWithStatus(w,
			http.StatusBadRequest,
			fmt.Sprintf("Provided Game Name does not match expression %s", s.gameNameRegex.String()))
		return
	} else if request.Name == "" {
		gameName = RandomAlphanumeric(s.config.MaxGameNameLength)
		s.logger.Infof("Received empty Game Name, generated name: %s", gameName)
	} else {
		gameName = request.Name
	}
	s.logger.Infof("Creating new game with name: %s", gameName)
}

func (s *Server) readMessage(ctx context.Context, conn *websocket.Conn, output chan<- []byte) error {
	var err error
	var messageType int
	var msg []byte
infLoop:
	for {
		messageType, msg, err = conn.ReadMessage()
		if err != nil {
			if IsCloseError(err) {
				break
			}
			s.logger.Errorf("Unable to read message: %v", err)
			break
		}
		if messageType != websocket.BinaryMessage {
			s.logger.Errorf("Expected binary message, got %d", messageType)
			err = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(4400, "Invalid message type"))
			if err != nil {
				s.logger.Errorf("Unable to write close message: %v", err)
			}
			break infLoop
		}
		select {
		case output <- msg:
		case <-ctx.Done():
			break infLoop
		}
	}
	closeErr := conn.Close()
	return errors.Join(err, closeErr)
}

func (s *Server) writeMessage(ctx context.Context, conn *websocket.Conn, input <-chan []byte) error {
	var err error
infLoop:
	for {
		select {
		case message, ok := <-input:
			if !ok {
				break infLoop
			}
			s.logger.Info("Fetched message to write")
			err = conn.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				if IsCloseError(err) {
					break infLoop
				}
				s.logger.Errorf("Unable to write message: %v", err)
				break infLoop
			}
		case <-ctx.Done():
			break infLoop
		}
	}
	closeErr := conn.Close()
	return errors.Join(err, closeErr)
}

func (s *Server) webSocketHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameName := vars["gameName"]
	playerName := vars["playerName"]
	if !s.gameNameRegex.MatchString(gameName) {
		WriteWithStatus(w,
			http.StatusBadRequest,
			fmt.Sprintf("GameName does not match expected expression: %s", s.gameNameRegex.String()))
		return
	}
	if !s.playerNameRegex.MatchString(playerName) {
		WriteWithStatus(w,
			http.StatusBadRequest,
			fmt.Sprintf("PlayerName does not match expected expression: %s", s.playerNameRegex.String()))
		return
	}
	if !websocket.IsWebSocketUpgrade(r) {
		WriteWithStatus(w, http.StatusBadRequest, "Expected WebSocket Upgrade request")
		return
	}
	s.logger.Infof("%s joining the game %s as %s", r.RemoteAddr, gameName, playerName)
	conn, err := s.upgrader.Upgrade(w, r, nil)
	defer ignore(conn.Close)
	if err != nil {
		s.logger.Errorf("Unable to upgrade connection: %v", err)
		return
	} else if s.connectionCounter.Load() >= int64(s.config.MaxWsConnections) {
		s.logger.Info("Sending Too Many Connections")
		err = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(4429, "Too many connections"))
		if err != nil {
			s.logger.Errorf("Unable to write close message: %v", err)
		}
		return
	}
	s.logger.Infof("WebSocket connection from: %s", conn.RemoteAddr().String())
	s.connectionCounter.Add(1)
	defer s.connectionCounter.Add(-1)
	payload := make(chan []byte, 1024)
	defer close(payload)
	group, ctx := errgroup.WithContext(context.Background())
	group.Go(func() error { return s.readMessage(ctx, conn, payload) })
	group.Go(func() error { return s.writeMessage(ctx, conn, payload) })
	err = group.Wait()
	if err != nil {
		if IsCloseError(err) {
			s.logger.Infof("Connection from player %s closed", playerName)
		} else {
			s.logger.Errorf("Unable to handle message: %v", err)
		}
	}
	s.logger.Infof("Handler for player %s game %s finished", playerName, gameName)
}

func NewServer(config ServerConfig) (*Server, error) {
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  config.WsReadBufferSize,
		WriteBufferSize: config.WsWriteBufferSize,
		CheckOrigin: func(j *http.Request) bool {
			return true
		},
	}
	level, err := zapcore.ParseLevel(config.LogLevel)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid log level: %s\n", config.LogLevel)
		return nil, err
	}
	logConfig := zap.NewDevelopmentConfig()
	logConfig.Level.SetLevel(level)
	logger := zap.Must(logConfig.Build())
	gameNameRegex, _ := regexp.Compile(fmt.Sprintf("^\\w{%d,%d}$",
		config.MinGameNameLength, config.MaxGameNameLength))
	playerNameRegex, _ := regexp.Compile(fmt.Sprintf("^\\w{%d,%d}$",
		config.MinPlayerNameLength, config.MaxPlayerNameLength))
	return &Server{
		logger:          logger.Sugar(),
		upgrader:        upgrader,
		gameNameRegex:   gameNameRegex,
		playerNameRegex: playerNameRegex,
		config:          config,
	}, nil
}

func (s *Server) Serve() {
	router := mux.NewRouter()
	router.Use(LoggingMiddleware(s.logger.Desugar()))
	router.Use(RealIP)
	router.HandleFunc("/health", health).Methods("GET")
	router.HandleFunc("/ws/games/{gameName}/players/{playerName}", s.webSocketHandler).Methods("GET")
	router.HandleFunc("/games", s.newGame).Methods("POST")

	http.Handle("/", router)
	s.logger.Infof("Starting Server with MaxWsConnections: %d, WsReadBufferSize: %d, WsWriteBufferSize: %d",
		s.config.MaxWsConnections, s.upgrader.ReadBufferSize, s.upgrader.WriteBufferSize)
	s.logger.Infof("Listening on port %d", s.config.Port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", s.config.Port), nil)
	if err != nil {
		s.logger.Errorf("Unable to start listener: %v", err)
		os.Exit(1)
	}
}

func (s *Server) Shutdown() {
	_ = s.logger.Sync()
}
