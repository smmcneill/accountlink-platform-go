# accountlink-platform-go

Go translation of `accountlink-platform` (Java/Spring/Maven).

## Prerequisites
- Go 1.25+
- `just` (used for project scripts)
- Docker (for container and local infra commands)
- `pre-commit` (optional, for local commit-time checks)

Install `just`:
```bash
brew install just
```

On macOS/Linux with Go toolchain:
```bash
go install github.com/casey/just@latest
```

Install `pre-commit`:
```bash
brew install pre-commit
```

Enable hooks in this repo:
```bash
pre-commit install
```

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

## Container Image
- `Dockerfile` is a distroless Debian runtime image (`gcr.io/distroless/static-debian12:nonroot`) that copies a prebuilt binary from `build/server`.
- `just build-linux-binary` compiles a Linux static binary into `build/server`.
- `just image-build` runs `build-linux-binary` first, then builds the runtime image.
- CDK deploy uses `fromAsset` with `Dockerfile`; `just cdk-synth` and `just cdk-deploy` run `build-linux-binary` automatically first.
- Build locally:

```bash
just image-build
# or
just image-build tag=123456789012.dkr.ecr.us-east-1.amazonaws.com/accountlink:latest
just image-build platform=linux/arm64
```

## Env vars
- `PORT` (default `8080`)
- `DB_DSN` (default `postgres://accountlink:accountlink@localhost:5444/accountlink?sslmode=disable`)
- Optional split DB settings when `DB_DSN` is not set:
  `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE`
- `DB_STARTUP_MAX_WAIT_MS` (default `300000`)
- `DB_STARTUP_RETRY_MS` (default `5000`)
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
- Uses two stacks:
  - foundation stack: VPC + Aurora PostgreSQL (Serverless v2)
  - service stack: ECS Fargate + HTTP Application Load Balancer
- Flyway config files should live in `infra/flyway/<env>.conf`.
- Environment/account config for `dev`, `test`, `prod` lives in `infra/cdk/lib/config/`.
- Stack outputs include Aurora endpoint/port/secret ARN for wiring DB connectivity and migrations.

### Required deployment env vars
- None required for synth/deploy if env config is populated.
- Optional overrides: `IMAGE_ASSET_PATH`, `IMAGE_ASSET_DOCKERFILE`, `AWS_ACCOUNT_ID`, `AWS_REGION`, `VPC_ID`, `CONTAINER_PORT`, `DESIRED_COUNT`, `APP_NAME`, `ENV_NAME`

### Commands
```bash
just setup
source local.env
just flyway-migrate
source dev.env
just cdk-deploy
just release
```

### Mutation testing
- `internal/app` and `internal/domain/accountlink.go` are included in the mutation check.
- Run locally:
- `just test-go` already runs unit tests plus mutation.
- After running `just setup`, you can run just mutation with:
```bash
just mutation
```
- Or install manually:
```bash
go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest
go-mutesting ./internal/app ./internal/domain/accountlink.go
```
- Same command is executed in CI after unit tests, so regressions are caught automatically.
