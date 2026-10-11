package account

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(
	accountHandler *AccountHandler,
	lowRisk func(http.Handler) http.Handler,
	highRisk func(http.Handler) http.Handler,
) chi.Router {
	r := chi.NewRouter()

	r.With(highRisk).Post("/", accountHandler.CreateAccount)
	r.With(lowRisk).Get("/search", accountHandler.SearchAccounts)
	r.With(lowRisk).Get("/{accountNumber}", accountHandler.GetAccountByAccountNumber)
	r.With(highRisk).Patch("/{accountNumber}", accountHandler.UpdateAccount)

	return r
}
