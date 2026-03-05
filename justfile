INFO_COLOR := "\u{1b}[1;36m"
WARN_COLOR := "\u{1b}[1;33m"
ERROR_COLOR := "\u{1b}[1;31m"
RESET_COLOR := "\u{1b}[0m"

# Show available recipes.
default:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Listing available recipes" "{{RESET_COLOR}}"
    @just --list

# Run the API server locally.
run:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Starting API server" "{{RESET_COLOR}}"
    @go run ./cmd/server

# Format Go source files.
fmt:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Formatting Go source files" "{{RESET_COLOR}}"
    @go fmt ./cmd/... ./internal/...

# Synchronize module dependencies.
tidy:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Tidying Go module dependencies" "{{RESET_COLOR}}"
    @go mod tidy

# Install local tooling dependencies via Homebrew.
setup:
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Validating Homebrew installation" "{{RESET_COLOR}}"
    @if ! command -v brew >/dev/null 2>&1; then \
        printf "%b%s%b\n" "{{ERROR_COLOR}}" "Homebrew not found. Install Homebrew, then run: just setup" "{{RESET_COLOR}}"; \
        exit 1; \
    fi
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Updating Homebrew metadata" "{{RESET_COLOR}}"
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew update
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Upgrading installed Homebrew packages" "{{RESET_COLOR}}"
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew upgrade
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Installing golangci-lint, flyway, and node" "{{RESET_COLOR}}"
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install golangci-lint
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install flyway
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install node
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Setup complete" "{{RESET_COLOR}}"

# Run Go linters.
lint:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running Go linters" "{{RESET_COLOR}}"
    @golangci-lint run ./cmd/... ./internal/...

# Run Go linters and apply auto-fixes when possible.
lint-fix:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running Go linters with auto-fix" "{{RESET_COLOR}}"
    @golangci-lint run --fix ./cmd/... ./internal/...

# Run application tests (Go).
test-go:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running Go tests" "{{RESET_COLOR}}"
    @just fmt
    @go test ./cmd/... ./internal/...

# Run infrastructure tests (CDK TypeScript).
test-infra:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running CDK infrastructure tests" "{{RESET_COLOR}}"
    @cd infra/cdk && npm test

# Run all tests (application + infrastructure).
test:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running full test suite (application + infrastructure)" "{{RESET_COLOR}}"
    @just test-go
    @just test-infra
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Full test suite complete" "{{RESET_COLOR}}"

# Install CDK app dependencies.
infra-install:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Installing CDK dependencies" "{{RESET_COLOR}}"
    @cd infra/cdk && npm install

# Build Linux binary for container/runtime usage.
build-linux-binary platform="linux/amd64":
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Building Linux server binary (platform={{platform}})" "{{RESET_COLOR}}"
    @mkdir -p build
    @case "{{platform}}" in \
        linux/amd64) GOARCH=amd64 ;; \
        linux/arm64) GOARCH=arm64 ;; \
        *) printf "%b%s%b\n" "{{ERROR_COLOR}}" "Unsupported platform '{{platform}}'. Supported: linux/amd64, linux/arm64" "{{RESET_COLOR}}"; exit 1 ;; \
    esac; \
    CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH go build -trimpath -ldflags="-s -w" -o build/server ./cmd/server
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Binary available at build/server" "{{RESET_COLOR}}"

# Run Flyway migrations for a target environment (expects infra/flyway/<env>.conf).
flyway-migrate env:
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Running Flyway migration for env={{env}}" "{{RESET_COLOR}}"
    @if [ ! -f "infra/flyway/{{env}}.conf" ]; then \
        printf "%b%s%b\n" "{{ERROR_COLOR}}" "Missing Flyway config: infra/flyway/{{env}}.conf" "{{RESET_COLOR}}"; \
        exit 1; \
    fi
    @flyway -configFiles=infra/flyway/{{env}}.conf migrate
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Flyway migration complete for env={{env}}" "{{RESET_COLOR}}"

# Bootstrap CDK resources in target account/region.
cdk-bootstrap env:
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Bootstrapping CDK for env={{env}}" "{{RESET_COLOR}}"
    @cd infra/cdk && npm install && npx cdk bootstrap -c envName={{env}}
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> CDK bootstrap complete for env={{env}}" "{{RESET_COLOR}}"

# Synthesize CDK templates for an environment.
cdk-synth env:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Synthesizing CDK templates for env={{env}}" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk synth -c envName={{env}}
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> CDK synth complete for env={{env}}" "{{RESET_COLOR}}"

# Deploy all CDK stacks for an environment.
cdk-deploy env:
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Deploying all CDK stacks for env={{env}}" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --all --require-approval never -c envName={{env}}
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> CDK deploy complete for env={{env}}" "{{RESET_COLOR}}"

# Deploy only the foundation stack (network/database).
cdk-deploy-foundation env app_name="accountlink":
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Deploying foundation stack for app={{app_name}}, env={{env}}" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --require-approval never -c envName={{env}} -c appName={{app_name}} {{app_name}}-{{env}}-foundation
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Foundation stack deploy complete" "{{RESET_COLOR}}"

# Deploy only the service stack (ecs/alb/app).
cdk-deploy-service env app_name="accountlink":
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Deploying service stack for app={{app_name}}, env={{env}}" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --require-approval never -c envName={{env}} -c appName={{app_name}} {{app_name}}-{{env}}-service
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Service stack deploy complete" "{{RESET_COLOR}}"

# Release flow: foundation deploy, migration, then service deploy.
release env app_name="accountlink":
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Starting release flow for app={{app_name}}, env={{env}}" "{{RESET_COLOR}}"
    @just cdk-deploy-foundation env={{env}} app_name={{app_name}}
    @just flyway-migrate {{env}}
    @just cdk-deploy-service env={{env}} app_name={{app_name}}
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Release flow complete for env={{env}}" "{{RESET_COLOR}}"

# Start local Postgres dependency.
db-up:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Starting local Postgres via docker compose" "{{RESET_COLOR}}"
    @docker compose up -d postgres
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Local Postgres is up" "{{RESET_COLOR}}"

# Remove all local Docker containers/images.
docker-clean:
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Removing all local Docker containers and images" "{{RESET_COLOR}}"
    @docker rm -f $(docker ps -qa)    
    @docker rmi -f $(docker images -qa)
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Docker cleanup complete" "{{RESET_COLOR}}"

# Build distroless runtime image from prebuilt binary.
image-build tag="accountlink-platform-go:latest" platform="linux/amd64":
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Building container image {{tag}} for platform={{platform}}" "{{RESET_COLOR}}"
    @just build-linux-binary platform={{platform}}
    @docker build --platform {{platform}} -t {{tag}} .
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Container image build complete: {{tag}}" "{{RESET_COLOR}}"
