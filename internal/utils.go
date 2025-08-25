package internal

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
)

func ignore(fn func() error) {
	_ = fn()
}

func IsCloseError(err error) bool {
	var closeError *websocket.CloseError
	if errors.As(err, &closeError) || errors.Is(err, net.ErrClosed) {
		return true
	}
	return false
}

func LoggingMiddleware(logger *zap.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		writer := &zapio.Writer{Log: logger, Level: logger.Level()}
		return handlers.LoggingHandler(writer, next)
	}
}

func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedFor := r.Header.Get("X-Forwarded-For")
		host, port, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil && (host == "::1" || host == "[::1]") {
			r.RemoteAddr = net.JoinHostPort("localhost", port)
		}
		if forwardedFor != "" {
			r.RemoteAddr = strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		}
		next.ServeHTTP(w, r)
	})
}

func WriteWithStatus(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func charRange(from byte, to byte) string {
	count := int(to - from + 1)
	result := make([]byte, count)
	for i := 0; i < count; i++ {
		result[i] = from + byte(i)
	}
	return string(result)
}

var LowerLetters = charRange('a', 'z')
var UpperLetters = charRange('A', 'Z')
var Digits = charRange('0', '9')
var Alphanumeric = fmt.Sprintf("%s%s%s", LowerLetters, UpperLetters, Digits)

func RandomAlphanumeric(length uint) string {
	result := make([]byte, length)
	for i := uint(0); i < length; i++ {
		index := rand.Int31n(int32(len(Alphanumeric)))
		result[i] = Alphanumeric[index]
	}
	return string(result)
}
