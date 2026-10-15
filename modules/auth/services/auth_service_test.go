package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Roy6lty/minibank/database/repository"
	"github.com/Roy6lty/minibank/internal/audit"
	"github.com/Roy6lty/minibank/internal/ratelimit"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type unitOfWorkFunc func(context.Context, func(repository.Repositories) error) error

func (f unitOfWorkFunc) WithinTx(ctx context.Context, work func(repository.Repositories) error) error {
	return f(ctx, work)
}

type refreshTokenRepositoryStub struct {
	revokedHash string
	revokeErr   error
}

type auditRecorderStub struct {
	events []audit.Event
}

func (s *auditRecorderStub) Record(
	_ context.Context,
	event audit.Event,
) error {
	s.events = append(s.events, event)
	return nil
}

type userRepositoryStub struct {
	user       db.User
	err        error
	getByEmail int
}

func (s *userRepositoryStub) Create(context.Context, db.CreateUserParams) (db.User, error) {
	return s.user, s.err
}

func (s *userRepositoryStub) GetByEmail(context.Context, string) (db.User, error) {
	s.getByEmail++
	return s.user, s.err
}

func (s *userRepositoryStub) GetByID(context.Context, pgtype.UUID) (db.User, error) {
	return s.user, s.err
}

type loginAttemptTrackerStub struct {
	checkRetry  time.Duration
	recordRetry time.Duration
	checkErr    error
	recordErr   error
	resetErr    error
	checkCalls  int
	recordCalls int
	resetCalls  int
	lastKey     string
}

func (s *loginAttemptTrackerStub) Check(_ context.Context, key string) (time.Duration, error) {
	s.checkCalls++
	s.lastKey = key
	return s.checkRetry, s.checkErr
}

func (s *loginAttemptTrackerStub) RecordFailure(_ context.Context, key string) (time.Duration, error) {
	s.recordCalls++
	s.lastKey = key
	return s.recordRetry, s.recordErr
}

func (s *loginAttemptTrackerStub) Reset(_ context.Context, key string) error {
	s.resetCalls++
	s.lastKey = key
	return s.resetErr
}

type accessTokenIssuerStub struct{}

func (accessTokenIssuerStub) IssueAccessToken(authModels.Principal) (string, error) {
	return "access-token", nil
}

func (s *refreshTokenRepositoryStub) Create(context.Context, db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}

func (s *refreshTokenRepositoryStub) GetByHash(context.Context, string) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}

func (s *refreshTokenRepositoryStub) Revoke(_ context.Context, tokenHash string) error {
	s.revokedHash = tokenHash
	return s.revokeErr
}

func TestLogoutRevokesHashedRefreshToken(t *testing.T) {
	const refreshSecret = "refresh-secret"
	const refreshToken = "refresh-token"
	refreshTokens := &refreshTokenRepositoryStub{}
	uow := unitOfWorkFunc(func(ctx context.Context, work func(repository.Repositories) error) error {
		return work(repository.Repositories{RefreshTokens: refreshTokens})
	})
	service := NewAuthService(uow, nil, nil, 0, 0, refreshSecret)

	if err := service.Logout(context.Background(), refreshToken); err != nil {
		t.Fatalf("Logout returned an error: %v", err)
	}

	if refreshTokens.revokedHash != hashRefreshToken(refreshToken, refreshSecret) {
		t.Fatal("logout should revoke the HMAC hash of the refresh token")
	}
}

func TestLogoutRejectsEmptyRefreshToken(t *testing.T) {
	called := false
	uow := unitOfWorkFunc(func(ctx context.Context, work func(repository.Repositories) error) error {
		called = true
		return nil
	})
	service := NewAuthService(uow, nil, nil, 0, 0, "refresh-secret")

	if err := service.Logout(context.Background(), " "); !errors.Is(err, authModels.ErrInvalidRefreshToken) {
		t.Fatalf("expected invalid refresh token error, got %v", err)
	}
	if called {
		t.Fatal("logout should not open a transaction for an empty token")
	}
}

func TestLogoutReturnsRevokeError(t *testing.T) {
	expectedErr := errors.New("database unavailable")
	refreshTokens := &refreshTokenRepositoryStub{revokeErr: expectedErr}
	uow := unitOfWorkFunc(func(ctx context.Context, work func(repository.Repositories) error) error {
		return work(repository.Repositories{RefreshTokens: refreshTokens})
	})
	service := NewAuthService(uow, nil, nil, 0, 0, "refresh-secret")

	if err := service.Logout(context.Background(), "refresh-token"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected revoke error, got %v", err)
	}
}

func TestLoginEmailRateLimitKeyDoesNotExposeEmail(t *testing.T) {
	email := "customer@example.com"
	key := loginEmailRateLimitKey(email, "rate-limit-secret")

	if strings.Contains(key, email) {
		t.Fatal("login rate-limit key must not contain the raw email")
	}
	if key != loginEmailRateLimitKey(email, "rate-limit-secret") {
		t.Fatal("login rate-limit key must be deterministic")
	}
	if key == loginEmailRateLimitKey(email, "different-secret") {
		t.Fatal("login rate-limit key must depend on its secret")
	}
}

func TestCheckLoginRateLimitReturnsRetryAfter(t *testing.T) {
	service := &authService{
		loginLimit: LoginRateLimitConfig{
			Limiter: limiterFunc(func(_ context.Context, key string, policy ratelimit.Policy) (ratelimit.Decision, error) {
				if !strings.HasPrefix(key, "login-email:") {
					t.Fatalf("unexpected login rate-limit key: %s", key)
				}
				if policy.Name != "login-email" {
					t.Fatalf("unexpected login rate-limit policy: %s", policy.Name)
				}
				return ratelimit.Decision{RetryAfter: 2 * time.Second}, nil
			}),
			Policy: ratelimit.Policy{Name: "login-email", Capacity: 5, RefillPerSecond: 1},
			Secret: "rate-limit-secret",
		},
	}

	err := service.checkLoginRateLimit(context.Background(), "customer@example.com")
	var rateLimitErr *authModels.RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
	if rateLimitErr.RetryAfter != 2*time.Second {
		t.Fatalf("expected 2 second retry delay, got %s", rateLimitErr.RetryAfter)
	}
}

func TestLoginRecordsFailedAttempt(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	tracker := &loginAttemptTrackerStub{}
	users := &userRepositoryStub{
		user: db.User{
			Email:        "customer@example.com",
			PasswordHash: string(passwordHash),
			Status:       "ACTIVE",
		},
	}
	service := NewAuthService(
		nil,
		users,
		nil,
		0,
		0,
		"refresh-secret",
		LoginRateLimitConfig{AttemptTracker: tracker, Secret: "rate-limit-secret"},
	)

	_, err = service.Login(context.Background(), authModels.LoginRequest{
		Email:    "customer@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, authModels.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if tracker.checkCalls != 1 || tracker.recordCalls != 1 || tracker.resetCalls != 0 {
		t.Fatalf("unexpected tracker calls: check=%d record=%d reset=%d", tracker.checkCalls, tracker.recordCalls, tracker.resetCalls)
	}
	if strings.Contains(tracker.lastKey, "customer@example.com") {
		t.Fatal("login attempt tracker must not receive the raw email")
	}
}

func TestLoginRecordsFailedAuthenticationAuditEvent(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	auditRepo := &auditRecorderStub{}
	users := &userRepositoryStub{
		user: db.User{
			PasswordHash: string(passwordHash),
			Status:       "ACTIVE",
		},
	}
	service := NewAuthService(
		unitOfWorkFunc(func(ctx context.Context, work func(repository.Repositories) error) error {
			return work(repository.Repositories{Audit: auditRepo})
		}),
		users,
		nil,
		0,
		0,
		"refresh-secret",
	)

	ctx := audit.WithRequestID(context.Background(), "request-123")
	_, err = service.Login(ctx, authModels.LoginRequest{
		Email:    "customer@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, authModels.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if len(auditRepo.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(auditRepo.events))
	}
	event := auditRepo.events[0]
	if event.EventType != audit.EventAuthenticationFailed || event.Outcome != audit.OutcomeFailure {
		t.Fatalf("unexpected audit event: %+v", event)
	}
	if event.RequestID != "request-123" {
		t.Fatalf("request ID was not recorded: %q", event.RequestID)
	}
	if event.ActorUserID != uuid.Nil {
		t.Fatal("failed login must not identify an actor user")
	}
}

func TestLoginResetsFailedAttemptsAfterSuccess(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	tracker := &loginAttemptTrackerStub{}
	users := &userRepositoryStub{
		user: db.User{
			ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
			CustomerID:   pgtype.UUID{Bytes: uuid.New(), Valid: true},
			Email:        "customer@example.com",
			PasswordHash: string(passwordHash),
			Role:         "customer",
			Status:       "ACTIVE",
		},
	}
	service := NewAuthService(
		unitOfWorkFunc(func(ctx context.Context, work func(repository.Repositories) error) error {
			return work(repository.Repositories{RefreshTokens: &refreshTokenRepositoryStub{}})
		}),
		users,
		accessTokenIssuerStub{},
		time.Minute,
		time.Hour,
		"refresh-secret",
		LoginRateLimitConfig{AttemptTracker: tracker, Secret: "rate-limit-secret"},
	)

	response, err := service.Login(context.Background(), authModels.LoginRequest{
		Email:    "customer@example.com",
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("expected successful login, got %v", err)
	}
	if response.AccessToken != "access-token" {
		t.Fatalf("unexpected access token: %q", response.AccessToken)
	}
	if tracker.checkCalls != 1 || tracker.recordCalls != 0 || tracker.resetCalls != 1 {
		t.Fatalf("unexpected tracker calls: check=%d record=%d reset=%d", tracker.checkCalls, tracker.recordCalls, tracker.resetCalls)
	}
}

func TestLoginRejectsProtectedEmailBeforeUserLookup(t *testing.T) {
	tracker := &loginAttemptTrackerStub{checkRetry: 10 * time.Minute}
	users := &userRepositoryStub{}
	service := NewAuthService(
		nil,
		users,
		nil,
		0,
		0,
		"refresh-secret",
		LoginRateLimitConfig{AttemptTracker: tracker, Secret: "rate-limit-secret"},
	)

	_, err := service.Login(context.Background(), authModels.LoginRequest{
		Email:    "customer@example.com",
		Password: "password",
	})
	var rateLimitErr *authModels.RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
	if rateLimitErr.RetryAfter != 10*time.Minute {
		t.Fatalf("unexpected retry duration: %s", rateLimitErr.RetryAfter)
	}
	if users.getByEmail != 0 {
		t.Fatal("protected login should not query the user repository")
	}
}

func TestLoginReturnsLockoutAfterFailureThreshold(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	tracker := &loginAttemptTrackerStub{recordRetry: 15 * time.Minute}
	users := &userRepositoryStub{
		user: db.User{
			Email:        "customer@example.com",
			PasswordHash: string(passwordHash),
			Status:       "ACTIVE",
		},
	}
	service := NewAuthService(
		nil,
		users,
		nil,
		0,
		0,
		"refresh-secret",
		LoginRateLimitConfig{AttemptTracker: tracker, Secret: "rate-limit-secret"},
	)

	_, err = service.Login(context.Background(), authModels.LoginRequest{
		Email:    "customer@example.com",
		Password: "wrong-password",
	})
	var rateLimitErr *authModels.RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected lockout rate-limit error, got %v", err)
	}
	if users.getByEmail != 1 || tracker.recordCalls != 1 {
		t.Fatalf("unexpected lookup/tracker calls: lookup=%d record=%d", users.getByEmail, tracker.recordCalls)
	}
}

func TestLoginDoesNotRecordInfrastructureFailureAsCredentialFailure(t *testing.T) {
	tracker := &loginAttemptTrackerStub{}
	databaseErr := errors.New("database unavailable")
	users := &userRepositoryStub{err: databaseErr}
	service := NewAuthService(
		nil,
		users,
		nil,
		0,
		0,
		"refresh-secret",
		LoginRateLimitConfig{AttemptTracker: tracker, Secret: "rate-limit-secret"},
	)

	_, err := service.Login(context.Background(), authModels.LoginRequest{
		Email:    "customer@example.com",
		Password: "password",
	})
	if !errors.Is(err, databaseErr) {
		t.Fatalf("expected database error, got %v", err)
	}
	if tracker.recordCalls != 0 {
		t.Fatal("infrastructure failures must not be recorded as failed credentials")
	}
}

type limiterFunc func(context.Context, string, ratelimit.Policy) (ratelimit.Decision, error)

func (f limiterFunc) Allow(ctx context.Context, key string, policy ratelimit.Policy) (ratelimit.Decision, error) {
	return f(ctx, key, policy)
}
