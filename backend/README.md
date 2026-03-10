# Backend (Go + Gin)

## What is implemented in this phase
- Tenant-scoped auth (register/login/me)
- Customer CRUD + archive (tenant-scoped)
- Default loyalty config creation during onboarding
- Initial Postgres schema migration for MVP entities
- Structured JSON logs + request ID middleware

## Endpoints implemented
- `GET /health`
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/logout`
- `GET /me`
- `GET /customers`
- `POST /customers`
- `GET /customers/:id`
- `PATCH /customers/:id`
- `POST /customers/:id/archive`

## Setup
1. Copy `.env.example` to `.env`
2. Provision Postgres database
3. Apply SQL in `migrations/0001_init.up.sql`
4. Run API:
   - `go run ./cmd/api`

## Tests
- `go test ./...`
