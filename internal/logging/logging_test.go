package logging_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/Roy6lty/minibank/internal/logging"
)

func TestNewJSONLoggerRedactsSensitiveAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := logging.NewJSONLogger(&output, slog.LevelInfo)

	logger.Info(
		"authentication event",
		"email", "customer@example.com",
		"password", "correct horse battery staple",
		"request_id", "request-123",
	)

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("expected valid JSON log record: %v", err)
	}
	if record["email"] != "[REDACTED]" {
		t.Fatalf("expected email to be redacted, got %v", record["email"])
	}
	if record["password"] != "[REDACTED]" {
		t.Fatalf("expected password to be redacted, got %v", record["password"])
	}
	if record["request_id"] != "request-123" {
		t.Fatalf("expected request ID to remain available, got %v", record["request_id"])
	}
}

func TestErrorAttributeDoesNotLeakSensitiveText(t *testing.T) {
	var output bytes.Buffer
	logger := logging.NewJSONLogger(&output, slog.LevelInfo)

	logger.Error(
		"request failed",
		logging.Error(errors.New("authorization=Bearer access-secret email=customer@example.com")),
	)

	for _, secret := range []string{"access-secret", "customer@example.com"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("log output leaked %q: %s", secret, output.String())
		}
	}
}
