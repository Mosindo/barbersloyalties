package payments

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	providerStripe            = "stripe"
	fakeWebhookSignatureValue = "fake-signature"
)

type CustomerCheckoutSessionParams struct {
	AmountCents int64
	Currency    string
	SuccessURL  string
	CancelURL   string
	TenantID    string
	CustomerID  string
	Description string
}

type CustomerCheckoutSession struct {
	ID            string
	URL           string
	PaymentIntent string
}

type PaymentRefundParams struct {
	PaymentIntentID string
	AmountCents     int64
	Currency        string
	Reason          string
}

type PaymentRefundResult struct {
	RefundID string
	Status   string
}

type PaymentWebhookEvent struct {
	ID   string
	Type string
	Data json.RawMessage
}

type paymentSessionEventObject struct {
	ID            string            `json:"id"`
	PaymentStatus string            `json:"payment_status"`
	Status        string            `json:"status"`
	AmountTotal   int64             `json:"amount_total"`
	Currency      string            `json:"currency"`
	PaymentIntent string            `json:"payment_intent"`
	Metadata      map[string]string `json:"metadata"`
}

type paymentRefundEventObject struct {
	ID             string            `json:"id"`
	Amount         int64             `json:"amount"`
	AmountRefunded int64             `json:"amount_refunded"`
	Currency       string            `json:"currency"`
	PaymentIntent  string            `json:"payment_intent"`
	Metadata       map[string]string `json:"metadata"`
}

type PaymentsProvider interface {
	IsConfigured() bool
	CreateCustomerCheckoutSession(ctx context.Context, input CustomerCheckoutSessionParams) (CustomerCheckoutSession, error)
	RefundPayment(ctx context.Context, input PaymentRefundParams) (PaymentRefundResult, error)
	ParsePaymentWebhook(payload []byte) (PaymentWebhookEvent, error)
	VerifyPaymentWebhookSignature(payload []byte, signatureHeader string) error
}

func parsePaymentWebhookPayload(payload []byte) (PaymentWebhookEvent, error) {
	var raw struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return PaymentWebhookEvent{}, fmt.Errorf("parse webhook payload: %w", err)
	}
	if raw.ID == "" || raw.Type == "" {
		return PaymentWebhookEvent{}, fmt.Errorf("missing webhook id or type")
	}

	return PaymentWebhookEvent{
		ID:   raw.ID,
		Type: raw.Type,
		Data: raw.Data.Object,
	}, nil
}

func parsePaymentSessionObject(raw json.RawMessage) (paymentSessionEventObject, error) {
	var out paymentSessionEventObject
	if err := json.Unmarshal(raw, &out); err != nil {
		return paymentSessionEventObject{}, fmt.Errorf("parse payment session object: %w", err)
	}
	if out.ID == "" {
		return paymentSessionEventObject{}, fmt.Errorf("missing checkout session id")
	}
	if out.Metadata == nil {
		out.Metadata = map[string]string{}
	}
	return out, nil
}

func parsePaymentRefundObject(raw json.RawMessage) (paymentRefundEventObject, error) {
	var out paymentRefundEventObject
	if err := json.Unmarshal(raw, &out); err != nil {
		return paymentRefundEventObject{}, fmt.Errorf("parse payment refund object: %w", err)
	}
	if out.ID == "" {
		return paymentRefundEventObject{}, fmt.Errorf("missing refund id")
	}
	if out.Amount <= 0 && out.AmountRefunded > 0 {
		out.Amount = out.AmountRefunded
	}
	if out.Metadata == nil {
		out.Metadata = map[string]string{}
	}
	return out, nil
}
