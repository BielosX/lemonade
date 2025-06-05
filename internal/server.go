package internal

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
	"os"
	"sync/atomic"
)

type Server struct {
	port              uint16
	logger            *zap.SugaredLogger
	maxWsConnections  int64
	upgrader          *websocket.Upgrader
	connectionCounter atomic.Int64
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
	return &Server{
		logger:           logger.Sugar(),
		port:             port,
		maxWsConnections: int64(maxWsConnections),
		upgrader:         upgrader,
	}
}

func (s *Server) Serve() {
	router := mux.NewRouter()
	router.Use(LoggingMiddleware(s.logger.Desugar()))
	router.HandleFunc("/health", health)
	router.HandleFunc("/ws", s.webSocketHandler)

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
