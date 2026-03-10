package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type UserIdentity struct {
	UserID   string
	TenantID string
	Email    string
	Role     string
}

type Claims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
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

func (m *TokenManager) Generate(identity UserIdentity) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:   identity.UserID,
		TenantID: identity.TenantID,
		Email:    identity.Email,
		Role:     identity.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   identity.UserID,
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

func (m *TokenManager) Parse(rawToken string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(rawToken, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	if claims.TenantID == "" || claims.UserID == "" {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}
