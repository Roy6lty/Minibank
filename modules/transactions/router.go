package transactions

import (
	"github.com/go-chi/chi/v5"
)

func NewRouter(
	transactionHandler *TransactionHandler,
) chi.Router {
	r := chi.NewRouter()

	r.Post("/", transactionHandler.CreateTransaction)
	r.Get("/{transactionID}", transactionHandler.GetTransactionByID)
	r.Patch("/{transactionID}", transactionHandler.UpdateTransaction)

	return r
}
