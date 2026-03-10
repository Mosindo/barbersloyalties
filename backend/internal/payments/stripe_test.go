package payments

import (
	"testing"
	"time"
)

func TestVerifyStripeSignatureValid(t *testing.T) {
	payload := []byte(`{"id":"evt_1","type":"checkout.session.completed","data":{"object":{}}}`)
	secret := "whsec_test"
	now := time.Now().UTC()
	header := BuildStripeSignatureHeader(payload, secret, now.Unix())

	if err := VerifyStripeSignature(payload, header, secret, now, 5*time.Minute); err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}
}

func TestVerifyStripeSignatureMismatch(t *testing.T) {
	payload := []byte(`{"id":"evt_1"}`)
	secret := "whsec_test"
	now := time.Now().UTC()
	header := BuildStripeSignatureHeader(payload, secret, now.Unix())

	if err := VerifyStripeSignature(payload, header, "wrong_secret", now, 5*time.Minute); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestVerifyStripeSignatureOutsideTolerance(t *testing.T) {
	payload := []byte(`{"id":"evt_1"}`)
	secret := "whsec_test"
	now := time.Now().UTC()
	header := BuildStripeSignatureHeader(payload, secret, now.Add(-10*time.Minute).Unix())

	if err := VerifyStripeSignature(payload, header, secret, now, 5*time.Minute); err == nil {
		t.Fatal("expected tolerance error")
	}
}

func TestCalculatePlatformFeeCents(t *testing.T) {
	fee := calculatePlatformFeeCents(10000, 500)
	if fee != 500 {
		t.Fatalf("expected 500, got %d", fee)
	}

	zero := calculatePlatformFeeCents(10000, 0)
	if zero != 0 {
		t.Fatalf("expected 0, got %d", zero)
	}
}
