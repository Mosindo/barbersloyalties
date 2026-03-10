package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/barbersloyalties/backend/internal/subscriptions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const stripeBillingProvider = "stripe_billing"
const fakeSubscriptionPriceFallback = "price_mock_9eur_monthly"

type ServiceConfig struct {
	SubscriptionPriceID string
	MobileSuccessURL    string
	MobileCancelURL     string
	AppBaseURL          string
	PublishableKey      string
}

type CreateSubscriptionCheckoutInput struct {
	TenantID   string
	SuccessURL string
	CancelURL  string
}

type CreateSubscriptionCheckoutResult struct {
	CheckoutSessionID string `json:"checkout_session_id"`
	CheckoutURL       string `json:"checkout_url"`
	PublishableKey    string `json:"publishable_key"`
}

type CreatePortalSessionInput struct {
	TenantID  string
	ReturnURL string
}

type CreatePortalSessionResult struct {
	PortalSessionID string `json:"portal_session_id"`
	PortalURL       string `json:"portal_url"`
}

type SubscriptionStatusResult struct {
	Subscription        subscriptions.Subscription `json:"subscription"`
	HasActiveAccess     bool                       `json:"has_active_access"`
	PublishableKey      string                     `json:"publishable_key"`
	SubscriptionPriceID string                     `json:"subscription_price_id"`
}

type WebhookResult struct {
	EventID      string                      `json:"event_id"`
	EventType    string                      `json:"event_type"`
	Deduplicated bool                        `json:"deduplicated"`
	Ignored      bool                        `json:"ignored"`
	Subscription *subscriptions.Subscription `json:"subscription,omitempty"`
}

type checkoutSessionEventObject struct {
	ID                string            `json:"id"`
	Mode              string            `json:"mode"`
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
	Customer          json.RawMessage   `json:"customer"`
	Subscription      json.RawMessage   `json:"subscription"`
}

type subscriptionEventObject struct {
	ID               string            `json:"id"`
	Status           string            `json:"status"`
	CurrentPeriodEnd int64             `json:"current_period_end"`
	Metadata         map[string]string `json:"metadata"`
	Customer         json.RawMessage   `json:"customer"`
}

type invoiceEventObject struct {
	ID           string          `json:"id"`
	Customer     json.RawMessage `json:"customer"`
	Subscription json.RawMessage `json:"subscription"`
}

type Service struct {
	pool                *pgxpool.Pool
	subscriptions       *subscriptions.Service
	billingProvider     BillingProvider
	subscriptionPriceID string
	mobileSuccessURL    string
	mobileCancelURL     string
	appBaseURL          string
	publishableKey      string
}

func NewService(
	pool *pgxpool.Pool,
	subscriptionService *subscriptions.Service,
	billingProvider BillingProvider,
	cfg ServiceConfig,
) *Service {
	if billingProvider == nil {
		billingProvider = NewFakeStripeProvider("", "")
	}

	return &Service{
		pool:                pool,
		subscriptions:       subscriptionService,
		billingProvider:     billingProvider,
		subscriptionPriceID: strings.TrimSpace(cfg.SubscriptionPriceID),
		mobileSuccessURL:    strings.TrimSpace(cfg.MobileSuccessURL),
		mobileCancelURL:     strings.TrimSpace(cfg.MobileCancelURL),
		appBaseURL:          strings.TrimSpace(cfg.AppBaseURL),
		publishableKey:      strings.TrimSpace(cfg.PublishableKey),
	}
}

func (s *Service) IsFakeProvider() bool {
	_, ok := s.billingProvider.(*FakeStripeProvider)
	return ok
}

func (s *Service) GetSubscription(ctx context.Context, tenantID string) (SubscriptionStatusResult, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return SubscriptionStatusResult{}, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}

	hasAccess, sub, err := s.subscriptions.HasActiveAccess(ctx, tenantID)
	if err != nil {
		return SubscriptionStatusResult{}, err
	}

	return SubscriptionStatusResult{
		Subscription:        sub,
		HasActiveAccess:     hasAccess,
		PublishableKey:      s.publishableKey,
		SubscriptionPriceID: s.subscriptionPriceID,
	}, nil
}

func (s *Service) CreateSubscriptionCheckout(ctx context.Context, input CreateSubscriptionCheckoutInput) (CreateSubscriptionCheckoutResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return CreateSubscriptionCheckoutResult{}, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}
	priceID := strings.TrimSpace(s.subscriptionPriceID)
	if priceID == "" && s.IsFakeProvider() {
		priceID = fakeSubscriptionPriceFallback
	}
	if priceID == "" {
		return CreateSubscriptionCheckoutResult{}, fmt.Errorf("%w: STRIPE_SUBSCRIPTION_PRICE_ID is required", domain.ErrInvalidRequest)
	}
	if !s.billingProvider.IsConfigured() {
		return CreateSubscriptionCheckoutResult{}, fmt.Errorf("stripe is not configured")
	}

	successURL := coalesceNonEmpty(input.SuccessURL, s.mobileSuccessURL)
	cancelURL := coalesceNonEmpty(input.CancelURL, s.mobileCancelURL)
	if successURL == "" || cancelURL == "" {
		return CreateSubscriptionCheckoutResult{}, fmt.Errorf("%w: MOBILE_SUCCESS_URL and MOBILE_CANCEL_URL must be configured", domain.ErrInvalidRequest)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateSubscriptionCheckoutResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	sub, err := s.subscriptions.GetOrCreateByTenant(ctx, tx, tenantID)
	if err != nil {
		return CreateSubscriptionCheckoutResult{}, err
	}

	session, err := s.billingProvider.CreateSubscriptionCheckoutSession(ctx, CreateCheckoutSessionParams{
		TenantID:         tenantID,
		PriceID:          priceID,
		SuccessURL:       successURL,
		CancelURL:        cancelURL,
		StripeCustomerID: sub.StripeCustomerID,
	})
	if err != nil {
		return CreateSubscriptionCheckoutResult{}, err
	}

	if session.CustomerID != "" || session.SubscriptionID != "" {
		_, err = s.subscriptions.UpsertByTenant(ctx, tx, subscriptions.UpsertByTenantInput{
			TenantID:             tenantID,
			StripeCustomerID:     coalesceNonEmpty(session.CustomerID, sub.StripeCustomerID),
			StripeSubscriptionID: coalesceNonEmpty(session.SubscriptionID, sub.StripeSubscriptionID),
			Status:               sub.Status,
			MonthlyPriceCents:    sub.MonthlyPriceCents,
			CurrentPeriodEnd:     sub.CurrentPeriodEnd,
		})
		if err != nil {
			return CreateSubscriptionCheckoutResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateSubscriptionCheckoutResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return CreateSubscriptionCheckoutResult{
		CheckoutSessionID: session.ID,
		CheckoutURL:       session.URL,
		PublishableKey:    s.publishableKey,
	}, nil
}

func (s *Service) CreatePortalSession(ctx context.Context, input CreatePortalSessionInput) (CreatePortalSessionResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return CreatePortalSessionResult{}, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}
	if !s.billingProvider.IsConfigured() {
		return CreatePortalSessionResult{}, fmt.Errorf("stripe is not configured")
	}

	returnURL := coalesceNonEmpty(input.ReturnURL, s.appBaseURL)
	if returnURL == "" {
		return CreatePortalSessionResult{}, fmt.Errorf("%w: APP_BASE_URL or return_url is required", domain.ErrInvalidRequest)
	}

	sub, err := s.subscriptions.GetOrCreateByTenant(ctx, nil, tenantID)
	if err != nil {
		return CreatePortalSessionResult{}, err
	}
	if strings.TrimSpace(sub.StripeCustomerID) == "" {
		return CreatePortalSessionResult{}, fmt.Errorf("%w: stripe customer is not available yet", domain.ErrInvalidRequest)
	}

	portalSession, err := s.billingProvider.CreateCustomerPortalSession(ctx, CreatePortalSessionParams{
		StripeCustomerID: sub.StripeCustomerID,
		ReturnURL:        returnURL,
	})
	if err != nil {
		return CreatePortalSessionResult{}, err
	}

	return CreatePortalSessionResult{
		PortalSessionID: portalSession.ID,
		PortalURL:       portalSession.URL,
	}, nil
}

func (s *Service) HandleStripeBillingWebhook(ctx context.Context, payload []byte, signatureHeader string) (WebhookResult, error) {
	if err := s.billingProvider.VerifyBillingWebhookSignature(payload, signatureHeader); err != nil {
		return WebhookResult{}, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}

	event, err := s.billingProvider.ParseBillingWebhook(payload)
	if err != nil {
		return WebhookResult{}, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return WebhookResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	inserted, err := s.insertProcessedWebhook(ctx, tx, stripeBillingProvider, event.ID)
	if err != nil {
		return WebhookResult{}, err
	}
	if !inserted {
		if err := tx.Commit(ctx); err != nil {
			return WebhookResult{}, fmt.Errorf("commit duplicate webhook transaction: %w", err)
		}
		return WebhookResult{EventID: event.ID, EventType: event.Type, Deduplicated: true}, nil
	}

	result, err := s.processWebhookEvent(ctx, tx, event)
	if err != nil {
		return WebhookResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return WebhookResult{}, fmt.Errorf("commit webhook transaction: %w", err)
	}

	return result, nil
}

func (s *Service) processWebhookEvent(ctx context.Context, tx pgx.Tx, event BillingWebhookEvent) (WebhookResult, error) {
	result := WebhookResult{EventID: event.ID, EventType: event.Type}

	switch event.Type {
	case "checkout.session.completed":
		obj, err := parseCheckoutSessionEventObject(event.Data)
		if err != nil {
			return WebhookResult{}, err
		}
		if strings.TrimSpace(obj.Mode) != "subscription" {
			result.Ignored = true
			return result, nil
		}

		tenantID := strings.TrimSpace(obj.Metadata["tenant_id"])
		if tenantID == "" {
			tenantID = strings.TrimSpace(obj.ClientReferenceID)
		}
		if tenantID == "" {
			result.Ignored = true
			return result, nil
		}

		sub, err := s.subscriptions.GetOrCreateByTenant(ctx, tx, tenantID)
		if err != nil {
			return WebhookResult{}, err
		}

		updated, err := s.subscriptions.UpsertByTenant(ctx, tx, subscriptions.UpsertByTenantInput{
			TenantID:             tenantID,
			StripeCustomerID:     coalesceNonEmpty(extractID(obj.Customer), sub.StripeCustomerID),
			StripeSubscriptionID: coalesceNonEmpty(extractID(obj.Subscription), sub.StripeSubscriptionID),
			Status:               sub.Status,
			MonthlyPriceCents:    sub.MonthlyPriceCents,
			CurrentPeriodEnd:     sub.CurrentPeriodEnd,
		})
		if err != nil {
			return WebhookResult{}, err
		}
		result.Subscription = &updated
		return result, nil

	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		obj, err := parseSubscriptionEventObject(event.Data)
		if err != nil {
			return WebhookResult{}, err
		}

		status := normalizeSubscriptionStatus(obj.Status)
		if event.Type == "customer.subscription.deleted" {
			status = subscriptions.StatusCanceled
		}
		periodEnd := unixToPointer(obj.CurrentPeriodEnd)

		tenantID := strings.TrimSpace(obj.Metadata["tenant_id"])
		if tenantID != "" {
			sub, err := s.subscriptions.GetOrCreateByTenant(ctx, tx, tenantID)
			if err != nil {
				return WebhookResult{}, err
			}
			updated, err := s.subscriptions.UpsertByTenant(ctx, tx, subscriptions.UpsertByTenantInput{
				TenantID:             tenantID,
				StripeCustomerID:     coalesceNonEmpty(extractID(obj.Customer), sub.StripeCustomerID),
				StripeSubscriptionID: coalesceNonEmpty(obj.ID, sub.StripeSubscriptionID),
				Status:               status,
				MonthlyPriceCents:    sub.MonthlyPriceCents,
				CurrentPeriodEnd:     periodEnd,
			})
			if err != nil {
				return WebhookResult{}, err
			}
			result.Subscription = &updated
			return result, nil
		}

		updated, err := s.subscriptions.UpdateByStripeRefs(ctx, tx, subscriptions.UpdateByStripeRefsInput{
			StripeSubscriptionID: obj.ID,
			StripeCustomerID:     extractID(obj.Customer),
			Status:               status,
			CurrentPeriodEnd:     periodEnd,
		})
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				result.Ignored = true
				return result, nil
			}
			return WebhookResult{}, err
		}
		result.Subscription = &updated
		return result, nil

	case "invoice.paid", "invoice.payment_succeeded", "invoice.payment_failed":
		obj, err := parseInvoiceEventObject(event.Data)
		if err != nil {
			return WebhookResult{}, err
		}

		status := subscriptions.StatusActive
		if event.Type == "invoice.payment_failed" {
			status = subscriptions.StatusPastDue
		}

		updated, err := s.subscriptions.UpdateByStripeRefs(ctx, tx, subscriptions.UpdateByStripeRefsInput{
			StripeSubscriptionID: extractID(obj.Subscription),
			StripeCustomerID:     extractID(obj.Customer),
			Status:               status,
			CurrentPeriodEnd:     nil,
		})
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				result.Ignored = true
				return result, nil
			}
			return WebhookResult{}, err
		}
		result.Subscription = &updated
		return result, nil

	default:
		result.Ignored = true
		return result, nil
	}
}

func (s *Service) insertProcessedWebhook(ctx context.Context, tx pgx.Tx, provider, eventID string) (bool, error) {
	query := `
		INSERT INTO processed_webhooks (id, provider, event_id, processed_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, event_id) DO NOTHING
		RETURNING id
	`

	var insertedID string
	err := tx.QueryRow(ctx, query, uuid.NewString(), provider, eventID, time.Now().UTC()).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("insert processed webhook: %w", err)
	}
	return insertedID != "", nil
}

func parseCheckoutSessionEventObject(raw json.RawMessage) (checkoutSessionEventObject, error) {
	var out checkoutSessionEventObject
	if err := json.Unmarshal(raw, &out); err != nil {
		return checkoutSessionEventObject{}, fmt.Errorf("parse checkout session object: %w", err)
	}
	if out.Metadata == nil {
		out.Metadata = map[string]string{}
	}
	return out, nil
}

func parseSubscriptionEventObject(raw json.RawMessage) (subscriptionEventObject, error) {
	var out subscriptionEventObject
	if err := json.Unmarshal(raw, &out); err != nil {
		return subscriptionEventObject{}, fmt.Errorf("parse subscription object: %w", err)
	}
	if out.Metadata == nil {
		out.Metadata = map[string]string{}
	}
	return out, nil
}

func parseInvoiceEventObject(raw json.RawMessage) (invoiceEventObject, error) {
	var out invoiceEventObject
	if err := json.Unmarshal(raw, &out); err != nil {
		return invoiceEventObject{}, fmt.Errorf("parse invoice object: %w", err)
	}
	return out, nil
}

func extractID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}

	var asObject struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &asObject); err == nil {
		return strings.TrimSpace(asObject.ID)
	}

	return ""
}

func normalizeSubscriptionStatus(raw string) string {
	switch strings.TrimSpace(raw) {
	case subscriptions.StatusActive,
		subscriptions.StatusTrialing,
		subscriptions.StatusPastDue,
		subscriptions.StatusCanceled,
		subscriptions.StatusIncomplete,
		subscriptions.StatusIncompleteExpire,
		subscriptions.StatusUnpaid:
		return strings.TrimSpace(raw)
	default:
		if strings.TrimSpace(raw) == "" {
			return subscriptions.StatusInactive
		}
		return strings.TrimSpace(raw)
	}
}

func unixToPointer(unix int64) *time.Time {
	if unix <= 0 {
		return nil
	}
	t := time.Unix(unix, 0).UTC()
	return &t
}

func coalesceNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return strings.TrimSpace(fallback)
}
