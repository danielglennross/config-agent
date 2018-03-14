package routing

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/danielglennross/config-agent/api"
	"github.com/danielglennross/config-agent/broadcast"
	"github.com/danielglennross/config-agent/logger"
	"github.com/danielglennross/config-agent/store"
	"github.com/gorilla/mux"
	"github.com/twinj/uuid"
)

type recordingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	payload    string
}

func newRecordingResponseWriter(w http.ResponseWriter) *recordingResponseWriter {
	return &recordingResponseWriter{w, http.StatusNoContent, "<empty>"}
}

func (rrw *recordingResponseWriter) WriteHeader(code int) {
	rrw.statusCode = code
	rrw.ResponseWriter.WriteHeader(code)
}

func (rrw *recordingResponseWriter) Write(data []byte) (int, error) {
	rrw.payload = string(data)
	return rrw.ResponseWriter.Write(data)
}

func (rrw *recordingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rrw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}

	return nil, nil, errors.New("I'm not a Hijacker")
}

func logReq(next http.Handler, log *logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextLog := log.NewContext(r.Context())

		contextLog.InfoMsg("Entering request")

		recordingW := newRecordingResponseWriter(w)

		next.ServeHTTP(recordingW, r)

		if recordingW.statusCode > 399 {
			contextLog.ErrorFields("Exiting request", logger.Fields{
				"StatusCode": recordingW.statusCode,
				"Payload":    recordingW.payload,
			})
			return
		}

		contextLog.Info("Exiting request", logger.Fields{
			"StatusCode": recordingW.statusCode,
			"Payload":    recordingW.payload,
		})
	})
}

func setContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var correlationToken = r.Header.Get("X-Correlation-Token")
		if correlationToken == "" {
			correlationToken = uuid.NewV4().String()
		}

		var ctx = r.Context()
		ctx = context.WithValue(ctx, logger.ContextKey("CorrelationToken"), correlationToken)
		ctx = context.WithValue(ctx, logger.ContextKey("Request"), fmt.Sprintf("[%s] %s", r.Method, r.RequestURI))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NewRouter ctor
func NewRouter(store store.BagStore, br broadcast.Receiver, bw broadcast.Writer, log *logger.Logger) http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/config/{bag}", api.HandleWebsocket(store, br)).Methods("GET")
	r.HandleFunc("/config/{bag}", api.UpdateBagHandler(store, bw)).Methods("PUT")

	// middleware
	composedHandlers := setContext(logReq(r, log))

	return composedHandlers
}
