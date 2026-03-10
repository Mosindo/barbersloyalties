# Migrations

This project currently uses SQL-first migrations.

## Apply (manual)
Run `0001_init.up.sql` against your target database.

## Rollback (manual)
Run `0001_init.down.sql`.

A migration runner (Goose/Atlas) can be added in the next phase if needed.
