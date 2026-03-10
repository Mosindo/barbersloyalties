package subscriptions

import "time"

const (
	StatusInactive         = "inactive"
	StatusActive           = "active"
	StatusTrialing         = "trialing"
	StatusPastDue          = "past_due"
	StatusCanceled         = "canceled"
	StatusIncomplete       = "incomplete"
	StatusIncompleteExpire = "incomplete_expired"
	StatusUnpaid           = "unpaid"
)

type Subscription struct {
	ID                   string     `json:"id"`
	TenantID             string     `json:"tenant_id"`
	StripeCustomerID     string     `json:"stripe_customer_id"`
	StripeSubscriptionID string     `json:"stripe_subscription_id"`
	Status               string     `json:"status"`
	MonthlyPriceCents    int64      `json:"monthly_price_cents"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type UpsertByTenantInput struct {
	TenantID             string
	StripeCustomerID     string
	StripeSubscriptionID string
	Status               string
	MonthlyPriceCents    int64
	CurrentPeriodEnd     *time.Time
}

type UpdateByStripeRefsInput struct {
	StripeSubscriptionID string
	StripeCustomerID     string
	Status               string
	CurrentPeriodEnd     *time.Time
}
