package payments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FakeStripeProvider struct {
	webhookSecret string
	baseURL       string
}

func NewFakeStripeProvider(baseURL, webhookSecret string) *FakeStripeProvider {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		trimmedBaseURL = "https://fake.stripe.local"
	}

	return &FakeStripeProvider{
		webhookSecret: strings.TrimSpace(webhookSecret),
		baseURL:       trimmedBaseURL,
	}
}

func (p *FakeStripeProvider) IsConfigured() bool {
	return p != nil
}

func (p *FakeStripeProvider) CreateCustomerCheckoutSession(_ context.Context, _ CustomerCheckoutSessionParams) (CustomerCheckoutSession, error) {
	sessionID := newMockID("cs_test")
	paymentIntentID := newMockID("pi")
	return CustomerCheckoutSession{
		ID:            sessionID,
		URL:           fmt.Sprintf("%s/checkout/%s", p.baseURL, sessionID),
		PaymentIntent: paymentIntentID,
	}, nil
}

func (p *FakeStripeProvider) RefundPayment(_ context.Context, _ PaymentRefundParams) (PaymentRefundResult, error) {
	return PaymentRefundResult{
		RefundID: newMockID("re"),
		Status:   "succeeded",
	}, nil
}

func (p *FakeStripeProvider) ParsePaymentWebhook(payload []byte) (PaymentWebhookEvent, error) {
	return parsePaymentWebhookPayload(payload)
}

func (p *FakeStripeProvider) VerifyPaymentWebhookSignature(_ []byte, signatureHeader string) error {
	header := strings.TrimSpace(signatureHeader)
	if p.webhookSecret == "" {
		if header != "" && header != fakeWebhookSignatureValue {
			return fmt.Errorf("invalid fake signature")
		}
		return nil
	}
	if header == "" {
		return fmt.Errorf("missing fake signature")
	}
	if header != p.webhookSecret && header != fakeWebhookSignatureValue {
		return fmt.Errorf("invalid fake signature")
	}
	return nil
}

func newMockID(prefix string) string {
	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(token) > 12 {
		token = token[:12]
	}
	return fmt.Sprintf("%s_mock_%s", strings.TrimSpace(prefix), token)
}

func newMockEventID() string {
	return fmt.Sprintf("evt_mock_%d_%s", time.Now().UTC().Unix(), strings.ReplaceAll(uuid.NewString(), "-", "")[:8])
}
