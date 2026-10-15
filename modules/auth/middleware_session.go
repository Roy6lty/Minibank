package auth

import (
	"context"
	"net/http"

	"github.com/Roy6lty/minibank/internal/audit"
)

type ActiveSessionChecker func(context.Context, Principal) (bool, error)

func RequireActiveSession(
	checker ActiveSessionChecker,
	recorders ...audit.Recorder,
) func(http.Handler) http.Handler {
	recorder := optionalAuditRecorder(recorders)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				recordAuditEvent(r.Context(), recorder, audit.Event{
					EventType: audit.EventAuthenticationFailed,
					Outcome:   audit.OutcomeFailure,
					RequestID: audit.RequestID(r.Context()),
				})
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			active, err := checker(r.Context(), principal)
			if err != nil {
				recordAuditEvent(r.Context(), recorder, audit.Event{
					EventType:   audit.EventAuthenticationFailed,
					Outcome:     audit.OutcomeFailure,
					ActorUserID: principal.UserID,
					RequestID:   audit.RequestID(r.Context()),
				})
				http.Error(w, "session status unavailable", http.StatusServiceUnavailable)
				return
			}
			if !active {
				recordAuditEvent(r.Context(), recorder, audit.Event{
					EventType:   audit.EventAuthenticationFailed,
					Outcome:     audit.OutcomeFailure,
					ActorUserID: principal.UserID,
					RequestID:   audit.RequestID(r.Context()),
				})
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
