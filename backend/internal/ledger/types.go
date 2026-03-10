package ledger

import (
	"encoding/json"
	"time"
)

const (
	TypeSubscriptionPayment        = "subscription_payment"
	TypeVisitPayment               = "visit_payment"
	TypeRewardUnlock               = "reward_unlock"
	TypeRewardRedemption           = "reward_redemption"
	TypeRefund                     = "refund"
	TypePaymentMethodChangeRefund  = "payment_method_change_refund"
	TypePaymentMethodChangeReplace = "payment_method_change_replacement"
	TypeManualAdjustment           = "manual_adjustment"
	PaymentMethodCash              = "cash"
	PaymentMethodStripe            = "stripe"
	PaymentMethodManual            = "manual"
	StatusPending                  = "pending"
	StatusSucceeded                = "succeeded"
	StatusFailed                   = "failed"
	StatusRefunded                 = "refunded"
	StatusCanceled                 = "canceled"
	ExternalProviderStripe         = "stripe"
	ExternalProviderNone           = ""
	DefaultCurrency                = "EUR"
)

type Transaction struct {
	ID                   string          `json:"id"`
	TenantID             string          `json:"tenant_id"`
	CustomerID           *string         `json:"customer_id,omitempty"`
	Type                 string          `json:"type"`
	PaymentMethod        string          `json:"payment_method"`
	Status               string          `json:"status"`
	AmountCents          int64           `json:"amount_cents"`
	Currency             string          `json:"currency"`
	PlatformFeeCents     int64           `json:"platform_fee_cents"`
	ExternalProvider     string          `json:"external_provider"`
	ExternalReference    string          `json:"external_reference"`
	RelatedTransactionID *string         `json:"related_transaction_id,omitempty"`
	MetadataJSON         json.RawMessage `json:"metadata_json"`
	CreatedAt            time.Time       `json:"created_at"`
}

type CreateInput struct {
	TenantID             string
	CustomerID           *string
	Type                 string
	PaymentMethod        string
	Status               string
	AmountCents          int64
	Currency             string
	PlatformFeeCents     int64
	ExternalProvider     string
	ExternalReference    string
	RelatedTransactionID *string
	MetadataJSON         json.RawMessage
	CreatedAt            time.Time
}

type ListParams struct {
	TenantID      string
	CustomerID    string
	Type          string
	PaymentMethod string
	Status        string
	Limit         int
	Offset        int
}
