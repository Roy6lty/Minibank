package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Roy6lty/minibank/internal/ratelimit"
	"github.com/google/uuid"
)

type verifierFunc func(string) (Principal, error)

type limiterFunc func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error)

func (f limiterFunc) Allow(ctx context.Context, key string, policy ratelimit.Policy) (ratelimit.Decision, error) {
	return f(ctx, key, policy)
}

func (f verifierFunc) VerifyAccessToken(token string) (Principal, error) {
	return f(token)
}

func TestRequireAuthenticationRejectsMissingHeader(t *testing.T) {
	nextCalled := false
	handler := RequireAuthentication(verifierFunc(func(string) (Principal, error) {
		t.Fatal("verifier should not be called")
		return Principal{}, nil
	}))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	if nextCalled {
		t.Fatal("next handler should not be called")
	}
}

func TestRequireAuthenticationAddsPrincipalToContext(t *testing.T) {
	expected := Principal{UserID: uuid.New(), CustomerID: uuid.New(), Role: "customer"}
	handler := RequireAuthentication(verifierFunc(func(token string) (Principal, error) {
		if token != "valid" {
			t.Fatalf("unexpected token: %s", token)
		}
		return expected, nil
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual, ok := PrincipalFromContext(r.Context())
		if !ok || actual.UserID != expected.UserID {
			t.Fatal("principal was not added to request context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer valid")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

func TestRequirePermissionRejectsUnauthorizedPrincipal(t *testing.T) {
	handler := RequirePermission("accounts:access")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), Principal{
		Role:        "customer",
		Permissions: []Permission{PermissionLedgerAccess},
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestRequireAllPermissionsRequiresEveryPermission(t *testing.T) {
	handler := RequireAllPermissions(
		PermissionAccountsAccess,
		PermissionTransfersAccess,
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), Principal{
		Permissions: []Permission{PermissionAccountsAccess},
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestRequireAnyPermissionAcceptsOneMatchingPermission(t *testing.T) {
	handler := RequireAnyPermission(
		PermissionAccountsAccess,
		PermissionTransfersAccess,
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), Principal{
		Permissions: []Permission{PermissionTransfersAccess},
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

func TestRequireAuthenticationRejectsInvalidToken(t *testing.T) {
	handler := RequireAuthentication(verifierFunc(func(string) (Principal, error) {
		return Principal{}, errors.New("invalid token")
	}))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestRequireActiveSessionRejectsInactiveUser(t *testing.T) {
	nextCalled := false
	principal := Principal{UserID: uuid.New(), CustomerID: uuid.New()}
	handler := RequireActiveSession(func(ctx context.Context, got Principal) (bool, error) {
		if got.UserID != principal.UserID {
			t.Fatal("unexpected principal passed to session checker")
		}
		return false, nil
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), principal))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	if nextCalled {
		t.Fatal("next handler should not be called")
	}
}

func TestRequireActiveSessionAllowsActiveUser(t *testing.T) {
	handler := RequireActiveSession(func(context.Context, Principal) (bool, error) {
		return true, nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), Principal{
		UserID:     uuid.New(),
		CustomerID: uuid.New(),
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

func TestRequireActiveSessionReturnsUnavailableWhenCheckFails(t *testing.T) {
	handler := RequireActiveSession(func(context.Context, Principal) (bool, error) {
		return false, errors.New("database unavailable")
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), Principal{
		UserID:     uuid.New(),
		CustomerID: uuid.New(),
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}

func TestRequireRateLimitAllowsRequest(t *testing.T) {
	principal := Principal{UserID: uuid.New(), CustomerID: uuid.New()}
	called := false
	handler := RequireRateLimit(
		limiterFunc(func(_ context.Context, key string, policy ratelimit.Policy) (ratelimit.Decision, error) {
			called = true
			if key != "user:"+principal.UserID.String() {
				t.Fatalf("unexpected rate-limit key: %s", key)
			}
			if policy.Name != "read" {
				t.Fatalf("unexpected rate-limit policy: %s", policy.Name)
			}
			return ratelimit.Decision{Allowed: true}, nil
		}),
		ratelimit.Policy{Name: "read", Capacity: 60, RefillPerSecond: 1},
		UserRateLimitKey,
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), principal))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent || !called {
		t.Fatal("rate-limited request should reach the next handler")
	}
}

func TestRequireRateLimitRejectsRequestWithRetryAfter(t *testing.T) {
	handler := RequireRateLimit(
		limiterFunc(func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error) {
			return ratelimit.Decision{RetryAfter: 1500 * time.Millisecond}, nil
		}),
		ratelimit.Policy{Name: "high-risk", Capacity: 5, RefillPerSecond: 0.1},
		UserRateLimitKey,
	)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("rate-limited request should not reach the next handler")
	}))

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), Principal{
		UserID:     uuid.New(),
		CustomerID: uuid.New(),
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "2" {
		t.Fatalf("expected Retry-After 2, got %q", recorder.Header().Get("Retry-After"))
	}
}

func TestRequireRateLimitReturnsUnavailableWhenLimiterFails(t *testing.T) {
	handler := RequireRateLimit(
		limiterFunc(func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error) {
			return ratelimit.Decision{}, errors.New("Redis unavailable")
		}),
		ratelimit.Policy{Name: "read", Capacity: 60, RefillPerSecond: 1},
		UserRateLimitKey,
	)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("limiter failure should stop the request")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(withPrincipal(request.Context(), Principal{
		UserID:     uuid.New(),
		CustomerID: uuid.New(),
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}

func TestRequirePublicRateLimitAllowsRequest(t *testing.T) {
	called := false
	handler := RequirePublicRateLimit(
		limiterFunc(func(_ context.Context, key string, policy ratelimit.Policy) (ratelimit.Decision, error) {
			called = true
			if key != "public-key" {
				t.Fatalf("unexpected rate-limit key: %s", key)
			}
			if policy.Name != "login" {
				t.Fatalf("unexpected rate-limit policy: %s", policy.Name)
			}
			return ratelimit.Decision{Allowed: true}, nil
		}),
		ratelimit.Policy{Name: "login", Capacity: 20, RefillPerSecond: 1},
		func(*http.Request) string { return "public-key" },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent || !called {
		t.Fatal("public rate-limited request should reach the next handler")
	}
}

func withPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}
