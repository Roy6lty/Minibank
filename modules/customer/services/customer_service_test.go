package services

import (
	"testing"

	"github.com/Roy6lty/minibank/modules/auth"
	"github.com/google/uuid"
)

func TestCustomerOwnedByAllowsOwnerAndAdmin(t *testing.T) {
	customerID := uuid.New()
	owner := auth.Principal{CustomerID: customerID, Role: "customer"}
	admin := auth.Principal{CustomerID: uuid.New(), Role: "admin"}

	if !customerOwnedBy(owner, customerID) {
		t.Fatal("customer owner should have access")
	}
	if customerOwnedBy(owner, uuid.New()) {
		t.Fatal("different customer should not have access")
	}
	if !customerOwnedBy(admin, customerID) {
		t.Fatal("admin should have access")
	}
}
