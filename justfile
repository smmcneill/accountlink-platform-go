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
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Installing go, golangci-lint, flyway, and node" "{{RESET_COLOR}}"
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install go
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install golangci-lint
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install flyway
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install node
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Installing Go mutation testing tool" "{{RESET_COLOR}}"
    @go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Setup complete" "{{RESET_COLOR}}"

# Run Go linters.
lint:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running Go linters" "{{RESET_COLOR}}"
    @golangci-lint run ./cmd/... ./internal/...

# Run Go linters and apply auto-fixes when possible.
lint-fix:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running Go linters with auto-fix" "{{RESET_COLOR}}"
    @golangci-lint run --fix ./cmd/... ./internal/...

# Ensure required shell variables are available before running environment-sensitive recipes.
_require-env:
    @if [ -z "${ENV:-}" ]; then \
        printf "%b%s%b\n" "{{ERROR_COLOR}}" "In order to run this recipe, you must source an environment file, for example '. local.env'." "{{RESET_COLOR}}"; \
        exit 1; \
    fi

# Run isolated BDD tests (godog) under test/.
# Caller must source env before running:
#   . local.env && just test-bdd
# Required environment variable:
#   BDD_BASE_URL
test-bdd: _require-env
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running BDD tests" "{{RESET_COLOR}}"
    BDD_BASE_URL="$BDD_BASE_URL" cd test && go test -count=1 -v ./...

# Run application tests (Go).
test-go:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running Go tests" "{{RESET_COLOR}}"
    @just fmt
    @go test ./cmd/... ./internal/... 
    @just mutation

# Run mutation tests (requires installing go-mutesting).
mutation:
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Running mutation tests for critical packages" "{{RESET_COLOR}}"
    @go-mutesting ./internal/app ./internal/domain/accountlink.go

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

# Run Flyway migrations for a target environment (expects infra/flyway/<ENV>.conf).
flyway-migrate: _require-env
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Running Flyway migration for env=$ENV" "{{RESET_COLOR}}"
    @if [ ! -f "infra/flyway/$ENV.conf" ]; then \
        printf "%b%s%b\n" "{{ERROR_COLOR}}" "Missing Flyway config: infra/flyway/$ENV.conf" "{{RESET_COLOR}}"; \
        exit 1; \
    fi
    @flyway -configFiles=infra/flyway/$ENV.conf migrate
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Flyway migration complete for env=$ENV" "{{RESET_COLOR}}"

# Bootstrap CDK resources in target account/region.
cdk-bootstrap: _require-env
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Bootstrapping CDK for env=$ENV" "{{RESET_COLOR}}"
    @cd infra/cdk && npm install && npx cdk bootstrap -c envName=$ENV
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> CDK bootstrap complete for env=$ENV" "{{RESET_COLOR}}"

# Synthesize CDK templates for an environment.
cdk-synth: _require-env
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Synthesizing CDK templates for env=$ENV" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk synth -c envName=$ENV
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> CDK synth complete for env=$ENV" "{{RESET_COLOR}}"

# Deploy all CDK stacks for an environment.
cdk-deploy: _require-env
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Deploying all CDK stacks for env=$ENV" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --all --require-approval never -c envName=$ENV
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> CDK deploy complete for env=$ENV" "{{RESET_COLOR}}"

# Destroy all CDK stacks for an environment.
cdk-undeploy: _require-env
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Destroying all CDK stacks for env=$ENV" "{{RESET_COLOR}}"
    @cd infra/cdk && npm install && npx cdk destroy --all --force -c envName=$ENV
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> CDK destroy complete for env=$ENV" "{{RESET_COLOR}}"

# Deploy only the foundation stack (network/database).
cdk-deploy-foundation app_name="accountlink": _require-env
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Deploying foundation stack for app={{app_name}}, env=$ENV" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --require-approval never -c envName=$ENV -c appName={{app_name}} {{app_name}}-$ENV-foundation
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Foundation stack deploy complete" "{{RESET_COLOR}}"

# Deploy only the service stack (ecs/alb/app).
cdk-deploy-service app_name="accountlink": _require-env
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Deploying service stack for app={{app_name}}, env=$ENV" "{{RESET_COLOR}}"
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --require-approval never -c envName=$ENV -c appName={{app_name}} {{app_name}}-$ENV-service
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Service stack deploy complete" "{{RESET_COLOR}}"

# Release flow: foundation deploy, migration, then service deploy.
release app_name="accountlink": _require-env
    @printf "%b%s%b\n" "{{WARN_COLOR}}" "==> Starting release flow for app={{app_name}}, env=$ENV" "{{RESET_COLOR}}"
    @just cdk-deploy-foundation app_name={{app_name}}
    @just flyway-migrate
    @just cdk-deploy-service app_name={{app_name}}
    @printf "%b%s%b\n" "{{INFO_COLOR}}" "==> Release flow complete for env=$ENV" "{{RESET_COLOR}}"

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
