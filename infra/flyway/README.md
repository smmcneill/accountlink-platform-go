# Flyway Config

Per-environment config files are expected at:
- `infra/flyway/dev.conf`
- `infra/flyway/test.conf`
- `infra/flyway/prod.conf`

Starter files are checked in with `REPLACE_ME_*` placeholders.
Update `flyway.url`, `flyway.user`, and `flyway.password` before running migrations.

`just flyway-migrate <env>` reads `infra/flyway/<env>.conf`.
