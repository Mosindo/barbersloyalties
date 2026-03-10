package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetByTenantID(ctx context.Context, q queryable, tenantID string) (Subscription, error) {
	if q == nil {
		q = r.pool
	}

	query := `
		SELECT id, tenant_id, stripe_customer_id, stripe_subscription_id, status,
			monthly_price_cents, current_period_end, created_at, updated_at
		FROM subscriptions
		WHERE tenant_id = $1
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var out Subscription
	err := q.QueryRow(ctx, query, strings.TrimSpace(tenantID)).Scan(
		&out.ID,
		&out.TenantID,
		&out.StripeCustomerID,
		&out.StripeSubscriptionID,
		&out.Status,
		&out.MonthlyPriceCents,
		&out.CurrentPeriodEnd,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subscription{}, domain.ErrNotFound
		}
		return Subscription{}, fmt.Errorf("get subscription by tenant id: %w", err)
	}

	return out, nil
}

func (r *Repository) UpsertByTenant(ctx context.Context, q queryable, input UpsertByTenantInput) (Subscription, error) {
	if q == nil {
		q = r.pool
	}

	query := `
		INSERT INTO subscriptions (
			id, tenant_id, stripe_customer_id, stripe_subscription_id, status,
			monthly_price_cents, current_period_end, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		ON CONFLICT (tenant_id) DO UPDATE
		SET stripe_customer_id = EXCLUDED.stripe_customer_id,
			stripe_subscription_id = EXCLUDED.stripe_subscription_id,
			status = EXCLUDED.status,
			monthly_price_cents = EXCLUDED.monthly_price_cents,
			current_period_end = EXCLUDED.current_period_end,
			updated_at = EXCLUDED.updated_at
		RETURNING id, tenant_id, stripe_customer_id, stripe_subscription_id, status,
			monthly_price_cents, current_period_end, created_at, updated_at
	`

	now := time.Now().UTC()
	var out Subscription
	err := q.QueryRow(ctx, query,
		uuid.NewString(),
		strings.TrimSpace(input.TenantID),
		strings.TrimSpace(input.StripeCustomerID),
		strings.TrimSpace(input.StripeSubscriptionID),
		strings.TrimSpace(input.Status),
		input.MonthlyPriceCents,
		input.CurrentPeriodEnd,
		now,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.StripeCustomerID,
		&out.StripeSubscriptionID,
		&out.Status,
		&out.MonthlyPriceCents,
		&out.CurrentPeriodEnd,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return Subscription{}, fmt.Errorf("upsert subscription by tenant: %w", err)
	}

	return out, nil
}

func (r *Repository) UpdateByStripeRefs(ctx context.Context, q queryable, input UpdateByStripeRefsInput) (Subscription, error) {
	if q == nil {
		q = r.pool
	}

	query := `
		UPDATE subscriptions
		SET status = $3,
			current_period_end = $4,
			updated_at = $5,
			stripe_customer_id = CASE WHEN $2 <> '' THEN $2 ELSE stripe_customer_id END
		WHERE stripe_subscription_id = $1
		   OR ($1 = '' AND stripe_customer_id = $2)
		RETURNING id, tenant_id, stripe_customer_id, stripe_subscription_id, status,
			monthly_price_cents, current_period_end, created_at, updated_at
	`

	var out Subscription
	err := q.QueryRow(ctx, query,
		strings.TrimSpace(input.StripeSubscriptionID),
		strings.TrimSpace(input.StripeCustomerID),
		strings.TrimSpace(input.Status),
		input.CurrentPeriodEnd,
		time.Now().UTC(),
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.StripeCustomerID,
		&out.StripeSubscriptionID,
		&out.Status,
		&out.MonthlyPriceCents,
		&out.CurrentPeriodEnd,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subscription{}, domain.ErrNotFound
		}
		return Subscription{}, fmt.Errorf("update subscription by stripe refs: %w", err)
	}

	return out, nil
}
