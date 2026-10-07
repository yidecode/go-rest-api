.PHONY: build test vet run docker compose-up compose-down

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/api

docker:
	docker build -t go-rest-api .

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
