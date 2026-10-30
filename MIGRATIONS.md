# Migrations policy

This repo uses **expand/contract** migrations for zero-downtime schema evolution.

## Down migrations are banned

We do **not** ship `*.down.sql` files. In production, "rollback" is performed by
deploying a fixed forward migration.

CI enforces this policy via `make lint-migrations`.

## Expand/contract checklist (zero-downtime)

When you need to change schema without breaking running code:

1. **Expand**
   - Add new column/table/index in a backwards compatible way.
   - New columns should start **NULLABLE** or with safe defaults.
2. **Backfill** (if needed)
   - Backfill in a separate migration or a background job.
3. **Switch**
   - Deploy code that reads/writes the new schema.
4. **Contract**
   - Enforce constraints (`NOT NULL`, `UNIQUE`, drop old columns) only after the
     system is fully on the new path.

## CI gate

`make migrate-auth-smoke` spins up a **fresh Postgres**, runs migrations, and
executes a basic smoke query. This catches "works locally" migration issues.
