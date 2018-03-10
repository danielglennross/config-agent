package routing

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/danielglennross/config-agent/api"
	"github.com/danielglennross/config-agent/broadcast"
	"github.com/danielglennross/config-agent/logger"
	"github.com/danielglennross/config-agent/store"
	"github.com/gorilla/mux"
	"github.com/twinj/uuid"
)

var (
	log = logger.NewLogger(logger.Fields{
		"file": "handler",
	})
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

func logReq(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationToken := r.Context().Value(api.ContextKey("CorrelationToken")).(string)
		reqLog := logger.NewRequestLogger(
			logger.SetCorrelationToken(correlationToken),
			logger.SetRequestInfo(r.Method, r.RequestURI),
		)

		reqLog.InfoMsg("Entering request")

		recordingW := newRecordingResponseWriter(w)

		next.ServeHTTP(recordingW, r)

		if recordingW.statusCode > 399 {
			reqLog.ErrorFields("Exiting request", logger.Fields{
				"StatusCode": recordingW.statusCode,
				"Payload":    recordingW.payload,
			})
			return
		}

		reqLog.Info("Exiting request", logger.Fields{
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

		ctx := context.WithValue(r.Context(), api.ContextKey("CorrelationToken"), correlationToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NewRouter ctor
func NewRouter(store store.BagStore, br broadcast.Receiver, bw broadcast.Writer) http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/config/{bag}", api.HandleWebsocket(store, br)).Methods("GET")
	r.HandleFunc("/config/{bag}", api.UpdateBagHandler(store, bw)).Methods("PUT")

	// middleware
	composedHandlers := setContext(logReq(r))

	return composedHandlers
}
