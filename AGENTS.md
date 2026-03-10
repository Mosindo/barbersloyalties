# AGENTS.md

## Product mission
Build a mobile-first SaaS for barbers with subscription billing, customer management, cash and Stripe payments, immutable transaction history, and a simple loyalty system.

## Delivery priorities
1. Correctness of transactions
2. Correctness of loyalty state
3. Stripe webhook idempotency
4. Multi-tenant isolation
5. Mobile UX speed
6. Clean, testable structure

## Architecture rules
- Every domain entity must be tenant-scoped.
- Transactions are append-only.
- Never delete transaction history.
- Loyalty state is a projection, not source of truth.
- Keep business logic in backend services, not handlers or UI.
- Webhook handlers must be idempotent.
- Prefer explicit domain services over scattered utility logic.

## MVP scope guardrails
- One loyalty model only: stamp card.
- No booking system.
- No multi-employee support.
- No multi-location UX.
- No partial refunds.
- Keep payment-method correction constrained.

## Coding expectations
- Small, reviewable commits
- Strong typing and clear error handling
- Add tests for critical transaction flows
- Keep naming explicit and domain-oriented
- Prefer readability over cleverness

## Validation expectations
Before marking work complete:
- run tests
- validate the happy path manually
- describe assumptions and known gaps
