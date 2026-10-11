package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(
	authHandler *AuthHandler,
	loginMiddleware ...func(http.Handler) http.Handler,
) chi.Router {
	r := chi.NewRouter()

	r.Post("/register", authHandler.Register)
	r.With(loginMiddleware...).Post("/login", authHandler.Login)
	r.Post("/refresh", authHandler.Refresh)
	r.Post("/logout", authHandler.Logout)

	return r
}
