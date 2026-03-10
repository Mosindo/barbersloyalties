# Migrations

This project currently uses SQL-first migrations.

## Apply (manual)
1. Run `0001_init.up.sql`
2. Run `0002_subscriptions_tenant_unique.up.sql`

## Rollback (manual)
1. Run `0002_subscriptions_tenant_unique.down.sql`
2. Run `0001_init.down.sql`

A migration runner (Goose/Atlas) can be added in the next phase if needed.
