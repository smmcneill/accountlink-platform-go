# Infrastructure

This directory contains infrastructure definitions and migration config for deploying `accountlink-platform-go`.

## Layout
- `cdk/`: AWS CDK app (TypeScript)
  - `lib/foundation-stack.ts`: VPC + Aurora PostgreSQL
  - `lib/service-stack.ts`: ECS Fargate + ALB + app wiring
  - `lib/config/`: environment config (`dev`, `test`, `prod`)
- `flyway/`: Flyway config files per environment (`dev.conf`, `test.conf`, `prod.conf`)

## Prerequisites
- AWS credentials configured for target account/region
- Node.js + npm
- CDK dependencies installed (`just infra-install` or `cd infra/cdk && npm install`)
- Flyway installed (`just setup` installs it via Homebrew)

## Deploy Flow
Use this order:
1. Deploy foundation resources:
   - `just cdk-deploy-foundation dev`
2. Run DB migrations:
   - `just flyway-migrate dev`
3. Deploy service resources:
   - `just cdk-deploy-service dev`

Or run all three:
- `just release dev`

## Config
- CDK environment/account settings:
  - `infra/cdk/lib/config/dev.ts`
  - `infra/cdk/lib/config/test.ts`
  - `infra/cdk/lib/config/prod.ts`
- Flyway per-environment config:
  - `infra/flyway/dev.conf`
  - `infra/flyway/test.conf`
  - `infra/flyway/prod.conf`

## Notes
- CDK uses Docker `fromAsset` to build/publish the app image.
- `just cdk-synth` and `just cdk-deploy*` prebuild `build/server` before synth/deploy.
- Aurora endpoint is private by default. Flyway must run from a network path that can resolve/reach the private endpoint (same VPC, bastion/SSM tunnel, ECS task, or VPC-enabled CI runner).
