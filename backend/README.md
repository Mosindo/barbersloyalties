# Backend (Go + Gin)

## Implemented MVP backend
- Tenant-scoped auth (`register`, `login`, `me`)
- Tenant-scoped customers CRUD + archive
- Immutable transaction ledger (append-only)
- Loyalty projection (`customer_loyalty_states`) updated from backend services only
- Cash payments flow
- Stripe customer payments flow (Checkout + payments webhook + idempotency)
- Stripe Billing subscriptions flow (Checkout mode=subscription + billing webhook + idempotency)
- Subscription gating middleware (`active` or `trialing` required for business endpoints)
- Dashboard summary endpoint
- Structured logs + request IDs

## Providers architecture (fake + stripe)
Business services do not branch on fake vs real logic. They depend on provider interfaces:
- `billing.BillingProvider`
- `payments.PaymentsProvider`

Available implementations:
- `FakeStripeProvider` (local MVP speed)
- `StripeProvider` (real adapter, production path)

Provider selection is env-driven:
- `PAYMENTS_PROVIDER=fake|stripe`
- `BILLING_PROVIDER=fake|stripe`

## Billing endpoints
- `POST /billing/create-subscription-checkout`
- `GET /billing/subscription`
- `POST /billing/create-portal-session`
- `POST /webhooks/stripe/billing`

## Payments endpoints
- `POST /customers/:id/create-stripe-checkout`
- `POST /webhooks/stripe/payments`

Business endpoints requiring active subscription:
- `GET /customers`
- `POST /customers`
- `GET /customers/:id`
- `PATCH /customers/:id`
- `POST /customers/:id/archive`
- `POST /customers/:id/pay-cash`
- `POST /customers/:id/create-stripe-checkout`
- `GET /transactions`
- `GET /transactions/:id`
- `GET /dashboard/summary`

## Dev fake webhook endpoints
Available only when:
- `APP_ENV != production`
- provider mode for the domain is `fake`

Billing:
- `POST /dev/fake-stripe/billing/complete-checkout`
- `POST /dev/fake-stripe/billing/payment-failed`

Payments:
- `POST /dev/fake-stripe/payments/complete-checkout`
- `POST /dev/fake-stripe/payments/payment-failed`
- `POST /dev/fake-stripe/payments/refund`

These endpoints go through the same webhook handlers and state transitions as normal webhook routes.
Fake webhook payloads mimic Stripe event envelopes:
- `object: "event"`
- `api_version`
- `created`
- `livemode`
- `pending_webhooks`
- `data.object` with Stripe-like object types (`checkout.session`, `subscription`, `charge`)

## Required environment variables
- `PAYMENTS_PROVIDER` (`fake` or `stripe`)
- `BILLING_PROVIDER` (`fake` or `stripe`)
- `STRIPE_SECRET_KEY`
- `STRIPE_PUBLISHABLE_KEY`
- `STRIPE_WEBHOOK_SECRET`
- `STRIPE_SUBSCRIPTION_PRICE_ID`
- `APP_BASE_URL`
- `MOBILE_SUCCESS_URL`
- `MOBILE_CANCEL_URL`

Also used for customer payments webhooks:
- `STRIPE_WEBHOOK_SECRET_PAYMENTS` (optional, fallback to `STRIPE_WEBHOOK_SECRET`)

## Fake mode quickstart (recommended for local MVP)
1. Set:
   - `PAYMENTS_PROVIDER=fake`
   - `BILLING_PROVIDER=fake`
2. Keep Stripe keys empty if you want full local-only simulation.
3. Use `/dev/fake-stripe/*` endpoints to trigger subscription/payment/refund transitions.

Fake provider generates Stripe-like IDs:
- `cs_test_mock_xxx`
- `sub_mock_xxx`
- `cus_mock_xxx`
- `pi_mock_xxx`

## Stripe real mode setup (Quickstart-aligned)
1. In Stripe Dashboard, create product `Barber Loyalty SaaS`.
2. Create recurring monthly price `9 EUR`.
3. Copy `price_id` to `STRIPE_SUBSCRIPTION_PRICE_ID`.
4. Configure webhook endpoint `POST /webhooks/stripe/billing` and subscribe to:
   - `checkout.session.completed`
   - `customer.subscription.created`
   - `customer.subscription.updated`
   - `customer.subscription.deleted`
   - `invoice.paid`
   - `invoice.payment_failed`
5. Put endpoint signing secret into `STRIPE_WEBHOOK_SECRET`.
6. Set:
   - `PAYMENTS_PROVIDER=stripe`
   - `BILLING_PROVIDER=stripe`

Webhook handling is idempotent via `processed_webhooks`.
Frontend must read subscription state from `GET /billing/subscription` only.

## Passer de fake a Stripe reel
Nothing changes in domain services, handlers, SQL schema, or ledger/loyalty logic.

Migration fake -> Stripe reel:
1. Renseigner `STRIPE_SECRET_KEY`
2. Renseigner `STRIPE_WEBHOOK_SECRET`
3. Renseigner `STRIPE_SUBSCRIPTION_PRICE_ID`
4. Basculer `BILLING_PROVIDER=stripe`
5. Basculer `PAYMENTS_PROVIDER=stripe`
6. Configurer les endpoints Stripe Checkout/Billing:
   - `/webhooks/stripe/payments`
   - `/webhooks/stripe/billing`
7. Tester avec Stripe test mode + Stripe CLI

Optional TODOs for full production parity:
- Implement real Stripe refunds in `payments.StripeProvider.RefundPayment`.
- Add dedicated integration tests with Stripe test mode + Stripe CLI replay.

## Local setup
1. Copy `.env.example` to `.env`
2. Provision PostgreSQL
3. Apply:
   - `migrations/0001_init.up.sql`
   - `migrations/0002_subscriptions_tenant_unique.up.sql`
4. Run API:
   - `go run ./cmd/api`

## Tests
- `go test ./...`
