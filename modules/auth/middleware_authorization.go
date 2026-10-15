package auth

import (
	"net/http"

	"github.com/Roy6lty/minibank/internal/audit"
)

func RequirePermission(
	permission Permission,
	recorders ...audit.Recorder,
) func(http.Handler) http.Handler {
	return requireAllPermissions([]Permission{permission}, optionalAuditRecorder(recorders))
}

func RequireAllPermissions(required ...Permission) func(http.Handler) http.Handler {
	return requireAllPermissions(required, nil)
}

func requireAllPermissions(
	required []Permission,
	recorder audit.Recorder,
) func(http.Handler) http.Handler {
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

			for _, permission := range required {
				if !hasPermission(principal, permission) {
					recordAuditEvent(r.Context(), recorder, audit.Event{
						EventType:    audit.EventAuthorizationDenied,
						Outcome:      audit.OutcomeFailure,
						ActorUserID:  principal.UserID,
						RequestID:    audit.RequestID(r.Context()),
						ResourceType: "permission",
						Details:      audit.PermissionDetails(string(permission)),
					})
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAnyPermission(allowed ...Permission) func(http.Handler) http.Handler {
	return requireAnyPermission(allowed, nil)
}

func RequireAnyPermissionWithAudit(
	recorder audit.Recorder,
	allowed ...Permission,
) func(http.Handler) http.Handler {
	return requireAnyPermission(allowed, recorder)
}

func requireAnyPermission(
	allowed []Permission,
	recorder audit.Recorder,
) func(http.Handler) http.Handler {
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

			for _, permission := range allowed {
				if hasPermission(principal, permission) {
					next.ServeHTTP(w, r)
					return
				}
			}

			recordAuditEvent(r.Context(), recorder, audit.Event{
				EventType:    audit.EventAuthorizationDenied,
				Outcome:      audit.OutcomeFailure,
				ActorUserID:  principal.UserID,
				RequestID:    audit.RequestID(r.Context()),
				ResourceType: "permission",
				Details:      audit.PermissionDetails("any"),
			})
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

func hasPermission(principal Principal, required Permission) bool {
	for _, candidate := range principal.Permissions {
		if candidate == Permission("*") || candidate == required {
			return true
		}
	}
	return false
}
