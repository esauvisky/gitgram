VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/esauvisky/gitgram/internal/ops.Version=$(VERSION)
CONFIG  ?= config.yaml

.PHONY: build run vet docker docker-up

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o gitgram ./cmd/gitgram

run:
	go run ./cmd/gitgram serve --config $(CONFIG) --poll

vet:
	go build ./... && go vet ./...

docker:
	docker build --build-arg VERSION=$(VERSION) -t gitgram:$(VERSION) -t gitgram:latest .

docker-up:
	VERSION=$(VERSION) docker compose up --build -d
