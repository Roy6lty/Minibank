package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokenManagerIssuesAndVerifiesAccessToken(t *testing.T) {
	manager, err := NewTokenManager("test-secret", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	principal := Principal{
		UserID:     uuid.New(),
		CustomerID: uuid.New(),
		Role:       "customer",
	}
	token, err := manager.IssueAccessToken(principal)
	if err != nil {
		t.Fatal(err)
	}

	verified, err := manager.VerifyAccessToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if verified.UserID != principal.UserID ||
		verified.CustomerID != principal.CustomerID ||
		verified.Role != principal.Role {
		t.Fatalf("verified principal does not match: got %+v", verified)
	}
	if len(verified.Permissions) == 0 {
		t.Fatal("expected role permissions in access token")
	}
}

func TestTokenManagerRejectsTamperedAccessToken(t *testing.T) {
	manager, err := NewTokenManager("test-secret", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	token, err := manager.IssueAccessToken(Principal{
		UserID:     uuid.New(),
		CustomerID: uuid.New(),
		Role:       "customer",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := manager.VerifyAccessToken(token + "tampered"); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}
