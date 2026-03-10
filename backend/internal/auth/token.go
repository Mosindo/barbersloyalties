package auth

import (
	"fmt"
	"time"

	"github.com/barbersloyalties/backend/internal/identity"
	"github.com/golang-jwt/jwt/v5"
)

type UserIdentity struct {
	UserID   string
	TenantID string
	Email    string
	Role     string
}

type TokenManager struct {
	secret []byte
	issuer string
	expiry time.Duration
}

func NewTokenManager(secret, issuer string, expiry time.Duration) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
		issuer: issuer,
		expiry: expiry,
	}
}

func (m *TokenManager) Generate(userIdentity UserIdentity) (string, error) {
	now := time.Now().UTC()
	claims := identity.Claims{
		UserID:   userIdentity.UserID,
		TenantID: userIdentity.TenantID,
		Email:    userIdentity.Email,
		Role:     userIdentity.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userIdentity.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (m *TokenManager) Parse(rawToken string) (*identity.Claims, error) {
	token, err := jwt.ParseWithClaims(rawToken, &identity.Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*identity.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	if claims.TenantID == "" || claims.UserID == "" {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}
