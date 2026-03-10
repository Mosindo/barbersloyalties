package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/barbersloyalties/backend/internal/ledger"
	"github.com/barbersloyalties/backend/internal/loyalty"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stripeEventCheckoutCompleted      = "checkout.session.completed"
	stripeEventCheckoutExpired        = "checkout.session.expired"
	stripeEventCheckoutAsyncSucceeded = "checkout.session.async_payment_succeeded"
	stripeEventCheckoutAsyncFailed    = "checkout.session.async_payment_failed"
	stripeEventChargeRefunded         = "charge.refunded"
)

type ServiceConfig struct {
	PlatformCommissionBPS int
}

type RecordCashVisitInput struct {
	TenantID    string
	CustomerID  string
	AmountCents int64
	Currency    string
	Notes       string
}

type RecordCashVisitResult struct {
	VisitTransaction        ledger.Transaction  `json:"visit_transaction"`
	RewardUnlockTransaction *ledger.Transaction `json:"reward_unlock_transaction,omitempty"`
	LoyaltyState            loyalty.State       `json:"loyalty_state"`
}

type RedeemRewardInput struct {
	TenantID   string
	CustomerID string
	Currency   string
	Notes      string
}

type RedeemRewardResult struct {
	RedemptionTransaction ledger.Transaction `json:"redemption_transaction"`
	LoyaltyState          loyalty.State      `json:"loyalty_state"`
}

type CreateStripeCheckoutInput struct {
	TenantID    string
	CustomerID  string
	AmountCents int64
	Currency    string
	SuccessURL  string
	CancelURL   string
	Description string
	Notes       string
}

type CreateStripeCheckoutResult struct {
	CheckoutSessionID  string             `json:"checkout_session_id"`
	CheckoutURL        string             `json:"checkout_url"`
	PendingTransaction ledger.Transaction `json:"pending_transaction"`
}

type StripeWebhookResult struct {
	EventID           string              `json:"event_id"`
	EventType         string              `json:"event_type"`
	Deduplicated      bool                `json:"deduplicated"`
	Ignored           bool                `json:"ignored"`
	FinalTransaction  *ledger.Transaction `json:"final_transaction,omitempty"`
	RewardUnlockTxnID string              `json:"reward_unlock_transaction_id,omitempty"`
}

type Service struct {
	pool                  *pgxpool.Pool
	ledgerService         *ledger.Service
	loyaltyService        *loyalty.Service
	paymentsProvider      PaymentsProvider
	platformCommissionBPS int
}

func NewService(
	pool *pgxpool.Pool,
	ledgerService *ledger.Service,
	loyaltyService *loyalty.Service,
	provider PaymentsProvider,
	cfg ServiceConfig,
) *Service {
	if provider == nil {
		provider = NewFakeStripeProvider("", "")
	}

	return &Service{
		pool:                  pool,
		ledgerService:         ledgerService,
		loyaltyService:        loyaltyService,
		paymentsProvider:      provider,
		platformCommissionBPS: cfg.PlatformCommissionBPS,
	}
}

func (s *Service) IsFakeProvider() bool {
	_, ok := s.paymentsProvider.(*FakeStripeProvider)
	return ok
}

func (s *Service) RecordCashVisit(ctx context.Context, input RecordCashVisitInput) (RecordCashVisitResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	customerID := strings.TrimSpace(input.CustomerID)
	if tenantID == "" || customerID == "" {
		return RecordCashVisitResult{}, fmt.Errorf("%w: tenant_id and customer_id are required", domain.ErrInvalidRequest)
	}
	if input.AmountCents <= 0 {
		return RecordCashVisitResult{}, fmt.Errorf("%w: amount_cents must be greater than zero", domain.ErrInvalidRequest)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RecordCashVisitResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := assertCustomerPayable(ctx, tx, tenantID, customerID); err != nil {
		return RecordCashVisitResult{}, err
	}

	now := time.Now().UTC()
	metadata, err := json.Marshal(map[string]any{
		"notes":  strings.TrimSpace(input.Notes),
		"source": "customers_pay_cash",
	})
	if err != nil {
		return RecordCashVisitResult{}, fmt.Errorf("marshal metadata: %w", err)
	}

	visitTxn, err := s.ledgerService.Append(ctx, tx, ledger.CreateInput{
		TenantID:         tenantID,
		CustomerID:       &customerID,
		Type:             ledger.TypeVisitPayment,
		PaymentMethod:    ledger.PaymentMethodCash,
		Status:           ledger.StatusSucceeded,
		AmountCents:      input.AmountCents,
		Currency:         input.Currency,
		PlatformFeeCents: 0,
		MetadataJSON:     metadata,
		CreatedAt:        now,
	})
	if err != nil {
		return RecordCashVisitResult{}, err
	}

	state, rewardUnlocked, err := s.loyaltyService.ApplyPaidVisit(ctx, tx, tenantID, customerID, now)
	if err != nil {
		return RecordCashVisitResult{}, err
	}

	rewardUnlockTxn, err := s.appendRewardUnlockTransaction(ctx, tx, visitTxn, rewardUnlocked, now)
	if err != nil {
		return RecordCashVisitResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RecordCashVisitResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return RecordCashVisitResult{
		VisitTransaction:        visitTxn,
		RewardUnlockTransaction: rewardUnlockTxn,
		LoyaltyState:            state,
	}, nil
}

func (s *Service) CreateStripeCheckout(ctx context.Context, input CreateStripeCheckoutInput) (CreateStripeCheckoutResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	customerID := strings.TrimSpace(input.CustomerID)
	successURL := strings.TrimSpace(input.SuccessURL)
	cancelURL := strings.TrimSpace(input.CancelURL)
	currency := normalizeCurrency(input.Currency)
	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = "Barber visit"
	}

	if tenantID == "" || customerID == "" {
		return CreateStripeCheckoutResult{}, fmt.Errorf("%w: tenant_id and customer_id are required", domain.ErrInvalidRequest)
	}
	if input.AmountCents <= 0 {
		return CreateStripeCheckoutResult{}, fmt.Errorf("%w: amount_cents must be greater than zero", domain.ErrInvalidRequest)
	}
	if successURL == "" || cancelURL == "" {
		return CreateStripeCheckoutResult{}, fmt.Errorf("%w: success_url and cancel_url are required", domain.ErrInvalidRequest)
	}
	if !s.paymentsProvider.IsConfigured() {
		return CreateStripeCheckoutResult{}, fmt.Errorf("stripe is not configured")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateStripeCheckoutResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := assertCustomerPayable(ctx, tx, tenantID, customerID); err != nil {
		return CreateStripeCheckoutResult{}, err
	}

	session, err := s.paymentsProvider.CreateCustomerCheckoutSession(ctx, CustomerCheckoutSessionParams{
		AmountCents: input.AmountCents,
		Currency:    currency,
		SuccessURL:  successURL,
		CancelURL:   cancelURL,
		TenantID:    tenantID,
		CustomerID:  customerID,
		Description: description,
	})
	if err != nil {
		return CreateStripeCheckoutResult{}, err
	}

	now := time.Now().UTC()
	metadata, err := json.Marshal(map[string]any{
		"notes":               strings.TrimSpace(input.Notes),
		"source":              "customers_create_stripe_checkout",
		"stripe_session_id":   session.ID,
		"stripe_checkout_url": session.URL,
	})
	if err != nil {
		return CreateStripeCheckoutResult{}, fmt.Errorf("marshal metadata: %w", err)
	}

	pendingTxn, err := s.ledgerService.Append(ctx, tx, ledger.CreateInput{
		TenantID:          tenantID,
		CustomerID:        &customerID,
		Type:              ledger.TypeVisitPayment,
		PaymentMethod:     ledger.PaymentMethodStripe,
		Status:            ledger.StatusPending,
		AmountCents:       input.AmountCents,
		Currency:          currency,
		PlatformFeeCents:  0,
		ExternalProvider:  providerStripe,
		ExternalReference: session.ID,
		MetadataJSON:      metadata,
		CreatedAt:         now,
	})
	if err != nil {
		return CreateStripeCheckoutResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateStripeCheckoutResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return CreateStripeCheckoutResult{
		CheckoutSessionID:  session.ID,
		CheckoutURL:        session.URL,
		PendingTransaction: pendingTxn,
	}, nil
}

func (s *Service) RedeemReward(ctx context.Context, input RedeemRewardInput) (RedeemRewardResult, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	customerID := strings.TrimSpace(input.CustomerID)
	if tenantID == "" || customerID == "" {
		return RedeemRewardResult{}, fmt.Errorf("%w: tenant_id and customer_id are required", domain.ErrInvalidRequest)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RedeemRewardResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := assertCustomerPayable(ctx, tx, tenantID, customerID); err != nil {
		return RedeemRewardResult{}, err
	}

	now := time.Now().UTC()
	state, err := s.loyaltyService.RedeemReward(ctx, tx, tenantID, customerID, now)
	if err != nil {
		return RedeemRewardResult{}, err
	}

	metadata, err := json.Marshal(map[string]any{
		"notes":  strings.TrimSpace(input.Notes),
		"source": "customers_redeem_reward",
	})
	if err != nil {
		return RedeemRewardResult{}, fmt.Errorf("marshal metadata: %w", err)
	}

	redemptionTxn, err := s.ledgerService.Append(ctx, tx, ledger.CreateInput{
		TenantID:         tenantID,
		CustomerID:       &customerID,
		Type:             ledger.TypeRewardRedemption,
		PaymentMethod:    ledger.PaymentMethodManual,
		Status:           ledger.StatusSucceeded,
		AmountCents:      0,
		Currency:         normalizeCurrency(input.Currency),
		PlatformFeeCents: 0,
		MetadataJSON:     metadata,
		CreatedAt:        now,
	})
	if err != nil {
		return RedeemRewardResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RedeemRewardResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return RedeemRewardResult{
		RedemptionTransaction: redemptionTxn,
		LoyaltyState:          state,
	}, nil
}

func (s *Service) HandleStripePaymentsWebhook(ctx context.Context, payload []byte, signatureHeader string) (StripeWebhookResult, error) {
	if err := s.paymentsProvider.VerifyPaymentWebhookSignature(payload, signatureHeader); err != nil {
		return StripeWebhookResult{}, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}

	event, err := s.paymentsProvider.ParsePaymentWebhook(payload)
	if err != nil {
		return StripeWebhookResult{}, fmt.Errorf("%w: %v", domain.ErrInvalidRequest, err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return StripeWebhookResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	inserted, err := s.insertProcessedWebhook(ctx, tx, providerStripe, event.ID)
	if err != nil {
		return StripeWebhookResult{}, err
	}
	if !inserted {
		if err := tx.Commit(ctx); err != nil {
			return StripeWebhookResult{}, fmt.Errorf("commit duplicate webhook transaction: %w", err)
		}
		return StripeWebhookResult{
			EventID:      event.ID,
			EventType:    event.Type,
			Deduplicated: true,
		}, nil
	}

	result, err := s.processStripeEvent(ctx, tx, event)
	if err != nil {
		return StripeWebhookResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return StripeWebhookResult{}, fmt.Errorf("commit webhook transaction: %w", err)
	}

	return result, nil
}

func (s *Service) processStripeEvent(ctx context.Context, tx pgx.Tx, event PaymentWebhookEvent) (StripeWebhookResult, error) {
	result := StripeWebhookResult{
		EventID:   event.ID,
		EventType: event.Type,
	}

	if strings.TrimSpace(event.Type) == stripeEventChargeRefunded {
		return s.processStripeRefundEvent(ctx, tx, event, result)
	}

	targetStatus, shouldHandle := targetStatusForStripeEvent(event.Type)
	if !shouldHandle {
		result.Ignored = true
		return result, nil
	}

	session, err := parsePaymentSessionObject(event.Data)
	if err != nil {
		return StripeWebhookResult{}, err
	}

	if event.Type == stripeEventCheckoutCompleted && strings.ToLower(strings.TrimSpace(session.PaymentStatus)) != "paid" {
		result.Ignored = true
		return result, nil
	}

	metadataTenantID := strings.TrimSpace(session.Metadata["tenant_id"])
	pendingTxn, err := s.findPendingStripeVisitTransaction(ctx, tx, session.ID, metadataTenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// If we cannot find a pending transaction, ignore safely to avoid corruption.
			result.Ignored = true
			return result, nil
		}
		return StripeWebhookResult{}, err
	}

	if pendingTxn.CustomerID == nil || strings.TrimSpace(*pendingTxn.CustomerID) == "" {
		return StripeWebhookResult{}, fmt.Errorf("%w: pending stripe visit transaction has no customer_id", domain.ErrInvalidRequest)
	}
	customerID := strings.TrimSpace(*pendingTxn.CustomerID)

	existingFinal, exists, err := s.findFinalizedStripeTransactionForPending(ctx, tx, pendingTxn.ID)
	if err != nil {
		return StripeWebhookResult{}, err
	}
	if exists {
		result.Deduplicated = true
		result.FinalTransaction = &existingFinal
		return result, nil
	}

	amount := pendingTxn.AmountCents
	if session.AmountTotal > 0 {
		amount = session.AmountTotal
	}
	currency := pendingTxn.Currency
	if c := normalizeCurrency(session.Currency); c != "" {
		currency = c
	}

	finalMetadata, err := json.Marshal(map[string]any{
		"stripe_event_id":       event.ID,
		"stripe_session_id":     session.ID,
		"stripe_payment_intent": strings.TrimSpace(session.PaymentIntent),
		"source":                "stripe_webhook_payments",
	})
	if err != nil {
		return StripeWebhookResult{}, fmt.Errorf("marshal final webhook metadata: %w", err)
	}

	relatedID := pendingTxn.ID
	platformFee := int64(0)
	if targetStatus == ledger.StatusSucceeded {
		platformFee = calculatePlatformFeeCents(amount, s.platformCommissionBPS)
	}

	finalTxn, err := s.ledgerService.Append(ctx, tx, ledger.CreateInput{
		TenantID:             pendingTxn.TenantID,
		CustomerID:           pendingTxn.CustomerID,
		Type:                 ledger.TypeVisitPayment,
		PaymentMethod:        ledger.PaymentMethodStripe,
		Status:               targetStatus,
		AmountCents:          amount,
		Currency:             currency,
		PlatformFeeCents:     platformFee,
		ExternalProvider:     providerStripe,
		ExternalReference:    session.ID,
		RelatedTransactionID: &relatedID,
		MetadataJSON:         finalMetadata,
		CreatedAt:            time.Now().UTC(),
	})
	if err != nil {
		return StripeWebhookResult{}, err
	}
	result.FinalTransaction = &finalTxn

	if targetStatus != ledger.StatusSucceeded {
		return result, nil
	}

	state, rewardUnlocked, err := s.loyaltyService.ApplyPaidVisit(ctx, tx, pendingTxn.TenantID, customerID, time.Now().UTC())
	if err != nil {
		return StripeWebhookResult{}, err
	}

	rewardUnlockTxn, err := s.appendRewardUnlockTransaction(ctx, tx, finalTxn, rewardUnlocked, state.UpdatedAt)
	if err != nil {
		return StripeWebhookResult{}, err
	}
	if rewardUnlockTxn != nil {
		result.RewardUnlockTxnID = rewardUnlockTxn.ID
	}

	return result, nil
}

func (s *Service) processStripeRefundEvent(ctx context.Context, tx pgx.Tx, event PaymentWebhookEvent, result StripeWebhookResult) (StripeWebhookResult, error) {
	refund, err := parsePaymentRefundObject(event.Data)
	if err != nil {
		return StripeWebhookResult{}, err
	}

	sessionID := strings.TrimSpace(refund.Metadata["stripe_session_id"])
	tenantID := strings.TrimSpace(refund.Metadata["tenant_id"])
	if sessionID == "" {
		result.Ignored = true
		return result, nil
	}

	visitTxn, err := s.findSucceededStripeVisitTransaction(ctx, tx, sessionID, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			result.Ignored = true
			return result, nil
		}
		return StripeWebhookResult{}, err
	}

	refundTxn, exists, err := s.findRefundTransactionForVisit(ctx, tx, visitTxn.ID)
	if err != nil {
		return StripeWebhookResult{}, err
	}
	if exists {
		result.Deduplicated = true
		result.FinalTransaction = &refundTxn
		return result, nil
	}

	amount := visitTxn.AmountCents
	if refund.Amount > 0 {
		amount = refund.Amount
	}

	metadata, err := json.Marshal(map[string]any{
		"stripe_event_id":       event.ID,
		"stripe_refund_id":      strings.TrimSpace(refund.ID),
		"stripe_session_id":     sessionID,
		"stripe_payment_intent": strings.TrimSpace(refund.PaymentIntent),
		"source":                "stripe_webhook_refund",
	})
	if err != nil {
		return StripeWebhookResult{}, fmt.Errorf("marshal refund metadata: %w", err)
	}

	relatedID := visitTxn.ID
	createdAt := time.Now().UTC()
	refundRecord, err := s.ledgerService.Append(ctx, tx, ledger.CreateInput{
		TenantID:             visitTxn.TenantID,
		CustomerID:           visitTxn.CustomerID,
		Type:                 ledger.TypeRefund,
		PaymentMethod:        ledger.PaymentMethodStripe,
		Status:               ledger.StatusRefunded,
		AmountCents:          amount,
		Currency:             normalizeCurrency(coalesceNonEmpty(refund.Currency, visitTxn.Currency)),
		PlatformFeeCents:     0,
		ExternalProvider:     providerStripe,
		ExternalReference:    strings.TrimSpace(refund.ID),
		RelatedTransactionID: &relatedID,
		MetadataJSON:         metadata,
		CreatedAt:            createdAt,
	})
	if err != nil {
		return StripeWebhookResult{}, err
	}

	// TODO(refund-loyalty): apply deterministic loyalty compensation when constrained refund flow is implemented.
	result.FinalTransaction = &refundRecord
	return result, nil
}

func (s *Service) appendRewardUnlockTransaction(ctx context.Context, tx pgx.Tx, triggerTxn ledger.Transaction, rewardUnlocked bool, at time.Time) (*ledger.Transaction, error) {
	if !rewardUnlocked {
		return nil, nil
	}
	if triggerTxn.CustomerID == nil {
		return nil, fmt.Errorf("%w: missing customer_id for reward unlock", domain.ErrInvalidRequest)
	}

	relatedID := triggerTxn.ID
	rewardMetadata, err := json.Marshal(map[string]any{
		"trigger_transaction_id": triggerTxn.ID,
		"reason":                 "stamp_threshold_reached",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal reward metadata: %w", err)
	}

	txn, err := s.ledgerService.Append(ctx, tx, ledger.CreateInput{
		TenantID:             triggerTxn.TenantID,
		CustomerID:           triggerTxn.CustomerID,
		Type:                 ledger.TypeRewardUnlock,
		PaymentMethod:        ledger.PaymentMethodManual,
		Status:               ledger.StatusSucceeded,
		AmountCents:          0,
		Currency:             triggerTxn.Currency,
		PlatformFeeCents:     0,
		RelatedTransactionID: &relatedID,
		MetadataJSON:         rewardMetadata,
		CreatedAt:            at,
	})
	if err != nil {
		return nil, err
	}

	return &txn, nil
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

func (s *Service) findPendingStripeVisitTransaction(ctx context.Context, tx pgx.Tx, sessionID, tenantID string) (ledger.Transaction, error) {
	query := `
		SELECT id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
		FROM transactions
		WHERE external_provider = $1
		  AND external_reference = $2
		  AND type = $3
		  AND payment_method = $4
		  AND status = $5
		  AND ($6 = '' OR tenant_id = $6)
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`

	var out ledger.Transaction
	err := tx.QueryRow(ctx, query,
		providerStripe,
		sessionID,
		ledger.TypeVisitPayment,
		ledger.PaymentMethodStripe,
		ledger.StatusPending,
		tenantID,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.Type,
		&out.PaymentMethod,
		&out.Status,
		&out.AmountCents,
		&out.Currency,
		&out.PlatformFeeCents,
		&out.ExternalProvider,
		&out.ExternalReference,
		&out.RelatedTransactionID,
		&out.MetadataJSON,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ledger.Transaction{}, domain.ErrNotFound
		}
		return ledger.Transaction{}, fmt.Errorf("find pending stripe visit transaction: %w", err)
	}

	return out, nil
}

func (s *Service) findSucceededStripeVisitTransaction(ctx context.Context, tx pgx.Tx, sessionID, tenantID string) (ledger.Transaction, error) {
	query := `
		SELECT id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
		FROM transactions
		WHERE external_provider = $1
		  AND external_reference = $2
		  AND type = $3
		  AND payment_method = $4
		  AND status = $5
		  AND ($6 = '' OR tenant_id = $6)
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`

	var out ledger.Transaction
	err := tx.QueryRow(ctx, query,
		providerStripe,
		sessionID,
		ledger.TypeVisitPayment,
		ledger.PaymentMethodStripe,
		ledger.StatusSucceeded,
		tenantID,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.Type,
		&out.PaymentMethod,
		&out.Status,
		&out.AmountCents,
		&out.Currency,
		&out.PlatformFeeCents,
		&out.ExternalProvider,
		&out.ExternalReference,
		&out.RelatedTransactionID,
		&out.MetadataJSON,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ledger.Transaction{}, domain.ErrNotFound
		}
		return ledger.Transaction{}, fmt.Errorf("find succeeded stripe visit transaction: %w", err)
	}

	return out, nil
}

func (s *Service) findRefundTransactionForVisit(ctx context.Context, tx pgx.Tx, visitTxnID string) (ledger.Transaction, bool, error) {
	query := `
		SELECT id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
		FROM transactions
		WHERE related_transaction_id = $1
		  AND type = $2
		  AND payment_method = $3
		  AND status = $4
		ORDER BY created_at DESC
		LIMIT 1
	`

	var out ledger.Transaction
	err := tx.QueryRow(ctx, query,
		visitTxnID,
		ledger.TypeRefund,
		ledger.PaymentMethodStripe,
		ledger.StatusRefunded,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.Type,
		&out.PaymentMethod,
		&out.Status,
		&out.AmountCents,
		&out.Currency,
		&out.PlatformFeeCents,
		&out.ExternalProvider,
		&out.ExternalReference,
		&out.RelatedTransactionID,
		&out.MetadataJSON,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ledger.Transaction{}, false, nil
		}
		return ledger.Transaction{}, false, fmt.Errorf("find refund transaction for visit: %w", err)
	}

	return out, true, nil
}

func (s *Service) findFinalizedStripeTransactionForPending(ctx context.Context, tx pgx.Tx, pendingID string) (ledger.Transaction, bool, error) {
	query := `
		SELECT id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
		FROM transactions
		WHERE related_transaction_id = $1
		  AND type = $2
		  AND payment_method = $3
		  AND status IN ($4, $5, $6)
		ORDER BY created_at DESC
		LIMIT 1
	`

	var out ledger.Transaction
	err := tx.QueryRow(ctx, query,
		pendingID,
		ledger.TypeVisitPayment,
		ledger.PaymentMethodStripe,
		ledger.StatusSucceeded,
		ledger.StatusFailed,
		ledger.StatusCanceled,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.Type,
		&out.PaymentMethod,
		&out.Status,
		&out.AmountCents,
		&out.Currency,
		&out.PlatformFeeCents,
		&out.ExternalProvider,
		&out.ExternalReference,
		&out.RelatedTransactionID,
		&out.MetadataJSON,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ledger.Transaction{}, false, nil
		}
		return ledger.Transaction{}, false, fmt.Errorf("find finalized stripe visit transaction: %w", err)
	}

	return out, true, nil
}

func assertCustomerPayable(ctx context.Context, tx pgx.Tx, tenantID, customerID string) error {
	query := `
		SELECT is_archived
		FROM customers
		WHERE tenant_id = $1 AND id = $2
	`

	var isArchived bool
	err := tx.QueryRow(ctx, query, tenantID, customerID).Scan(&isArchived)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("check customer: %w", err)
	}
	if isArchived {
		return fmt.Errorf("%w: archived customers cannot receive payments", domain.ErrInvalidRequest)
	}

	return nil
}

func targetStatusForStripeEvent(eventType string) (string, bool) {
	switch strings.TrimSpace(eventType) {
	case stripeEventCheckoutCompleted, stripeEventCheckoutAsyncSucceeded:
		return ledger.StatusSucceeded, true
	case stripeEventCheckoutAsyncFailed:
		return ledger.StatusFailed, true
	case stripeEventCheckoutExpired:
		return ledger.StatusCanceled, true
	default:
		return "", false
	}
}

func calculatePlatformFeeCents(amountCents int64, commissionBPS int) int64 {
	if amountCents <= 0 || commissionBPS <= 0 {
		return 0
	}
	return (amountCents * int64(commissionBPS)) / 10000
}

func normalizeCurrency(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return ledger.DefaultCurrency
	}
	return value
}

func coalesceNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return strings.TrimSpace(fallback)
}
