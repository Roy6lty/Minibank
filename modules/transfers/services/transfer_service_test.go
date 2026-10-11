package services

import (
	"testing"

	accountModels "github.com/Roy6lty/minibank/modules/account"
	"github.com/Roy6lty/minibank/modules/auth"
	transferModels "github.com/Roy6lty/minibank/modules/transfers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestTransferAccountOwnedByAllowsOwnerAndAdmin(t *testing.T) {
	ownerID := uuid.New()
	account := accountModels.TransferAccount{
		CustomerID: pgtype.UUID{Bytes: ownerID, Valid: true},
	}
	owner := auth.Principal{CustomerID: ownerID, Role: "customer"}
	admin := auth.Principal{CustomerID: uuid.New(), Role: "admin"}

	if !accountOwnedBy(owner, account) {
		t.Fatal("account owner should have access")
	}
	if accountOwnedBy(auth.Principal{CustomerID: uuid.New(), Role: "customer"}, account) {
		t.Fatal("different customer should not have access")
	}
	if !accountOwnedBy(admin, account) {
		t.Fatal("admin should have access")
	}
}

func TestCanTransitionTransferStatus(t *testing.T) {
	tests := []struct {
		name    string
		current transferModels.TransferStatus
		next    transferModels.TransferStatus
		allowed bool
	}{
		{
			name:    "pending to failed",
			current: transferModels.TransferStatusPending,
			next:    transferModels.TransferStatusFailed,
			allowed: true,
		},
		{
			name:    "pending to completed is internal only",
			current: transferModels.TransferStatusPending,
			next:    transferModels.TransferStatusCompleted,
			allowed: false,
		},
		{
			name:    "pending to processing is deferred",
			current: transferModels.TransferStatusPending,
			next:    transferModels.TransferStatusProcessing,
			allowed: false,
		},
		{
			name:    "completed is terminal",
			current: transferModels.TransferStatusCompleted,
			next:    transferModels.TransferStatusFailed,
			allowed: false,
		},
		{
			name:    "failed is terminal",
			current: transferModels.TransferStatusFailed,
			next:    transferModels.TransferStatusPending,
			allowed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := canTransitionTransferStatus(test.current, test.next); got != test.allowed {
				t.Fatalf("expected transition allowed=%t, got %t", test.allowed, got)
			}
		})
	}
}
