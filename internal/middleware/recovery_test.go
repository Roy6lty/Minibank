package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverPassesThroughSuccessfulRequests(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

func TestRecoverReturnsGenericInternalServerError(t *testing.T) {
	const panicMessage = "database password leaked"
	handler := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(panicMessage)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/transfers", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	if got := recorder.Body.String(); got != "internal server error\n" {
		t.Fatalf("expected generic error response, got %q", got)
	}
	if strings.Contains(recorder.Body.String(), panicMessage) {
		t.Fatal("panic details must not be returned to the client")
	}
}
