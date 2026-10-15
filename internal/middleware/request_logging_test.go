package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Roy6lty/minibank/internal/logging"
	"github.com/Roy6lty/minibank/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func TestRequestLoggerEmitsSafeLifecycleFields(t *testing.T) {
	var output bytes.Buffer
	logger := logging.NewJSONLogger(&output, slog.LevelInfo)
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(logger))
	router.Get("/customers/{customerID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := router
	request := httptest.NewRequest(http.MethodGet, "/customers/customer-secret", nil)
	request.Header.Set("X-Request-ID", "request-123")
	request.RemoteAddr = "192.0.2.10:1234"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") != "request-123" {
		t.Fatalf("expected request ID response header, got %q", recorder.Header().Get("X-Request-ID"))
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("expected valid JSON log record: %v", err)
	}
	if record["method"] != http.MethodGet {
		t.Fatalf("expected method GET, got %v", record["method"])
	}
	if record["route"] != "/customers/{customerID}" {
		t.Fatalf("expected route pattern, got %v", record["route"])
	}
	if record["status"] != float64(http.StatusCreated) {
		t.Fatalf("expected status 201 in log, got %v", record["status"])
	}
	if record["request_id"] != "request-123" {
		t.Fatalf("expected request ID in log, got %v", record["request_id"])
	}
	if _, ok := record["remote_addr"]; ok {
		t.Fatal("request log must not include the remote address")
	}
}
