# Progress Status

## Completed
- Monorepo skeleton created (`backend`, `mobile`, `docs`)
- Root `AGENTS.md` added with mission, architecture, and scope guardrails
- Backend service bootstrapped in Go + Gin
- Postgres pool/config and environment loading implemented
- Tenant-scoped auth endpoints implemented (`register`, `login`, `me`)
- Customer CRUD + archive endpoints implemented with tenant isolation
- Ledger domain implemented with append-only writes
- Cash visit flow implemented with atomic DB transaction:
  - `visit_payment` transaction append
  - loyalty projection update
  - `reward_unlock` transaction append when threshold reached
- Stripe customer checkout endpoint implemented with pending ledger transaction creation
- Stripe payments webhook implemented with:
  - signature verification
  - `processed_webhooks` idempotency
  - pending->final transaction finalization (succeeded/failed/canceled) via append-only events
  - loyalty projection updates on succeeded payments
- Subscription domain implemented (`subscriptions` table service/repository)
- Stripe billing checkout endpoint implemented
- Stripe billing webhook implemented with idempotency and status sync
- Transactional endpoint gating implemented (requires active/trialing subscription)
- Transaction list/detail endpoints implemented
- Dashboard summary endpoint implemented
- Default stamp-card loyalty config auto-created on onboarding
- Migrations include core schema + unique index for one subscription row per tenant
- Unit tests added for auth token flow, customer service validation, loyalty rules, ledger validation, Stripe signature/commission helpers, and subscription status rules

## Next priority (Cloud Task 4)
- Reward redemption endpoint + UI integration
- Full refund flow (MVP constraints) with compensating ledger transaction
- Payment-method correction flow (MVP constraints)
- Integration tests for refund/correction and loyalty consistency

## Risks / assumptions
- Register flow currently creates tenant + user without one explicit DB transaction across repositories.
- Billing webhook handling is robust for common Stripe events but not yet backed by DB integration tests.
- Refund and payment-method correction flows are not implemented yet.
