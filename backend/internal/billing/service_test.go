package billing

import (
	"errors"
	"testing"

	"github.com/barbersloyalties/backend/internal/domain"
)

func TestCreateSubscriptionCheckoutRequiresTenantID(t *testing.T) {
	svc := NewService(nil, nil, nil, ServiceConfig{
		SubscriptionPriceID: "price_123",
		MobileSuccessURL:    "https://example.com/success",
		MobileCancelURL:     "https://example.com/cancel",
	})

	_, err := svc.CreateSubscriptionCheckout(t.Context(), CreateSubscriptionCheckoutInput{})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestCreateSubscriptionCheckoutRequiresPriceID(t *testing.T) {
	stripeProvider := NewStripeProvider("", "")
	svc := NewService(nil, nil, nil, ServiceConfig{
		SubscriptionPriceID: "",
		MobileSuccessURL:    "https://example.com/success",
		MobileCancelURL:     "https://example.com/cancel",
	})
	svc.billingProvider = stripeProvider

	_, err := svc.CreateSubscriptionCheckout(t.Context(), CreateSubscriptionCheckoutInput{TenantID: "tenant-1"})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestParseCheckoutSessionEventObjectAndExtractID(t *testing.T) {
	raw := []byte(`{"id":"cs_test","mode":"subscription","client_reference_id":"tenant-1","metadata":{"tenant_id":"tenant-1"},"customer":"cus_123","subscription":"sub_123"}`)
	obj, err := parseCheckoutSessionEventObject(raw)
	if err != nil {
		t.Fatalf("parse checkout session: %v", err)
	}
	if obj.ID != "cs_test" {
		t.Fatalf("expected cs_test, got %s", obj.ID)
	}
	if extractID(obj.Customer) != "cus_123" {
		t.Fatalf("expected customer id cus_123")
	}
	if extractID(obj.Subscription) != "sub_123" {
		t.Fatalf("expected subscription id sub_123")
	}
}

func TestNormalizeSubscriptionStatus(t *testing.T) {
	if got := normalizeSubscriptionStatus("active"); got != "active" {
		t.Fatalf("expected active, got %s", got)
	}
	if got := normalizeSubscriptionStatus(""); got != "inactive" {
		t.Fatalf("expected inactive fallback, got %s", got)
	}
}
