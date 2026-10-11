package services

import (
	"context"
	"time"
)

// LoginAttemptTracker protects a normalized, opaque login subject from
// repeated failed authentication attempts.
type LoginAttemptTracker interface {
	Check(context.Context, string) (time.Duration, error)
	RecordFailure(context.Context, string) (time.Duration, error)
	Reset(context.Context, string) error
}
