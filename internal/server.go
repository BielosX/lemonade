package internal

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
	"os"
	"regexp"
	"sync/atomic"
)

type Server struct {
	port                uint16
	logger              *zap.SugaredLogger
	maxWsConnections    int64
	upgrader            *websocket.Upgrader
	connectionCounter   atomic.Int64
	gameNameRegex       *regexp.Regexp
	playerNameRegex     *regexp.Regexp
	maxGameNameLength   uint
	minGameNameLength   uint
	maxPlayerNameLength uint
	minPlayerNameLength uint
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
		gameName = RandomAlphanumeric(s.maxGameNameLength)
		s.logger.Infof("Received empty Game Name, generated name: %s", gameName)
	} else {
		gameName = request.Name
	}
	s.logger.Infof("Creating new game with name: %s", gameName)
}

func (s *Server) readMessage(conn *websocket.Conn, done *OneShot, output chan<- []byte) {
	for {
		messageType, msg, err := conn.ReadMessage()
		if err != nil {
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
			err = conn.Close()
			if err != nil {
				s.logger.Error("Unable to close connection")
			}
			break
		}
		output <- msg
	}
	done.Signal()
}

func (s *Server) writeMessage(conn *websocket.Conn, done *OneShot, input <-chan []byte) {
	for {
		message := <-input
		s.logger.Info("Fetched message to write")
		err := conn.WriteMessage(websocket.BinaryMessage, message)
		if err != nil {
			s.logger.Errorf("Unable to write message: %v", err)
			break
		}
	}
	done.Signal()
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
	defer Close(conn)
	if err != nil {
		s.logger.Errorf("Unable to upgrade connection: %v", err)
		return
	} else if s.connectionCounter.Load() >= s.maxWsConnections {
		s.logger.Info("Sending Too Many Connections")
		err = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(4429, "Too many connections"))
		if err != nil {
			s.logger.Errorf("Unable to write close message: %v", err)
		}
		err = conn.Close()
		if err != nil {
			s.logger.Errorf("Unable to close connection: %v", err)
		}
		return
	}
	s.logger.Infof("WebSocket connection from: %s", conn.RemoteAddr().String())
	s.connectionCounter.Add(1)
	defer s.connectionCounter.Add(-1)
	done := NewOneShot()
	payload := make(chan []byte, 1024)
	go s.readMessage(conn, done, payload)
	go s.writeMessage(conn, done, payload)
	<-done.Done()
}

func NewServer(port uint16,
	logger *zap.Logger,
	maxWsConnections uint64,
	maxGameNameLength uint,
	minGameNameLength uint,
	maxPlayerNameLength uint,
	minPlayerNameLength uint,
	wsReadBufferSize int,
	wsWriteBufferSize int) *Server {
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  wsReadBufferSize,
		WriteBufferSize: wsWriteBufferSize,
		CheckOrigin: func(j *http.Request) bool {
			return true
		},
	}
	gameNameRegex, _ := regexp.Compile(fmt.Sprintf("^\\w{%d,%d}$", minGameNameLength, maxGameNameLength))
	playerNameRegex, _ := regexp.Compile(fmt.Sprintf("^\\w{%d,%d}$", minPlayerNameLength, maxPlayerNameLength))
	return &Server{
		logger:              logger.Sugar(),
		port:                port,
		maxWsConnections:    int64(maxWsConnections),
		upgrader:            upgrader,
		gameNameRegex:       gameNameRegex,
		playerNameRegex:     playerNameRegex,
		maxGameNameLength:   maxGameNameLength,
		minGameNameLength:   minGameNameLength,
		maxPlayerNameLength: maxPlayerNameLength,
		minPlayerNameLength: minPlayerNameLength,
	}
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
		s.maxWsConnections, s.upgrader.ReadBufferSize, s.upgrader.WriteBufferSize)
	s.logger.Infof("Listening on port %d", s.port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil)
	if err != nil {
		s.logger.Errorf("Unable to start listener: %v", err)
		os.Exit(1)
	}
}
