# CDK Infrastructure

AWS CDK app for provisioning `accountlink-platform-go` infrastructure.

## Scope
This README covers only the CDK project in `infra/cdk`.

## Deployment Diagram

```mermaid
flowchart TB
    %% External
    Internet[(Internet Users)]

    %% Foundation stack
    subgraph Foundation["Foundation Stack"]
        direction TB

        EventTopic["SNS Topic<br/>accountlink-dev-events"]
        DbSecret["Secrets Manager<br/>Aurora credentials"]

        subgraph VPC["VPC 10.0.0.0/16"]
            direction TB

            IGW["Internet Gateway"]

            %% Public
            subgraph Public["Public Subnets"]
                direction LR
                PubA["PublicSubnet1<br/>us-east-1a"]
                PubB["PublicSubnet2<br/>us-east-1b"]

                NAT["NAT Gateway<br/>(in PublicSubnet1)"]
                ALB["Application Load Balancer<br/>internet-facing"]
            end

            %% Private
            subgraph Private["Private Subnets"]
                direction LR
                PrivA["PrivateSubnet1<br/>us-east-1a"]
                PrivB["PrivateSubnet2<br/>us-east-1b"]

                TaskA["Fargate Task<br/>App :8080"]
                TaskB["Fargate Task<br/>App :8080"]
            end

            %% Data
            subgraph Data["Data Layer"]
                direction TB
                DbSubnetGroup["DB Subnet Group<br/>private subnets"]
                Aurora["Aurora PostgreSQL<br/>Serverless v2"]
                Writer["Writer Instance"]
            end

            %% Security groups
            AlbSg["ALB SG<br/>:80 from 0.0.0.0/0"]
            ServiceSg["Service SG<br/>:8080 from ALB SG"]
            DbSg["DB SG<br/>:5432 from Service SG"]
        end
    end

    %% Service stack
    subgraph Service["Service Stack"]
        direction TB

        Cluster["ECS Cluster<br/>accountlink-dev"]
        TaskDef["Task Definition<br/>512 CPU / 1024 MB"]
        TaskRole["Task Role<br/>read secret<br/>publish SNS"]
        ExecRole["Execution Role<br/>ECR + logs + secret"]
        TG["Target Group<br/>HTTP :8080"]
        Listener["Listener<br/>HTTP :80"]
        Logs["CloudWatch Logs"]
        Image["ECR Image"]
    end

    %% Ingress
    Internet -->|"HTTP :80"| ALB
    IGW --> ALB
    ALB --> Listener
    Listener --> TG
    TG --> TaskA
    TG --> TaskB

    %% Security relationships
    AlbSg -.-> ALB
    ServiceSg -.-> TaskA
    ServiceSg -.-> TaskB
    AlbSg -->|"allow :8080"| ServiceSg
    ServiceSg -->|"allow :5432"| DbSg
    DbSg -.-> Aurora

    %% ECS relationships
    Cluster --> TaskDef
    TaskDef --> TaskA
    TaskDef --> TaskB

    %% Runtime dependencies
    ExecRole --> Image
    ExecRole --> Logs
    ExecRole --> DbSecret

    TaskRole --> DbSecret
    TaskRole --> EventTopic

    TaskA -->|"read secret"| DbSecret
    TaskB -->|"read secret"| DbSecret

    TaskA -->|"Postgres :5432"| Aurora
    TaskB -->|"Postgres :5432"| Aurora

    TaskA -->|"publish"| EventTopic
    TaskB -->|"publish"| EventTopic

    %% Data layer
    DbSubnetGroup --> Aurora
    Aurora --> Writer

    %% Egress
    NAT -->|"outbound internet"| TaskA
    NAT -->|"outbound internet"| TaskB
```

## What this CDK app creates
- Foundation stack (`<app>-<env>-foundation`)
  - VPC (new or imported)
  - Aurora PostgreSQL Serverless v2 cluster
  - Security groups for DB access
  - SNS topic for account link events
- Service stack (`<app>-<env>-service`)
  - ECS Fargate service
  - Application Load Balancer (HTTP)
  - ECR asset build/push from the repository `Dockerfile`
  - Runtime wiring to Aurora via generated secret/endpoint values
  - SNS publish permission and topic ARN injection for app event publishing

## Project layout
- `bin/app.ts`: entrypoint and context/env resolution
- `lib/foundation-stack.ts`: network + database
- `lib/service-stack.ts`: ECS/ALB + app deployment
- `lib/config/`: per-environment defaults (`dev`, `test`, `prod`)

## Prerequisites
- AWS credentials for the target account/region
- Node.js + npm
- Docker (required for image asset build during synth/deploy)
- `just` (optional, but preferred for consistent commands)

## Install dependencies
```bash
just infra-install
```

## Run infra tests
```bash
just test-infra
```

## Environment configuration
Defaults are defined in:
- `lib/config/dev.ts`
- `lib/config/test.ts`
- `lib/config/prod.ts`

Required per environment:
- `account`
- `region`

Common configurable values:
- `appName`
- `desiredCount`
- `containerPort`
- `vpcId` (optional import of an existing VPC)

## Context and environment overrides
`bin/app.ts` resolves values in this order:
1. CDK context (`-c key=value`)
2. Environment variables
3. `lib/config/<env>.ts` defaults

Supported keys:
- `envName` / `ENV_NAME`
- `appName` / `APP_NAME`
- `account` / `AWS_ACCOUNT_ID`
- `region` / `AWS_REGION`
- `vpcId` / `VPC_ID`
- `containerPort` / `CONTAINER_PORT`
- `desiredCount` / `DESIRED_COUNT`
- `imageAssetPath` / `IMAGE_ASSET_PATH`
- `imageAssetDockerfile` / `IMAGE_ASSET_DOCKERFILE`

## Bootstrap, synth, diff, deploy
From repo root:
```bash
just cdk-bootstrap dev
just cdk-synth dev
just cdk-deploy dev
```

## Deploy in stages
```bash
just cdk-deploy-foundation dev
just cdk-deploy-service dev
```

Optional app name override:
```bash
just cdk-deploy-foundation env=dev app_name=accountlink
just cdk-deploy-service env=dev app_name=accountlink
```

## Related workflow outside this directory
Database migrations are managed with Flyway in `infra/flyway` and are intentionally documented separately.
