package payments

import (
	"context"
	"testing"
)

func TestFakeStripeProviderCreatesMockCheckoutIDs(t *testing.T) {
	provider := NewFakeStripeProvider("https://fake.stripe.local", "")

	session, err := provider.CreateCustomerCheckoutSession(context.Background(), CustomerCheckoutSessionParams{
		AmountCents: 2500,
		Currency:    "EUR",
		SuccessURL:  "app://success",
		CancelURL:   "app://cancel",
		TenantID:    "tenant-1",
		CustomerID:  "customer-1",
		Description: "Barber visit",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(session.ID) == 0 || session.ID[:13] != "cs_test_mock_" {
		t.Fatalf("expected mock checkout session id, got %s", session.ID)
	}
	if len(session.PaymentIntent) == 0 || session.PaymentIntent[:8] != "pi_mock_" {
		t.Fatalf("expected mock payment intent id, got %s", session.PaymentIntent)
	}
}

func TestFakeStripeProviderRefundPayment(t *testing.T) {
	provider := NewFakeStripeProvider("https://fake.stripe.local", "")

	refund, err := provider.RefundPayment(context.Background(), PaymentRefundParams{
		PaymentIntentID: "pi_mock_123",
		AmountCents:     2500,
		Currency:        "EUR",
		Reason:          "requested_by_customer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refund.RefundID) == 0 || refund.RefundID[:8] != "re_mock_" {
		t.Fatalf("expected mock refund id, got %s", refund.RefundID)
	}
	if refund.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %s", refund.Status)
	}
}
