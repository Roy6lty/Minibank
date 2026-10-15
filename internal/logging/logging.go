package logging

import (
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
)

const redactedValue = "[REDACTED]"

var (
	sensitiveKeyPattern       = regexp.MustCompile(`(?i)((?:authorization|password|password_hash|secret|access_token|refresh_token|token|cookie|set_cookie|email|phone|account_number|customer_id|account_id|transfer_id|transaction_id|user_id|remote_addr|ip|ip_address)\s*[:=]\s*(?:bearer\s+)?)[^\s,;]+`)
	sqlSensitiveValuePattern  = regexp.MustCompile(`(?i)(\((?:email|phone|account_number|customer_id|account_id|transfer_id|transaction_id|user_id)\)\s*=\s*)\([^)]*\)`)
	jsonSensitiveValuePattern = regexp.MustCompile(`(?i)((?:"|')(?:authorization|password|password_hash|secret|access_token|refresh_token|token|cookie|set_cookie|email|phone|account_number|customer_id|account_id|transfer_id|transaction_id|user_id)(?:"|')\s*:\s*(?:"|'))[^"']*((?:"|'))`)
	emailPattern              = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	accountNumberPattern      = regexp.MustCompile(`\b\d{10,20}\b`)
)

func NewJSONLogger(writer io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	}))
}

func Error(err error) slog.Attr {
	return ErrorValue(err)
}

func ErrorValue(value any) slog.Attr {
	if value == nil {
		return slog.Group("error", slog.String("type", "<nil>"))
	}

	return slog.Group(
		"error",
		slog.String("type", fmt.Sprintf("%T", value)),
		slog.String("message", redactText(fmt.Sprint(value))),
	)
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}
	return attr
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	switch normalized {
	case "authorization", "password", "password_hash", "secret", "access_token", "refresh_token", "token", "cookie", "set_cookie", "body", "request_body", "headers", "query", "form":
		return true
	case "email", "phone", "account_number", "customer_id", "account_id", "transfer_id", "transaction_id", "user_id", "remote_addr", "ip", "ip_address":
		return true
	default:
		return false
	}
}

func redactText(value string) string {
	value = sensitiveKeyPattern.ReplaceAllString(value, "$1"+redactedValue)
	value = sqlSensitiveValuePattern.ReplaceAllString(value, "$1("+redactedValue+")")
	value = jsonSensitiveValuePattern.ReplaceAllString(value, "$1"+redactedValue+"$2")
	value = emailPattern.ReplaceAllString(value, redactedValue)
	return accountNumberPattern.ReplaceAllString(value, redactedValue)
}
