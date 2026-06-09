APP_NAME=signalbox

.PHONY: run test vet fmt fmt-check build docker-up docker-down

run:
	go run ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	test -z "$$(gofmt -l .)"

build:
	go build -trimpath -o bin/$(APP_NAME) ./cmd/api

docker-up:
	docker compose --env-file .env up --build

docker-down:
	docker compose down
