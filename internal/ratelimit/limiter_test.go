package ratelimit

import (
	"testing"
	"time"
)

func TestNewLoginAttemptTrackerRejectsInvalidPolicy(t *testing.T) {
	limiter := &RedisLimiter{}
	cases := []struct {
		name   string
		policy LoginAttemptPolicy
	}{
		{
			name: "zero max failures",
			policy: LoginAttemptPolicy{
				FailureWindow:   time.Minute,
				LockoutDuration: time.Minute,
			},
		},
		{
			name: "zero failure window",
			policy: LoginAttemptPolicy{
				MaxFailures:     5,
				LockoutDuration: time.Minute,
			},
		},
		{
			name: "zero lockout duration",
			policy: LoginAttemptPolicy{
				MaxFailures:   5,
				FailureWindow: time.Minute,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := limiter.NewLoginAttemptTracker(test.policy); err == nil {
				t.Fatal("expected invalid login attempt policy error")
			}
		})
	}
}

func TestNewLoginAttemptTrackerAcceptsValidPolicy(t *testing.T) {
	limiter := &RedisLimiter{}
	policy := LoginAttemptPolicy{
		MaxFailures:     5,
		FailureWindow:   15 * time.Minute,
		LockoutDuration: 15 * time.Minute,
	}

	tracker, err := limiter.NewLoginAttemptTracker(policy)
	if err != nil {
		t.Fatalf("expected valid policy, got %v", err)
	}
	if tracker.policy != policy {
		t.Fatalf("tracker stored unexpected policy: %+v", tracker.policy)
	}
}
