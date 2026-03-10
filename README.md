# Barber Loyalties Monorepo

Mobile-first SaaS MVP for barbers and small salons.

## Stack
- Mobile: React Native + TypeScript + Tamagui
- Backend: Go + Gin + PostgreSQL
- Payments: Stripe Billing + Stripe Checkout

## Repository layout
- `backend/`: Go API, migrations, domain services
- `mobile/`: React Native app structure
- `docs/`: architecture notes, test checklists

## Current status
- Foundation + cash/ledger/loyalty path complete
- Stripe customer payments checkout + webhook idempotency complete
- Stripe billing checkout + billing webhook idempotency complete
- Stripe customer portal session endpoint complete
- Subscription-aware transactional gating complete

## Quick start (backend)
1. Copy `.env.example` to `.env` in `backend/`
2. Select provider modes:
   - `PAYMENTS_PROVIDER=fake|stripe`
   - `BILLING_PROVIDER=fake|stripe`
3. Create PostgreSQL database
4. Run migrations in `backend/migrations`
5. Start API:
   - `go run ./cmd/api`

## MVP non-negotiables
- `tenant_id` on all relevant data
- append-only transactions
- loyalty is a projection, rebuildable from ledger
- Stripe webhook idempotency
