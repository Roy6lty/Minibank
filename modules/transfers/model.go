package transfers

import "time"

type TransferStatus string

const (
	TransferStatusPending    TransferStatus = "PENDING"
	TransferStatusProcessing TransferStatus = "PROCESSING"
	TransferStatusCompleted  TransferStatus = "COMPLETED"
	TransferStatusFailed     TransferStatus = "FAILED"
	TransferStatusReversed   TransferStatus = "REVERSED"
)

type CreateTransferRequest struct {
	IdempotencyKey        string `json:"idempotency_key" validate:"required,max=100"`
	SenderAccountNumber   string `json:"sender_account_number" validate:"required,len=10,numeric"`
	RecieverAccountNumber string `json:"reciever_account_number" validate:"required,len=10,numeric"`
	Amount                int64  `json:"amount" validate:"gt=0"`
}

type GetTransferDetailsRequest struct {
	TransferID  *string `json:"transfer_id"`
	ReferenceID *string `json:"reference_id"`
}

type TransferRequestResponse struct {
	Reference   string    `json:"reference"`
	Amount      int64     `json:"amount"`
	Curreny     string    `json:"currency"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type UpdateTransferRequest struct {
	Status TransferStatus `json:"status" validate:"required,oneof=PENDING PROCESSING COMPLETED FAILED REVERSED"`
}
