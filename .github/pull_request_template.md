## Summary
- What changed:
- Why this change:

## Testing
- Commands run:
  - `just fmt`
  - `just lint`
  - `just test-go`
- Additional test notes:

## Deployment / Ops Notes
- Migration impact (Flyway):
- Infrastructure impact (CDK):
- Rollback or mitigation plan:

## Checklist
- [ ] Scope is focused and excludes unrelated refactors.
- [ ] `just fmt` and `just lint` pass.
- [ ] Relevant tests pass (`just test-go`, plus `just test-infra` when infra changed).
- [ ] API/behavior changes include or update tests.
- [ ] Config/env var changes are documented in `README.md`.
- [ ] Migration/deployment impact is called out (Flyway/CDK) when applicable.
- [ ] Rollback or mitigation notes are included for risky changes.
