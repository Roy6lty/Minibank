package auth

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/Roy6lty/minibank/internal/httpx"
	appModels "github.com/Roy6lty/minibank/models"
)

type authService interface {
	Register(context.Context, RegisterRequest) (TokenResponse, error)
	Login(context.Context, LoginRequest) (TokenResponse, error)
	Refresh(context.Context, string) (TokenResponse, error)
	Logout(context.Context, string) error
}

type AuthHandler struct {
	service authService
}

func NewAuthHandler(service authService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var request RegisterRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}

	tokens, err := h.service.Register(r.Context(), request)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, appModels.APIResponse{
		Message: "registration successful",
		Success: true,
		Data:    tokens,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}

	tokens, err := h.service.Login(r.Context(), request)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "login successful",
		Success: true,
		Data:    tokens,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var request RefreshRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}

	tokens, err := h.service.Refresh(r.Context(), request.RefreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "token refreshed",
		Success: true,
		Data:    tokens,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var request RefreshRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}

	if err := h.service.Logout(r.Context(), request.RefreshToken); err != nil {
		writeAuthError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "logout successful",
		Success: true,
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var rateLimitErr *RateLimitError
	var rateLimitUnavailableErr *RateLimitUnavailableError
	if errors.As(err, &rateLimitUnavailableErr) {
		status = http.StatusServiceUnavailable
	} else if errors.As(err, &rateLimitErr) {
		retryAfter := int(math.Ceil(rateLimitErr.RetryAfter.Seconds()))
		if retryAfter < 1 {
			retryAfter = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		status = http.StatusTooManyRequests
	} else if errors.Is(err, ErrInvalidCredentials) ||
		errors.Is(err, ErrInvalidRefreshToken) {
		status = http.StatusUnauthorized
	} else if strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "invalid") ||
		strings.Contains(err.Error(), "password") {
		status = http.StatusBadRequest
	} else if strings.Contains(err.Error(), "duplicate") ||
		strings.Contains(err.Error(), "already exists") {
		status = http.StatusConflict
	}

	slog.Error("authentication request failed", "error", err)
	http.Error(w, http.StatusText(status), status)
}
