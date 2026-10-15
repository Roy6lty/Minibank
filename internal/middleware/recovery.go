package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/Roy6lty/minibank/internal/logging"
)

// Recover prevents a panic in a request handler from terminating the server.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.ErrorContext(
					r.Context(),
					"panic recovered",
					logging.ErrorValue(recovered),
					"request_id", RequestID(r.Context()),
					"method", r.Method,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
