package services

import (
	"testing"

	accountModels "github.com/Roy6lty/minibank/modules/account"
	"github.com/Roy6lty/minibank/modules/auth"
	"github.com/google/uuid"
)

func TestAccountOwnedByAllowsOwnerAndAdmin(t *testing.T) {
	accountOwner := uuid.New()
	owner := auth.Principal{CustomerID: accountOwner, Role: "customer"}
	admin := auth.Principal{CustomerID: uuid.New(), Role: "admin"}

	if !accountOwnedBy(owner, accountOwner) {
		t.Fatal("account owner should have access")
	}
	if accountOwnedBy(owner, uuid.New()) {
		t.Fatal("different customer should not have access")
	}
	if !accountOwnedBy(admin, accountOwner) {
		t.Fatal("admin should have access")
	}
}

func TestCanTransitionAccountStatus(t *testing.T) {
	tests := []struct {
		name    string
		current accountModels.AccountStatus
		next    accountModels.AccountStatus
		allowed bool
	}{
		{
			name:    "active to frozen",
			current: accountModels.AccountStatusActive,
			next:    accountModels.AccountStatusFrozen,
			allowed: true,
		},
		{
			name:    "frozen to active",
			current: accountModels.AccountStatusFrozen,
			next:    accountModels.AccountStatusActive,
			allowed: true,
		},
		{
			name:    "active to closed",
			current: accountModels.AccountStatusActive,
			next:    accountModels.AccountStatusClosed,
			allowed: true,
		},
		{
			name:    "frozen to closed",
			current: accountModels.AccountStatusFrozen,
			next:    accountModels.AccountStatusClosed,
			allowed: true,
		},
		{
			name:    "closed to active",
			current: accountModels.AccountStatusClosed,
			next:    accountModels.AccountStatusActive,
			allowed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := canTransitionAccountStatus(test.current, test.next); got != test.allowed {
				t.Fatalf("expected transition allowed=%t, got %t", test.allowed, got)
			}
		})
	}
}

func TestValidAccountType(t *testing.T) {
	if !validAccountType(accountModels.AccountTypeSavings) {
		t.Fatal("savings should be a valid account type")
	}
	if !validAccountType(accountModels.AccountTypeCurrent) {
		t.Fatal("current should be a valid account type")
	}
	if validAccountType(accountModels.AccountType("INVESTMENT")) {
		t.Fatal("unexpected account type should be invalid")
	}
}

func TestValidCurrency(t *testing.T) {
	for _, currency := range []accountModels.Currency{
		accountModels.CurrencyNGN,
		accountModels.CurrencyUSD,
		accountModels.CurrencyGBP,
		accountModels.CurrencyEUR,
	} {
		if !validCurrency(currency) {
			t.Fatalf("%s should be a valid currency", currency)
		}
	}
	if validCurrency(accountModels.Currency("CAD")) {
		t.Fatal("unsupported currency should be invalid")
	}
}
