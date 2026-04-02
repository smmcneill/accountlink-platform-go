# AGENTS.md

Guidance for coding agents working in `accountlink-platform-go`.

## 1. Project Snapshot
- Service: Go implementation of Account Link platform APIs.
- Entry point: `cmd/server/main.go`.
- Core layers:
  - `internal/domain`: domain models and invariants.
  - `internal/app`: application services and outbox processor.
  - `internal/persistence`: Postgres repositories + idempotency/outbox stores.
  - `internal/api`: HTTP handlers.
  - `internal/events`: event publishers (`logging` and `sns`).
  - `internal/server`: HTTP server wiring.
  - `internal/db`: migration runner.
- Infra:
  - CDK (TypeScript): `infra/cdk`.
  - Flyway configs: `infra/flyway`.
  - SQL migrations: `migrations/` and `internal/db/migrations/`.

## 2. Working Style
- Prefer smallest safe change that solves the task.
- Follow existing package boundaries; avoid cross-layer leakage.
- Preserve behavior parity and idempotency semantics.
- Do not make unrelated refactors while fixing a focused issue.

## 3. Local Commands
Use `just` recipes whenever possible instead of ad-hoc commands.

- Run server: `just run`
- Format: `just fmt`
- Lint: `just lint`
- Lint with autofix: `just lint-fix`
- Go tests + mutation tests: `just test-go`
- Mutation tests only: `just mutation`
- Infra tests: `just test-infra`
- Full test suite: `just test`

If your change affects runtime behavior, run at least `just test-go` before finishing.

## 4. Quality Gate Expectations
- Lint must pass via `golangci-lint` (configured in `.golangci.yml`).
- Keep tests deterministic and focused.
- Add or update tests with code changes, especially in:
  - `internal/app`
  - `internal/domain`
  - `internal/api`
- Mutation coverage is enforced for critical packages; avoid superficial assertions.

## 5. Environment and Safety
- Environment-sensitive recipes require sourced env files (`local.env`, `dev.env`, etc.).
- Key env vars are documented in `README.md` (`DB_DSN`, `EVENT_TARGET`, SNS settings, outbox polling values, etc.).
- Be cautious with high-impact commands in `justfile`:
  - `docker-clean` removes all local Docker containers/images.
  - `cdk-undeploy` destroys deployed stacks.
- Never run destructive operations unless explicitly requested.

## 6. Deployment Notes
- `just build-linux-binary` creates `build/server` used by Docker/CDK.
- `just release` sequence is: foundation deploy -> flyway migrate -> service deploy.
- CDK commands expect `ENV` to be set by sourced env files.

## 7. Editing Guidance
- Keep logs and errors structured and actionable.
- Propagate context cancellation/timeouts where appropriate.
- For new config, thread values through `internal/config` and document in `README.md`.
- For new persistence behavior, validate with tests using existing repository/service test patterns.

## 8. Definition of Done
Before handing off:
1. Code is formatted and lint-clean.
2. Relevant tests pass locally.
3. New behavior is covered by tests.
4. Docs (`README.md`, this file, or infra docs) updated if interfaces/operations changed.

## 9. PR Checklist
Use this checklist in pull request descriptions.

- [ ] Scope is focused and excludes unrelated refactors.
- [ ] `just fmt` and `just lint` pass.
- [ ] Relevant tests pass (`just test-go`, plus `just test-infra` when infra changed).
- [ ] API/behavior changes include or update tests.
- [ ] Config/env var changes are documented in `README.md`.
- [ ] Migration/deployment impact is called out (Flyway/CDK) when applicable.
- [ ] Rollback or mitigation notes are included for risky changes.
