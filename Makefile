.PHONY: build test test-race lint run-api run-outbox run-fanout run-worker docker-up docker-down sqlc k6

# Build unified binary
build:
	go build -ldflags="-w -s" -o bin/relayforge ./cmd/relayforge

# Run all unit and integration tests
test:
	go test -v ./...

# Run tests with race detection
test-race:
	go test -v -race ./...

# Generate SQL queries with sqlc
sqlc:
	sqlc generate

# Run local daemons
run-api:
	go run ./cmd/relayforge api

run-outbox:
	go run ./cmd/relayforge outbox

run-fanout:
	go run ./cmd/relayforge fanout

run-worker:
	go run ./cmd/relayforge worker

# Docker Compose orchestration
docker-up:
	docker compose -f deployments/docker-compose.yml up -d --build

docker-down:
	docker compose -f deployments/docker-compose.yml down -v

# Run k6 load test
k6:
	k6 run tests/load/ingestion_load.js

