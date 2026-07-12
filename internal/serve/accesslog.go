package serve

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = 200
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// accessLog wraps an http.Handler with standard access logging.
func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 0}
		next.ServeHTTP(rw, r)
		dur := time.Since(start)
		if rw.status == 0 {
			rw.status = 200
		}
		log.Info("access",
			"method", r.Method,
			"path", r.URL.RequestURI(),
			"status", rw.status,
			"duration", dur.Round(time.Microsecond).String(),
			"size", rw.size,
			"remote", r.RemoteAddr,
		)
	})
}
