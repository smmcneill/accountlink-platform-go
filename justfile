# Show available recipes.
default:
    @just --list

# Run the API server locally.
run:
    @go run ./cmd/server

# Format Go source files.
fmt:
    @go fmt ./cmd/... ./internal/...

# Synchronize module dependencies.
tidy:
    @go mod tidy

# Install local tooling dependencies via Homebrew.
setup:
    @if ! command -v brew >/dev/null 2>&1; then \
        echo "Homebrew not found. Install Homebrew, then run: just setup"; \
        exit 1; \
    fi
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew update
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew upgrade
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install golangci-lint
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install flyway
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install node

# Run Go linters.
lint:
    @golangci-lint run ./cmd/... ./internal/...

# Run Go linters and apply auto-fixes when possible.
lint-fix:
    @golangci-lint run --fix ./cmd/... ./internal/...

# Format and test all Go packages.
test:
    @just fmt
    @go test ./cmd/... ./internal/...

# Install CDK app dependencies.
infra-install:
    @cd infra/cdk && npm install

# Build Linux binary for container/runtime usage.
build-linux-binary platform="linux/amd64":
    @mkdir -p build
    @case "{{platform}}" in \
        linux/amd64) GOARCH=amd64 ;; \
        linux/arm64) GOARCH=arm64 ;; \
        *) echo "Unsupported platform '{{platform}}'. Supported: linux/amd64, linux/arm64"; exit 1 ;; \
    esac; \
    CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH go build -trimpath -ldflags="-s -w" -o build/server ./cmd/server

# Run Flyway migrations for a target environment (expects infra/flyway/<env>.conf).
flyway-migrate env:
    @if [ ! -f "infra/flyway/{{env}}.conf" ]; then \
        echo "Missing Flyway config: infra/flyway/{{env}}.conf"; \
        exit 1; \
    fi
    @flyway -configFiles=infra/flyway/{{env}}.conf migrate

# Bootstrap CDK resources in target account/region.
cdk-bootstrap env:
    @cd infra/cdk && npm install && npx cdk bootstrap -c envName={{env}}

# Synthesize CDK templates for an environment.
cdk-synth env:
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk synth -c envName={{env}}

# Deploy all CDK stacks for an environment.
cdk-deploy env:
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --all --require-approval never -c envName={{env}}

# Deploy only the foundation stack (network/database).
cdk-deploy-foundation env app_name="accountlink":
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --require-approval never -c envName={{env}} -c appName={{app_name}} {{app_name}}-{{env}}-foundation

# Deploy only the service stack (ecs/alb/app).
cdk-deploy-service env app_name="accountlink":
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --require-approval never -c envName={{env}} -c appName={{app_name}} {{app_name}}-{{env}}-service

# Release flow: foundation deploy, migration, then service deploy.
release env app_name="accountlink":
    @just cdk-deploy-foundation env={{env}} app_name={{app_name}}
    @just flyway-migrate {{env}}
    @just cdk-deploy-service env={{env}} app_name={{app_name}}

# Start local Postgres dependency.
db-up:
    @docker compose up -d postgres

# Remove all local Docker containers/images.
docker-clean:
    @docker rm -f $(docker ps -qa)    
    @docker rmi -f $(docker images -qa)

# Build distroless runtime image from prebuilt binary.
image-build tag="accountlink-platform-go:latest" platform="linux/amd64":
    @just build-linux-binary platform={{platform}}
    @docker build --platform {{platform}} -t {{tag}} .
