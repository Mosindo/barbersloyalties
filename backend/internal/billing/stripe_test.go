package billing

import (
	"context"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v84/webhook"
)

func TestStripeProviderIsConfigured(t *testing.T) {
	provider := NewStripeProvider("sk_test_123", "whsec_123")
	if !provider.IsConfigured() {
		t.Fatal("expected provider to be configured")
	}

	provider = NewStripeProvider("", "whsec_123")
	if provider.IsConfigured() {
		t.Fatal("expected provider without secret key to be unconfigured")
	}
}

func TestVerifyBillingWebhookSignatureRequiresSecret(t *testing.T) {
	provider := NewStripeProvider("sk_test_123", "")
	err := provider.VerifyBillingWebhookSignature([]byte(`{}`), "t=1,v1=abc")
	if err == nil {
		t.Fatal("expected webhook secret validation error")
	}
}

func TestVerifyAndParseBillingWebhookValid(t *testing.T) {
	secret := "whsec_test_billing"
	payload := []byte(`{"id":"evt_1","object":"event","api_version":"2026-02-25.clover","type":"invoice.paid","data":{"object":{"id":"in_1","object":"invoice"}}}`)

	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    secret,
		Timestamp: time.Now().UTC(),
		Scheme:    "v1",
	})

	provider := NewStripeProvider("sk_test_123", secret)
	if err := provider.VerifyBillingWebhookSignature(payload, signed.Header); err != nil {
		t.Fatalf("expected valid billing webhook signature, got %v", err)
	}

	event, err := provider.ParseBillingWebhook(payload)
	if err != nil {
		t.Fatalf("expected valid billing webhook payload, got %v", err)
	}
	if event.ID != "evt_1" {
		t.Fatalf("expected evt_1, got %s", event.ID)
	}
	if event.Type != "invoice.paid" {
		t.Fatalf("expected invoice.paid, got %s", event.Type)
	}
}

func TestFakeStripeProviderCreatesMockCheckoutIDs(t *testing.T) {
	provider := NewFakeStripeProvider("https://fake.stripe.local", "")

	session, err := provider.CreateSubscriptionCheckoutSession(context.Background(), CreateCheckoutSessionParams{
		TenantID:   "tenant-1",
		PriceID:    "price_123",
		SuccessURL: "app://success",
		CancelURL:  "app://cancel",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(session.ID) == 0 || session.ID[:13] != "cs_test_mock_" {
		t.Fatalf("expected mock checkout session id, got %s", session.ID)
	}
	if len(session.CustomerID) == 0 || session.CustomerID[:9] != "cus_mock_" {
		t.Fatalf("expected mock customer id, got %s", session.CustomerID)
	}
	if len(session.SubscriptionID) == 0 || session.SubscriptionID[:9] != "sub_mock_" {
		t.Fatalf("expected mock subscription id, got %s", session.SubscriptionID)
	}
}
