GO ?= $(shell command -v go 2>/dev/null)
GOFMT ?= $(shell command -v gofmt 2>/dev/null)

.PHONY: run test testv fmt vet tidy build clean check-go check-gofmt

check-go:
	@test -n "$(GO)" || (echo "go not found in PATH. Install Go (>= 1.22) or run: make GO=/path/to/go <target>"; exit 1)

check-gofmt:
	@test -n "$(GOFMT)" || (echo "gofmt not found in PATH. Install Go (>= 1.22) or run: make GOFMT=/path/to/gofmt fmt"; exit 1)

run: check-go
	$(GO) run ./cmd/server

test: check-go
	$(GO) test ./...

testv: check-go
	$(GO) test -v ./...

fmt: check-gofmt
	$(GOFMT) -w $$(find . -name '*.go')

vet: check-go
	$(GO) vet ./...

tidy: check-go
	$(GO) mod tidy

build: check-go
	mkdir -p bin
	$(GO) build -o bin/server ./cmd/server

clean:
	rm -rf bin
