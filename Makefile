.PHONY: help build clean fmt test coverage coverage-html dep vet lint

.DEFAULT_GOAL := help

## help: print this help message
help:
	@echo 'Usage: make <target>'
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'


## build: build the gatonaranja binary
build:
	go build -o gatonaranja


## clean: remove Go build artifacts
clean:
	go clean


## fmt: format the code using golangci-lint
fmt:
	golangci-lint fmt


## test: run the tests
test:
	go test ./...


## coverage: run the tests and generate a coverage report
coverage:
	go test ./... -coverprofile=coverage.out


## coverage-html: generate an HTML coverage report
coverage-html: coverage
	go tool cover -html=coverage.out


## dep: download Go module dependencies
dep:
	go mod download


## vet: run go vet
vet:
	go vet ./...


## lint: lint the code using golangci-lint
lint:
	golangci-lint run
