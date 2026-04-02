GOLANGCI_LINT_VERSION ?= v2.6.1
GIT_COMMIT ?= $(shell git rev-list -1 HEAD 2>/dev/null || echo "unknown")
VERSION ?= snapshot
IMG ?= ch-migrator-example:latest

GOBIN ?= $(shell go env GOPATH)/bin

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	$(GOBIN)/golangci-lint run

.PHONY: build
build: fmt vet
	CGO_ENABLED=0 go build -ldflags "-X github.com/ClickHouse/ch-migrator-example/pkg/version.version=$(VERSION) -X github.com/ClickHouse/ch-migrator-example/pkg/version.gitCommit=$(GIT_COMMIT)" -o bin/ch-migrator-example main.go

.PHONY: test
test:
	go test ./pkg/... -v -count=1

.PHONY: integration-test
integration-test:
	go test ./integration-test/... -v -timeout 5m -count=1

.PHONY: new-migration
new-migration:
ifndef NAME
	$(error NAME is required. Usage: make new-migration NAME=add_user_events)
endif
	cd pkg/migrations && goose create $(NAME) sql

.PHONY: new-go-migration
new-go-migration:
ifndef NAME
	$(error NAME is required. Usage: make new-go-migration NAME=set_ttl)
endif
	cd pkg/migrations && goose create $(NAME) go

.PHONY: docker-build
docker-build:
	docker build --build-arg VERSION=$(VERSION) --build-arg GIT_COMMIT=$(GIT_COMMIT) -t $(IMG) .
