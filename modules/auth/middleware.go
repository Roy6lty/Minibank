package auth

import (
	"context"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/Roy6lty/minibank/internal/ratelimit"
)

type ActiveSessionChecker func(context.Context, Principal) (bool, error)
type RateLimitKey func(*http.Request, Principal) string
type PublicRateLimitKey func(*http.Request) string

type principalContextKey struct{}

func RequireAuthentication(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			principal, err := verifier.VerifyAccessToken(parts[1])
			if err != nil {
				http.Error(w, "invalid access token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequirePermission(permission Permission) func(http.Handler) http.Handler {
	return RequireAllPermissions(permission)
}

func RequireActiveSession(checker ActiveSessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			active, err := checker(r.Context(), principal)
			if err != nil {
				http.Error(w, "session status unavailable", http.StatusServiceUnavailable)
				return
			}
			if !active {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireRateLimit(
	limiter ratelimit.Limiter,
	policy ratelimit.Policy,
	keys ...RateLimitKey,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			requestKeys := make([]string, 0, len(keys))
			for _, keyFunc := range keys {
				requestKeys = append(requestKeys, keyFunc(r, principal))
			}
			if !allowRateLimitedRequest(w, r, limiter, policy, requestKeys...) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequirePublicRateLimit(
	limiter ratelimit.Limiter,
	policy ratelimit.Policy,
	keys ...PublicRateLimitKey,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestKeys := make([]string, 0, len(keys))
			for _, keyFunc := range keys {
				requestKeys = append(requestKeys, keyFunc(r))
			}
			if !allowRateLimitedRequest(w, r, limiter, policy, requestKeys...) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func allowRateLimitedRequest(
	w http.ResponseWriter,
	r *http.Request,
	limiter ratelimit.Limiter,
	policy ratelimit.Policy,
	keys ...string,
) bool {
	for _, key := range keys {
		decision, err := limiter.Allow(r.Context(), key, policy)
		if err != nil {
			http.Error(w, "rate limit unavailable", http.StatusServiceUnavailable)
			return false
		}
		if !decision.Allowed {
			retryAfter := int(math.Ceil(decision.RetryAfter.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return false
		}
	}
	return true
}

func UserRateLimitKey(_ *http.Request, principal Principal) string {
	return "user:" + principal.UserID.String()
}

func IPRateLimitKey(request *http.Request, _ Principal) string {
	return publicIPRateLimitKey(request)
}

func PublicIPRateLimitKey(request *http.Request) string {
	return publicIPRateLimitKey(request)
}

func publicIPRateLimitKey(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return "ip:" + host
	}
	return "ip:" + request.RemoteAddr
}

func RequireAllPermissions(required ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			for _, permission := range required {
				if !hasPermission(principal, permission) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAnyPermission(allowed ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			for _, permission := range allowed {
				if hasPermission(principal, permission) {
					next.ServeHTTP(w, r)
					return
				}
			}

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

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}
