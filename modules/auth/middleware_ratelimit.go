package auth

import (
	"math"
	"net"
	"net/http"
	"strconv"

	"github.com/Roy6lty/minibank/internal/ratelimit"
)

type RateLimitKey func(*http.Request, Principal) string
type PublicRateLimitKey func(*http.Request) string

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
