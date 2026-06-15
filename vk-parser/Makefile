.PHONY: run build tidy fmt vet

run:
	go run ./cmd/app

build:
	go build -o bin/app ./cmd/app

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...
