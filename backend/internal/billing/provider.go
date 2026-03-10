package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
	billingportalsession "github.com/stripe/stripe-go/v84/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v84/checkout/session"
	"github.com/stripe/stripe-go/v84/webhook"
)

const fakeWebhookSignatureValue = "fake-signature"

type CreateCheckoutSessionParams struct {
	TenantID         string
	PriceID          string
	SuccessURL       string
	CancelURL        string
	StripeCustomerID string
}

type CreatedCheckoutSession struct {
	ID             string
	URL            string
	CustomerID     string
	SubscriptionID string
}

type CreatePortalSessionParams struct {
	StripeCustomerID string
	ReturnURL        string
}

type CreatedPortalSession struct {
	ID  string
	URL string
}

type BillingWebhookEvent struct {
	ID   string
	Type string
	Data json.RawMessage
}

type BillingProvider interface {
	IsConfigured() bool
	CreateSubscriptionCheckoutSession(ctx context.Context, input CreateCheckoutSessionParams) (CreatedCheckoutSession, error)
	CreateCustomerPortalSession(ctx context.Context, input CreatePortalSessionParams) (CreatedPortalSession, error)
	ParseBillingWebhook(payload []byte) (BillingWebhookEvent, error)
	VerifyBillingWebhookSignature(payload []byte, signatureHeader string) error
}

type StripeProvider struct {
	secretKey     string
	webhookSecret string
}

func NewStripeProvider(secretKey, webhookSecret string) *StripeProvider {
	return &StripeProvider{
		secretKey:     strings.TrimSpace(secretKey),
		webhookSecret: strings.TrimSpace(webhookSecret),
	}
}

func (p *StripeProvider) IsConfigured() bool {
	return p != nil && p.secretKey != ""
}

func (p *StripeProvider) CreateSubscriptionCheckoutSession(ctx context.Context, input CreateCheckoutSessionParams) (CreatedCheckoutSession, error) {
	if !p.IsConfigured() {
		return CreatedCheckoutSession{}, fmt.Errorf("stripe secret key is not configured")
	}
	if strings.TrimSpace(input.PriceID) == "" {
		return CreatedCheckoutSession{}, fmt.Errorf("stripe subscription price id is required")
	}

	stripe.Key = p.secretKey

	tenantID := strings.TrimSpace(input.TenantID)
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(strings.TrimSpace(input.SuccessURL)),
		CancelURL:         stripe.String(strings.TrimSpace(input.CancelURL)),
		ClientReferenceID: stripe.String(tenantID),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(strings.TrimSpace(input.PriceID)),
				Quantity: stripe.Int64(1),
			},
		},
	}
	if tenantID != "" {
		params.AddMetadata("tenant_id", tenantID)
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{}
		params.SubscriptionData.AddMetadata("tenant_id", tenantID)
	}
	if strings.TrimSpace(input.StripeCustomerID) != "" {
		params.Customer = stripe.String(strings.TrimSpace(input.StripeCustomerID))
	}

	session, err := checkoutsession.New(params)
	if err != nil {
		return CreatedCheckoutSession{}, fmt.Errorf("create stripe checkout session: %w", err)
	}
	if session == nil || session.ID == "" || session.URL == "" {
		return CreatedCheckoutSession{}, fmt.Errorf("invalid stripe checkout session response")
	}

	customerID := ""
	if session.Customer != nil {
		customerID = session.Customer.ID
	}
	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	return CreatedCheckoutSession{
		ID:             session.ID,
		URL:            session.URL,
		CustomerID:     strings.TrimSpace(customerID),
		SubscriptionID: strings.TrimSpace(subscriptionID),
	}, nil
}

func (p *StripeProvider) CreateCustomerPortalSession(ctx context.Context, input CreatePortalSessionParams) (CreatedPortalSession, error) {
	if !p.IsConfigured() {
		return CreatedPortalSession{}, fmt.Errorf("stripe secret key is not configured")
	}
	if strings.TrimSpace(input.StripeCustomerID) == "" {
		return CreatedPortalSession{}, fmt.Errorf("stripe customer id is required")
	}

	stripe.Key = p.secretKey

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(strings.TrimSpace(input.StripeCustomerID)),
		ReturnURL: stripe.String(strings.TrimSpace(input.ReturnURL)),
	}

	session, err := billingportalsession.New(params)
	if err != nil {
		return CreatedPortalSession{}, fmt.Errorf("create stripe billing portal session: %w", err)
	}
	if session == nil || session.ID == "" || session.URL == "" {
		return CreatedPortalSession{}, fmt.Errorf("invalid stripe billing portal response")
	}

	return CreatedPortalSession{
		ID:  session.ID,
		URL: session.URL,
	}, nil
}

func (p *StripeProvider) ParseBillingWebhook(payload []byte) (BillingWebhookEvent, error) {
	return parseBillingWebhookPayload(payload)
}

func (p *StripeProvider) VerifyBillingWebhookSignature(payload []byte, signatureHeader string) error {
	if strings.TrimSpace(p.webhookSecret) == "" {
		return fmt.Errorf("stripe webhook secret is not configured")
	}

	_, err := webhook.ConstructEventWithOptions(payload, signatureHeader, p.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return fmt.Errorf("verify stripe billing webhook signature: %w", err)
	}
	return nil
}

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

func (p *FakeStripeProvider) CreateSubscriptionCheckoutSession(_ context.Context, input CreateCheckoutSessionParams) (CreatedCheckoutSession, error) {
	customerID := strings.TrimSpace(input.StripeCustomerID)
	if customerID == "" {
		customerID = newMockID("cus")
	}
	sessionID := newMockID("cs_test")

	return CreatedCheckoutSession{
		ID:             sessionID,
		URL:            fmt.Sprintf("%s/checkout/%s", p.baseURL, sessionID),
		CustomerID:     customerID,
		SubscriptionID: newMockID("sub"),
	}, nil
}

func (p *FakeStripeProvider) CreateCustomerPortalSession(_ context.Context, input CreatePortalSessionParams) (CreatedPortalSession, error) {
	if strings.TrimSpace(input.StripeCustomerID) == "" {
		return CreatedPortalSession{}, fmt.Errorf("stripe customer id is required")
	}
	portalID := newMockID("bps")
	return CreatedPortalSession{
		ID:  portalID,
		URL: fmt.Sprintf("%s/portal/%s?customer=%s", p.baseURL, portalID, strings.TrimSpace(input.StripeCustomerID)),
	}, nil
}

func (p *FakeStripeProvider) ParseBillingWebhook(payload []byte) (BillingWebhookEvent, error) {
	return parseBillingWebhookPayload(payload)
}

func (p *FakeStripeProvider) VerifyBillingWebhookSignature(_ []byte, signatureHeader string) error {
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

func parseBillingWebhookPayload(payload []byte) (BillingWebhookEvent, error) {
	var raw struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return BillingWebhookEvent{}, fmt.Errorf("parse billing webhook payload: %w", err)
	}
	if raw.ID == "" || raw.Type == "" {
		return BillingWebhookEvent{}, fmt.Errorf("missing webhook id or type")
	}

	return BillingWebhookEvent{
		ID:   raw.ID,
		Type: raw.Type,
		Data: raw.Data.Object,
	}, nil
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
