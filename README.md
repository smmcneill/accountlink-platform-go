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

## Cloud Deployment (CDK + Flyway)
- CDK code lives in `infra/cdk` (TypeScript).
- Stack targets ECS Fargate + Application Load Balancer + Route53 record.
- Flyway config files should live in `infra/flyway/<env>.conf`.

### Required deployment env vars
- `IMAGE_URI` (container image, e.g. ECR URI)
- `HOSTED_ZONE_DOMAIN` (e.g. `example.com`)
- `RECORD_NAME` (e.g. `api`; use empty string for zone apex)
- `CERTIFICATE_ARN` (ACM cert in same region as ALB)
- Optional: `VPC_ID`, `CONTAINER_PORT` (default `8080`), `DESIRED_COUNT` (default `1`), `APP_NAME`, `ENV_NAME`

### Commands
```bash
just setup
just flyway-migrate dev
just cdk-deploy dev
just release dev
```
