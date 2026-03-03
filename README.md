# accountlink-platform-go

Go translation of `accountlink-platform` (Java/Spring/Maven).

## Endpoints
- `GET /_health` -> `ok`
- `GET /account-links/{id}`
- `POST /account-links` with optional `Idempotency-Key`

## Behavior parity
- Same account link domain fields/status values.
- Idempotent create semantics using `idempotency_keys` table.
- Outbox write on create and periodic outbox publisher.
- `409 Conflict` when idempotency key is reused with a different payload.

## Run
```bash
go run ./cmd/server
```

## Env vars
- `PORT` (default `8080`)
- `DB_DSN` (default `postgres://accountlink:accountlink@localhost:5444/accountlink?sslmode=disable`)
- `EVENT_TARGET` (`logging` default, `sns` optional)
- `ACCOUNTLINK_SNS_TOPIC_ARN`
- `ACCOUNTLINK_SNS_ENDPOINT` (for localstack)
- `ACCOUNTLINK_SNS_REGION` (default `us-east-1`)
- `OUTBOX_POLL_DELAY_MS` (default `10000`)
- `OUTBOX_POLL_BATCH_SIZE` (default `100`)

## Local dependencies
Use the same docker compose as the Java project for Postgres + LocalStack.
