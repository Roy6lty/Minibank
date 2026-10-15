package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Roy6lty/minibank/internal/audit"
)

type principalContextKey struct{}

func recordAuditEvent(ctx context.Context, recorder audit.Recorder, event audit.Event) {
	if recorder == nil {
		return
	}
	if err := recorder.Record(ctx, event); err != nil {
		slog.ErrorContext(ctx, "record authentication audit event", "error", err)
	}
}

func optionalAuditRecorder(recorders []audit.Recorder) audit.Recorder {
	if len(recorders) == 0 {
		return nil
	}
	return recorders[0]
}

func RequireAuthentication(
	verifier TokenVerifier,
	recorders ...audit.Recorder,
) func(http.Handler) http.Handler {
	recorder := optionalAuditRecorder(recorders)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				recordAuditEvent(r.Context(), recorder, audit.Event{
					EventType: audit.EventAuthenticationFailed,
					Outcome:   audit.OutcomeFailure,
					RequestID: audit.RequestID(r.Context()),
				})
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			principal, err := verifier.VerifyAccessToken(parts[1])
			if err != nil {
				recordAuditEvent(r.Context(), recorder, audit.Event{
					EventType: audit.EventAuthenticationFailed,
					Outcome:   audit.OutcomeFailure,
					RequestID: audit.RequestID(r.Context()),
				})
				http.Error(w, "invalid access token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
			ctx = audit.WithActorUserID(ctx, principal.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}
