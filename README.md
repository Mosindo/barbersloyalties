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
- Phase 1 in progress: foundation + multi-tenant auth + customers

## Quick start (backend)
1. Copy `.env.example` to `.env` in `backend/`
2. Create PostgreSQL database
3. Run migrations in `backend/migrations`
4. Start API:
   - `go run ./cmd/api`

## MVP non-negotiables
- `tenant_id` on all relevant data
- append-only transactions
- loyalty is a projection, rebuildable from ledger
- Stripe webhook idempotency
