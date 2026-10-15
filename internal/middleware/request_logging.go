package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Roy6lty/minibank/internal/audit"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := requestID(r.Header.Get("X-Request-ID"))
			w.Header().Set("X-Request-ID", requestID)

			ctx := audit.WithRequestID(r.Context(), requestID)
			request := r.WithContext(ctx)
			started := time.Now()
			writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(writer, request)

			route := "unmatched"
			if routeContext := chi.RouteContext(request.Context()); routeContext != nil && routeContext.RoutePattern() != "" {
				route = routeContext.RoutePattern()
			}
			logger.InfoContext(
				ctx,
				"http request completed",
				"request_id", requestID,
				"method", r.Method,
				"route", route,
				"status", writer.status,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

func RequestID(ctx context.Context) string {
	return audit.RequestID(ctx)
}

func requestID(value string) string {
	if value != "" && len(value) <= 128 && isSafeRequestID(value) {
		return value
	}
	return uuid.NewString()
}

func isSafeRequestID(value string) bool {
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("-_.", character) {
			continue
		}
		return false
	}
	return true
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
