package reporting

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/barbersloyalties/backend/internal/ledger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) DashboardSummary(ctx context.Context, tenantID string) (DashboardSummary, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return DashboardSummary{}, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}

	query := `
		SELECT
			(SELECT COUNT(*) FROM customers WHERE tenant_id = $1 AND is_archived = false) AS customers_count,
			(SELECT COUNT(*) FROM transactions
				WHERE tenant_id = $1 AND type = $2 AND status = $3) AS paid_visits_count,
			(SELECT COALESCE(SUM(amount_cents), 0) FROM transactions
				WHERE tenant_id = $1 AND type = $2 AND status = $3 AND payment_method = $4) AS cash_revenue_cents,
			(SELECT COALESCE(SUM(amount_cents), 0) FROM transactions
				WHERE tenant_id = $1 AND type = $2 AND status = $3 AND payment_method = $5) AS stripe_revenue_cents,
			(SELECT COALESCE(SUM(platform_fee_cents), 0) FROM transactions
				WHERE tenant_id = $1 AND type = $2 AND status = $3) AS platform_fee_total_cents,
			(SELECT COALESCE(SUM(available_rewards), 0) FROM customer_loyalty_states WHERE tenant_id = $1) AS available_rewards_total,
			(SELECT COALESCE(SUM(used_rewards), 0) FROM customer_loyalty_states WHERE tenant_id = $1) AS used_rewards_total
	`

	var out DashboardSummary
	err := s.pool.QueryRow(ctx, query,
		tenantID,
		ledger.TypeVisitPayment,
		ledger.StatusSucceeded,
		ledger.PaymentMethodCash,
		ledger.PaymentMethodStripe,
	).Scan(
		&out.CustomersCount,
		&out.PaidVisitsCount,
		&out.CashRevenueCents,
		&out.StripeRevenueCents,
		&out.PlatformFeeTotalCents,
		&out.AvailableRewardsTotal,
		&out.UsedRewardsTotal,
	)
	if err != nil {
		return DashboardSummary{}, fmt.Errorf("query dashboard summary: %w", err)
	}

	out.TenantID = tenantID
	out.GeneratedAt = time.Now().UTC()
	return out, nil
}
