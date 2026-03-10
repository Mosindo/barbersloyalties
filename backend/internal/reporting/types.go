package reporting

import "time"

type DashboardSummary struct {
	TenantID              string    `json:"tenant_id"`
	GeneratedAt           time.Time `json:"generated_at"`
	CustomersCount        int64     `json:"customers_count"`
	PaidVisitsCount       int64     `json:"paid_visits_count"`
	CashRevenueCents      int64     `json:"cash_revenue_cents"`
	StripeRevenueCents    int64     `json:"stripe_revenue_cents"`
	PlatformFeeTotalCents int64     `json:"platform_fee_total_cents"`
	AvailableRewardsTotal int64     `json:"available_rewards_total"`
	UsedRewardsTotal      int64     `json:"used_rewards_total"`
}
