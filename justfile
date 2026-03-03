default:
    @just --list

run:
    @go run ./cmd/server

fmt:
    @go fmt ./cmd/... ./internal/...

tidy:
    @go mod tidy

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

lint:
    @golangci-lint run ./cmd/... ./internal/...

lint-fix:
    @golangci-lint run --fix ./cmd/... ./internal/...

test:
    @just fmt
    @go test ./cmd/... ./internal/...

infra-install:
    @cd infra/cdk && npm install

build-linux-binary platform="linux/amd64":
    @mkdir -p build
    @case "{{platform}}" in \
        linux/amd64) GOARCH=amd64 ;; \
        linux/arm64) GOARCH=arm64 ;; \
        *) echo "Unsupported platform '{{platform}}'. Supported: linux/amd64, linux/arm64"; exit 1 ;; \
    esac; \
    CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH go build -trimpath -ldflags="-s -w" -o build/server ./cmd/server

flyway-migrate env:
    @if [ ! -f "infra/flyway/{{env}}.conf" ]; then \
        echo "Missing Flyway config: infra/flyway/{{env}}.conf"; \
        exit 1; \
    fi
    @flyway -configFiles=infra/flyway/{{env}}.conf migrate

cdk-bootstrap env:
    @cd infra/cdk && npm install && npx cdk bootstrap -c envName={{env}}

cdk-synth env:
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk synth -c envName={{env}}

cdk-deploy env:
    @just build-linux-binary
    @cd infra/cdk && npm install && npx cdk deploy --require-approval never -c envName={{env}}

release env:
    @just flyway-migrate {{env}}
    @just cdk-deploy {{env}}

db-up:
    @docker compose up -d postgres

docker-clean:
    @docker rm -f $(docker ps -qa)    
    @docker rmi -f $(docker images -qa)

image-build tag="accountlink-platform-go:latest" platform="linux/amd64":
    @just build-linux-binary platform={{platform}}
    @docker build --platform {{platform}} -t {{tag}} .
