package transactions

type TransactionsStatus string

const (
	TransactionsStatusPending    TransactionsStatus = "PENDING"
	TransactionsStatusProcessing TransactionsStatus = "PROCESSING"
	TransactionsStatusCompleted  TransactionsStatus = "COMPLETED"
	TransactionsStatusFailed     TransactionsStatus = "FAILED"
	TransactionsStatusReversed   TransactionsStatus = "REVERSED"
)

type CreateTransactionRequest struct {
	Reference   string `json:"reference"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
}

type GetTransactionsRequest struct {
	ID *string `json:"id"`
}

type UpdateTransactionRequest struct {
	Status *TransactionsStatus `json:"status"`
}
