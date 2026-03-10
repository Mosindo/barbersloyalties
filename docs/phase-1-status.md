# Phase 1 Status

## Completed
- Monorepo skeleton created (`backend`, `mobile`, `docs`)
- Root `AGENTS.md` added with mission, architecture, and scope guardrails
- Backend service bootstrapped in Go + Gin
- Postgres pool/config and environment loading implemented
- Tenant-scoped auth endpoints implemented (`register`, `login`, `me`)
- Customer CRUD + archive endpoints implemented with tenant isolation
- Default stamp-card loyalty config auto-created on onboarding
- Initial migration includes all core MVP tables and key indexes
- Unit tests added for auth token flow and customer service validation

## Remaining for Cloud Task 1 completion
- Tenant isolation integration tests (HTTP + DB)
- Auth integration tests (`register/login/me` with real DB)
- Customer search pagination edge-case tests
- Subscription gating middleware behavior

## Risks / assumptions
- Register flow currently creates tenant + user without explicit SQL transaction wrapping across repositories.
- Users are globally unique by lowercase email for MVP simplicity (enforced by migration index).
- Stripe and ledger flows are intentionally deferred to Cloud Task 2/3.
