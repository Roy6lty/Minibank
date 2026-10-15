package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/Roy6lty/minibank/database/repository"
	"github.com/Roy6lty/minibank/internal/audit"
	"github.com/Roy6lty/minibank/internal/ratelimit"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(context.Context, authModels.RegisterRequest) (authModels.TokenResponse, error)
	Login(context.Context, authModels.LoginRequest) (authModels.TokenResponse, error)
	Refresh(context.Context, string) (authModels.TokenResponse, error)
	Logout(context.Context, string) error
}

type accessTokenIssuer interface {
	IssueAccessToken(authModels.Principal) (string, error)
}

type LoginRateLimitConfig struct {
	Limiter        ratelimit.Limiter
	Policy         ratelimit.Policy
	Secret         string
	AttemptTracker LoginAttemptTracker
}

type authService struct {
	uow           repository.UnitOfWork
	users         repository.UserRepository
	tokens        accessTokenIssuer
	accessTTL     time.Duration
	refreshTTL    time.Duration
	refreshSecret string
	loginLimit    LoginRateLimitConfig
}

func NewAuthService(
	uow repository.UnitOfWork,
	users repository.UserRepository,
	tokens accessTokenIssuer,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	refreshSecret string,
	loginLimit ...LoginRateLimitConfig,
) AuthService {
	var configuredLoginLimit LoginRateLimitConfig
	if len(loginLimit) > 0 {
		configuredLoginLimit = loginLimit[0]
	}
	return &authService{
		uow:           uow,
		users:         users,
		tokens:        tokens,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		refreshSecret: refreshSecret,
		loginLimit:    configuredLoginLimit,
	}
}

func (a *authService) Register(
	ctx context.Context,
	request authModels.RegisterRequest,
) (authModels.TokenResponse, error) {
	email, err := normalizeEmail(request.Email)
	if err != nil {
		return authModels.TokenResponse{}, err
	}
	if strings.TrimSpace(request.FirstName) == "" ||
		strings.TrimSpace(request.LastName) == "" {
		return authModels.TokenResponse{}, fmt.Errorf("first and last name are required")
	}
	if err := validatePassword(request.Password); err != nil {
		return authModels.TokenResponse{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return authModels.TokenResponse{}, fmt.Errorf("hash password: %w", err)
	}

	var user db.User
	err = a.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		customer, err := repos.Customers.Create(ctx, db.CreateCustomerParams{
			FirstName: request.FirstName,
			LastName:  request.LastName,
			Email:     email,
			Phone: pgtype.Text{
				String: request.Phone,
				Valid:  request.Phone != "",
			},
		})
		if err != nil {
			return fmt.Errorf("create customer: %w", err)
		}

		user, err = repos.Users.Create(ctx, db.CreateUserParams{
			CustomerID:   customer.ID,
			Email:        email,
			PasswordHash: string(passwordHash),
			Role:         "customer",
			Status:       "ACTIVE",
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		if err := recordAuthAudit(ctx, repos.Audit, audit.Event{
			EventType:    audit.EventAuthRegister,
			Outcome:      audit.OutcomeSuccess,
			RequestID:    audit.RequestID(ctx),
			ResourceType: audit.ResourceUser,
			ResourceID:   uuid.UUID(user.ID.Bytes),
		}); err != nil {
			return fmt.Errorf("record registration: %w", err)
		}

		return nil
	})
	if err != nil {
		return authModels.TokenResponse{}, err
	}

	return a.issueSession(ctx, user, "")
}

func (a *authService) Login(
	ctx context.Context,
	request authModels.LoginRequest,
) (authModels.TokenResponse, error) {
	email, err := normalizeEmail(request.Email)
	if err != nil || request.Password == "" {
		return authModels.TokenResponse{}, authModels.ErrInvalidCredentials
	}
	if err := a.checkLoginRateLimit(ctx, email); err != nil {
		return authModels.TokenResponse{}, err
	}
	if err := a.checkLoginProtection(ctx, email); err != nil {
		return authModels.TokenResponse{}, err
	}

	user, err := a.users.GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return authModels.TokenResponse{}, fmt.Errorf("get user for login: %w", err)
		}
		if trackerErr := a.recordLoginFailure(ctx, email); trackerErr != nil {
			return authModels.TokenResponse{}, trackerErr
		}
		a.recordFailedLogin(ctx)
		return authModels.TokenResponse{}, authModels.ErrInvalidCredentials
	}
	if user.Status != "ACTIVE" {
		if trackerErr := a.recordLoginFailure(ctx, email); trackerErr != nil {
			return authModels.TokenResponse{}, trackerErr
		}
		a.recordFailedLogin(ctx)
		return authModels.TokenResponse{}, authModels.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)); err != nil {
		if trackerErr := a.recordLoginFailure(ctx, email); trackerErr != nil {
			return authModels.TokenResponse{}, trackerErr
		}
		a.recordFailedLogin(ctx)
		return authModels.TokenResponse{}, authModels.ErrInvalidCredentials
	}
	if err := a.resetLoginFailures(ctx, email); err != nil {
		return authModels.TokenResponse{}, err
	}

	return a.issueSession(ctx, user, audit.EventAuthLogin)
}

func (a *authService) checkLoginProtection(ctx context.Context, email string) error {
	if a.loginLimit.AttemptTracker == nil {
		return nil
	}
	retryAfter, err := a.loginLimit.AttemptTracker.Check(
		ctx,
		loginEmailRateLimitKey(email, a.loginLimit.Secret),
	)
	if err != nil {
		return &authModels.RateLimitUnavailableError{Err: fmt.Errorf("check failed login protection: %w", err)}
	}
	if retryAfter > 0 {
		return &authModels.RateLimitError{RetryAfter: retryAfter}
	}
	return nil
}

func (a *authService) recordLoginFailure(ctx context.Context, email string) error {
	if a.loginLimit.AttemptTracker == nil {
		return nil
	}
	retryAfter, err := a.loginLimit.AttemptTracker.RecordFailure(
		ctx,
		loginEmailRateLimitKey(email, a.loginLimit.Secret),
	)
	if err != nil {
		return &authModels.RateLimitUnavailableError{Err: fmt.Errorf("record failed login: %w", err)}
	}
	if retryAfter > 0 {
		return &authModels.RateLimitError{RetryAfter: retryAfter}
	}
	return nil
}

func (a *authService) resetLoginFailures(ctx context.Context, email string) error {
	if a.loginLimit.AttemptTracker == nil {
		return nil
	}
	if err := a.loginLimit.AttemptTracker.Reset(
		ctx,
		loginEmailRateLimitKey(email, a.loginLimit.Secret),
	); err != nil {
		return &authModels.RateLimitUnavailableError{Err: fmt.Errorf("reset failed login protection: %w", err)}
	}
	return nil
}

func (a *authService) checkLoginRateLimit(ctx context.Context, email string) error {
	if a.loginLimit.Limiter == nil {
		return nil
	}
	decision, err := a.loginLimit.Limiter.Allow(
		ctx,
		loginEmailRateLimitKey(email, a.loginLimit.Secret),
		a.loginLimit.Policy,
	)
	if err != nil {
		return &authModels.RateLimitUnavailableError{Err: fmt.Errorf("check login rate limit: %w", err)}
	}
	if !decision.Allowed {
		return &authModels.RateLimitError{RetryAfter: decision.RetryAfter}
	}
	return nil
}

func (a *authService) Refresh(
	ctx context.Context,
	refreshToken string,
) (authModels.TokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return authModels.TokenResponse{}, authModels.ErrInvalidRefreshToken
	}

	tokenHash := hashRefreshToken(refreshToken, a.refreshSecret)
	var user db.User
	newRefreshToken, newRefreshHash, err := generateRefreshToken(a.refreshSecret)
	if err != nil {
		return authModels.TokenResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}

	err = a.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		stored, err := repos.RefreshTokens.GetByHash(ctx, tokenHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return authModels.ErrInvalidRefreshToken
			}
			return fmt.Errorf("get refresh token: %w", err)
		}

		user, err = repos.Users.GetByID(ctx, stored.UserID)
		if err != nil || user.Status != "ACTIVE" {
			return authModels.ErrInvalidRefreshToken
		}

		if _, err := repos.RefreshTokens.Create(ctx, db.CreateRefreshTokenParams{
			UserID:    user.ID,
			TokenHash: newRefreshHash,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(a.refreshTTL), Valid: true},
		}); err != nil {
			return fmt.Errorf("store refresh token: %w", err)
		}

		if err := repos.RefreshTokens.Revoke(ctx, tokenHash); err != nil {
			return fmt.Errorf("revoke refresh token: %w", err)
		}

		if err := recordAuthAudit(ctx, repos.Audit, audit.Event{
			EventType:    audit.EventAuthRefresh,
			Outcome:      audit.OutcomeSuccess,
			ActorUserID:  uuid.UUID(user.ID.Bytes),
			RequestID:    audit.RequestID(ctx),
			ResourceType: audit.ResourceUser,
			ResourceID:   uuid.UUID(user.ID.Bytes),
		}); err != nil {
			return fmt.Errorf("record refresh: %w", err)
		}

		return nil
	})
	if err != nil {
		return authModels.TokenResponse{}, err
	}

	accessToken, err := a.tokens.IssueAccessToken(principalFromUser(user))
	if err != nil {
		return authModels.TokenResponse{}, fmt.Errorf("issue access token: %w", err)
	}

	return authModels.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(a.accessTTL.Seconds()),
	}, nil
}

func (a *authService) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return authModels.ErrInvalidRefreshToken
	}

	tokenHash := hashRefreshToken(refreshToken, a.refreshSecret)
	if err := a.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		if err := repos.RefreshTokens.Revoke(ctx, tokenHash); err != nil {
			return err
		}
		return recordAuthAudit(ctx, repos.Audit, audit.Event{
			EventType:    audit.EventAuthLogout,
			Outcome:      audit.OutcomeSuccess,
			RequestID:    audit.RequestID(ctx),
			ResourceType: audit.ResourceSession,
		})
	}); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	return nil
}

func (a *authService) issueSession(
	ctx context.Context,
	user db.User,
	eventType string,
) (authModels.TokenResponse, error) {
	refreshToken, refreshHash, err := generateRefreshToken(a.refreshSecret)
	if err != nil {
		return authModels.TokenResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}

	err = a.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		_, err := repos.RefreshTokens.Create(ctx, db.CreateRefreshTokenParams{
			UserID:    user.ID,
			TokenHash: refreshHash,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(a.refreshTTL), Valid: true},
		})
		if err != nil {
			return err
		}
		if eventType == "" {
			return nil
		}
		return recordAuthAudit(ctx, repos.Audit, audit.Event{
			EventType:    eventType,
			Outcome:      audit.OutcomeSuccess,
			ActorUserID:  uuid.UUID(user.ID.Bytes),
			RequestID:    audit.RequestID(ctx),
			ResourceType: audit.ResourceUser,
			ResourceID:   uuid.UUID(user.ID.Bytes),
		})
	})
	if err != nil {
		return authModels.TokenResponse{}, fmt.Errorf("store refresh token: %w", err)
	}

	accessToken, err := a.tokens.IssueAccessToken(principalFromUser(user))
	if err != nil {
		return authModels.TokenResponse{}, fmt.Errorf("issue access token: %w", err)
	}

	return authModels.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(a.accessTTL.Seconds()),
	}, nil
}

func recordAuthAudit(
	ctx context.Context,
	recorder audit.Recorder,
	event audit.Event,
) error {
	if recorder == nil {
		return nil
	}
	return recorder.Record(ctx, event)
}

func (a *authService) recordFailedLogin(ctx context.Context) {
	if a.uow == nil {
		return
	}
	if err := a.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		return recordAuthAudit(ctx, repos.Audit, audit.Event{
			EventType: audit.EventAuthenticationFailed,
			Outcome:   audit.OutcomeFailure,
			RequestID: audit.RequestID(ctx),
		})
	}); err != nil {
		slog.ErrorContext(ctx, "record failed login audit event", "error", err)
	}
}

func principalFromUser(user db.User) authModels.Principal {
	return authModels.Principal{
		UserID:      uuid.UUID(user.ID.Bytes),
		CustomerID:  uuid.UUID(user.CustomerID.Bytes),
		Role:        user.Role,
		Permissions: authModels.PermissionsForRole(user.Role),
	}
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", fmt.Errorf("invalid email")
	}
	return email, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

func generateRefreshToken(secret string) (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}

	plain := base64.RawURLEncoding.EncodeToString(raw)
	return plain, hashRefreshToken(plain, secret), nil
}

func hashRefreshToken(token, secret string) string {
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write([]byte(token))
	return hex.EncodeToString(digest.Sum(nil))
}

func loginEmailRateLimitKey(email, secret string) string {
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write([]byte(email))
	return "login-email:" + hex.EncodeToString(digest.Sum(nil))
}
