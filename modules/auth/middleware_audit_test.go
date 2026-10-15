package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Roy6lty/minibank/internal/audit"
	"github.com/google/uuid"
)

type auditRecorderStub struct {
	event audit.Event
}

func (s *auditRecorderStub) Record(_ context.Context, event audit.Event) error {
	s.event = event
	return nil
}

func TestRequireAuthenticationRecordsInvalidToken(t *testing.T) {
	recorder := &auditRecorderStub{}
	handler := RequireAuthentication(verifierFunc(func(string) (Principal, error) {
		return Principal{}, errors.New("invalid token")
	}), recorder)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid token should not reach the next handler")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(audit.WithRequestID(request.Context(), "request-123"))
	request.Header.Set("Authorization", "Bearer invalid")
	recorderHTTP := httptest.NewRecorder()
	handler.ServeHTTP(recorderHTTP, request)

	if recorderHTTP.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorderHTTP.Code)
	}
	if recorder.event.EventType != audit.EventAuthenticationFailed || recorder.event.Outcome != audit.OutcomeFailure {
		t.Fatalf("unexpected audit event: %+v", recorder.event)
	}
	if recorder.event.RequestID != "request-123" {
		t.Fatalf("expected request ID to be recorded, got %q", recorder.event.RequestID)
	}
}

func TestRequirePermissionRecordsDeniedPermission(t *testing.T) {
	recorder := &auditRecorderStub{}
	actorID := uuid.New()
	handler := RequirePermission(PermissionAccountsAccess, recorder)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("forbidden request should not reach the next handler")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := audit.WithRequestID(request.Context(), "request-456")
	request = request.WithContext(withPrincipal(ctx, Principal{
		UserID:      actorID,
		Permissions: []Permission{PermissionLedgerAccess},
	}))
	recorderHTTP := httptest.NewRecorder()
	handler.ServeHTTP(recorderHTTP, request)

	if recorderHTTP.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorderHTTP.Code)
	}
	if recorder.event.EventType != audit.EventAuthorizationDenied || recorder.event.ActorUserID != actorID {
		t.Fatalf("unexpected audit event: %+v", recorder.event)
	}
	if recorder.event.ResourceType != "permission" || recorder.event.RequestID != "request-456" {
		t.Fatalf("unexpected audit context: %+v", recorder.event)
	}
}
