package auth

import (
	"testing"
	"time"
)

func TestTokenManagerGenerateAndParse(t *testing.T) {
	manager := NewTokenManager("test-secret", "test-issuer", time.Hour)

	token, err := manager.Generate(UserIdentity{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Email:    "owner@example.com",
		Role:     "owner",
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Fatalf("expected user id user-1, got %s", claims.UserID)
	}
	if claims.TenantID != "tenant-1" {
		t.Fatalf("expected tenant id tenant-1, got %s", claims.TenantID)
	}
	if claims.Email != "owner@example.com" {
		t.Fatalf("expected email owner@example.com, got %s", claims.Email)
	}
}

func TestTokenManagerParseInvalidToken(t *testing.T) {
	manager := NewTokenManager("test-secret", "test-issuer", time.Hour)
	if _, err := manager.Parse("not-a-token"); err == nil {
		t.Fatal("expected parse error for invalid token")
	}
}
