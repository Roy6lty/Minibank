package services

import (
	"testing"

	"github.com/Roy6lty/minibank/modules/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestLedgerAccountOwnedByAllowsOwnerAndAdmin(t *testing.T) {
	ownerID := uuid.New()
	owner := auth.Principal{CustomerID: ownerID, Role: "customer"}
	admin := auth.Principal{CustomerID: uuid.New(), Role: "admin"}
	customerID := pgtype.UUID{Bytes: ownerID, Valid: true}

	if !accountOwnedBy(owner, customerID) {
		t.Fatal("account owner should have access")
	}
	if accountOwnedBy(auth.Principal{CustomerID: uuid.New(), Role: "customer"}, customerID) {
		t.Fatal("different customer should not have access")
	}
	if !accountOwnedBy(admin, customerID) {
		t.Fatal("admin should have access")
	}
}
