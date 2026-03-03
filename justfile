default:
    @just --list

run:
    @go run ./cmd/server

test:
    @go test ./...

db-up:
    @docker compose up -d postgres

docker-clean:
    @docker rm -f $(docker ps -qa)    
    @docker rmi -f $(docker images -qa)
