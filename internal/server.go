package internal

import (
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
	port              uint16
	logger            *zap.SugaredLogger
	maxWsConnections  int64
	upgrader          *websocket.Upgrader
	connectionCounter atomic.Int64
	gameNameRegex     *regexp.Regexp
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
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
	gameName := mux.Vars(r)["name"]
	if !s.gameNameRegex.MatchString(gameName) {
		WriteWithStatus(w,
			http.StatusBadRequest,
			fmt.Sprintf("GameName does not match expected expression: %s", s.gameNameRegex.String()))
		return
	}
	s.logger.Infof("%s joining the game %s", r.RemoteAddr, gameName)
	if !websocket.IsWebSocketUpgrade(r) {
		WriteWithStatus(w, http.StatusBadRequest, "Expected WebSocket Upgrade request")
		return
	}
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
	wsReadBufferSize int,
	wsWriteBufferSize int) *Server {
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  wsReadBufferSize,
		WriteBufferSize: wsWriteBufferSize,
		CheckOrigin: func(j *http.Request) bool {
			return true
		},
	}
	regex, _ := regexp.Compile("^\\w{1,15}$")
	return &Server{
		logger:           logger.Sugar(),
		port:             port,
		maxWsConnections: int64(maxWsConnections),
		upgrader:         upgrader,
		gameNameRegex:    regex,
	}
}

func (s *Server) Serve() {
	router := mux.NewRouter()
	router.Use(LoggingMiddleware(s.logger.Desugar()))
	router.Use(RealIP)
	router.HandleFunc("/health", health).Methods("GET")
	router.HandleFunc("/join/{name}", s.webSocketHandler).Methods("GET")

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
