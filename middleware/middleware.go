package middleware

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"
)

// RequestIDKey is the context key for the request ID
type RequestIDKey struct{}

// LoggingMiddleware logs request information and adds a request ID to the context
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()

		// Add request ID to context
		ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Create a response wrapper to capture the status code
		rw := &responseWriter{w, http.StatusOK}

		// Log the incoming request
		log.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Msg("Request received")

		// Call the next handler with the updated context
		next.ServeHTTP(rw, r.WithContext(ctx))

		// Log the response
		log.Info().
			Str("request_id", requestID).
			Int("status", rw.status).
			Dur("duration", time.Since(start)).
			Msg("Request completed")
	})
}

// TimeoutMiddleware adds a timeout to the request context
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					w.WriteHeader(http.StatusGatewayTimeout)
					w.Write([]byte("Request timeout"))
				}
			}
		})
	}
}

// responseWriter is a wrapper for http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code before writing it
func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// RecoverMiddleware recovers from panics and logs the error
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID, _ := r.Context().Value(RequestIDKey{}).(string)
				log.Error().
					Str("request_id", requestID).
					Interface("error", err).
					Msg("Panic recovered")
				
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
