CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY,
    business_name TEXT NOT NULL,
    owner_name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    stripe_customer_id TEXT NOT NULL DEFAULT '',
    stripe_subscription_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    monthly_price_cents INTEGER NOT NULL,
    current_period_end TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    full_name TEXT NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    is_archived BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS loyalty_configs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    stamp_threshold INTEGER NOT NULL,
    reward_value INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS customer_loyalty_states (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    stamps_count INTEGER NOT NULL DEFAULT 0,
    available_rewards INTEGER NOT NULL DEFAULT 0,
    used_rewards INTEGER NOT NULL DEFAULT 0,
    total_paid_visits INTEGER NOT NULL DEFAULT 0,
    last_visit_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, customer_id)
);

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    customer_id UUID REFERENCES customers(id),
    type TEXT NOT NULL CHECK (type IN (
        'subscription_payment',
        'visit_payment',
        'reward_unlock',
        'reward_redemption',
        'refund',
        'payment_method_change_refund',
        'payment_method_change_replacement',
        'manual_adjustment'
    )),
    payment_method TEXT NOT NULL CHECK (payment_method IN ('cash', 'stripe', 'manual')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded', 'canceled')),
    amount_cents INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'EUR',
    platform_fee_cents INTEGER NOT NULL DEFAULT 0,
    external_provider TEXT NOT NULL DEFAULT '',
    external_reference TEXT NOT NULL DEFAULT '',
    related_transaction_id UUID REFERENCES transactions(id),
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS processed_webhooks (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL,
    event_id TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL,
    UNIQUE (provider, event_id)
);

CREATE INDEX IF NOT EXISTS idx_users_tenant_email ON users(tenant_id, email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower_unique ON users((lower(email)));
CREATE INDEX IF NOT EXISTS idx_customers_tenant_phone ON customers(tenant_id, phone);
CREATE INDEX IF NOT EXISTS idx_customers_tenant_created_at ON customers(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_transactions_tenant_created_desc ON transactions(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_tenant_customer_created_desc ON transactions(tenant_id, customer_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_processed_webhooks_provider_event ON processed_webhooks(provider, event_id);
