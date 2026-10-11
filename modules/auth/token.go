package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenVerifier interface {
	VerifyAccessToken(string) (Principal, error)
}

type TokenManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

type accessClaims struct {
	UserID      string   `json:"uid"`
	CustomerID  string   `json:"cid"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	TokenType   string   `json:"token_type"`
	jwt.RegisteredClaims
}

func NewTokenManager(secret string, accessTTL time.Duration) (*TokenManager, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("access token secret is required")
	}
	if accessTTL <= 0 {
		return nil, fmt.Errorf("access token lifetime must be greater than zero")
	}

	return &TokenManager{
		secret:    []byte(secret),
		issuer:    "minibank",
		accessTTL: accessTTL,
	}, nil
}

func (m *TokenManager) IssueAccessToken(principal Principal) (string, error) {
	if principal.UserID == uuid.Nil || principal.CustomerID == uuid.Nil {
		return "", fmt.Errorf("principal IDs are required")
	}

	permissions := principal.Permissions
	if len(permissions) == 0 {
		permissions = PermissionsForRole(principal.Role)
	}
	permissionClaims := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		permissionClaims = append(permissionClaims, string(permission))
	}

	now := time.Now()
	claims := accessClaims{
		UserID:      principal.UserID.String(),
		CustomerID:  principal.CustomerID.String(),
		Role:        principal.Role,
		Permissions: permissionClaims,
		TokenType:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   principal.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *TokenManager) VerifyAccessToken(rawToken string) (Principal, error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !token.Valid || claims.TokenType != "access" {
		return Principal{}, fmt.Errorf("invalid access token")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return Principal{}, fmt.Errorf("invalid access token subject")
	}
	customerID, err := uuid.Parse(claims.CustomerID)
	if err != nil {
		return Principal{}, fmt.Errorf("invalid access token customer")
	}

	permissions := make([]Permission, 0, len(claims.Permissions))
	for _, permission := range claims.Permissions {
		permissions = append(permissions, Permission(permission))
	}

	return Principal{
		UserID:      userID,
		CustomerID:  customerID,
		Role:        claims.Role,
		Permissions: permissions,
	}, nil
}

func PermissionsForRole(role string) []Permission {
	switch strings.ToLower(role) {
	case "admin":
		return []Permission{"*"}
	case "customer":
		return []Permission{
			PermissionCustomersAccess,
			PermissionAccountsAccess,
			PermissionTransfersAccess,
			PermissionLedgerAccess,
		}
	default:
		return nil
	}
}
