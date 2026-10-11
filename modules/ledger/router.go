package ledger

import "github.com/go-chi/chi/v5"

func NewRouter(ledgerHandler *LedgerHandler) chi.Router {
	r := chi.NewRouter()

	r.Get("/accounts/{accountID}/balance", ledgerHandler.GetBalance)
	r.Get("/accounts/{accountID}/entries", ledgerHandler.GetEntriesByAccount)
	r.Get("/transfers/{transferID}/entries", ledgerHandler.GetEntriesByTransfer)
	r.Get("/entries/{entryID}", ledgerHandler.GetEntryByID)

	return r
}
