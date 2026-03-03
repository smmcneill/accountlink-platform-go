default:
    @just --list

run:
    @go run ./cmd/server

fmt:
    @go fmt ./...
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
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install aws-cdk
    @HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install node

lint:
    @golangci-lint run

lint-fix:
    @golangci-lint run --fix

test:
    @just fmt
    @go test ./...

infra-install:
    @cd infra/cdk && npm install

flyway-migrate env:
    @if [ ! -f "infra/flyway/{{env}}.conf" ]; then \
        echo "Missing Flyway config: infra/flyway/{{env}}.conf"; \
        exit 1; \
    fi
    @flyway -configFiles=infra/flyway/{{env}}.conf migrate

cdk-bootstrap env:
    @cd infra/cdk && npm install && cdk bootstrap -c envName={{env}}

cdk-synth env:
    @cd infra/cdk && npm install && cdk synth -c envName={{env}}

cdk-deploy env:
    @cd infra/cdk && npm install && cdk deploy --require-approval never -c envName={{env}}

release env:
    @just flyway-migrate {{env}}
    @just cdk-deploy {{env}}

db-up:
    @docker compose up -d postgres

docker-clean:
    @docker rm -f $(docker ps -qa)    
    @docker rmi -f $(docker images -qa)
