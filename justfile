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
    @brew update
    @brew upgrade
    @brew install golangci-lint

lint:
    @golangci-lint run

lint-fix:
    @golangci-lint run --fix

test:
    @just fmt
    @go test ./...

db-up:
    @docker compose up -d postgres

docker-clean:
    @docker rm -f $(docker ps -qa)    
    @docker rmi -f $(docker images -qa)
