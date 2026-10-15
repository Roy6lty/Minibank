package transactions

type TransactionsStatus string

const TransactionTypeTransfer = "TRANSFER"

const (
	TransactionsStatusPending    TransactionsStatus = "PENDING"
	TransactionsStatusProcessing TransactionsStatus = "PROCESSING"
	TransactionsStatusCompleted  TransactionsStatus = "COMPLETED"
	TransactionsStatusFailed     TransactionsStatus = "FAILED"
	TransactionsStatusReversed   TransactionsStatus = "REVERSED"
)

type CreateTransactionRequest struct {
	Reference   string `json:"reference" validate:"required,max=100"`
	Type        string `json:"type" validate:"required,oneof=TRANSFER DEPOSIT WITHDRAWAL FEE INTEREST REVERSAL"`
	Status      string `json:"status" validate:"omitempty,oneof=PENDING PROCESSING COMPLETED FAILED REVERSED"`
	Amount      int64  `json:"amount" validate:"gt=0"`
	Currency    string `json:"currency" validate:"required,len=3"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

type GetTransactionsRequest struct {
	ID *string `json:"id"`
}

type UpdateTransactionRequest struct {
	Status *TransactionsStatus `json:"status" validate:"required,oneof=PENDING PROCESSING COMPLETED FAILED REVERSED"`
}
