.PHONY: run test testv fmt vet tidy build clean

run:
	go run ./cmd/server

test:
	go test ./...

testv:
	go test -v ./...

fmt:
	gofmt -w $$(find . -name '*.go')

vet:
	go vet ./...

tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o bin/server ./cmd/server

clean:
	rm -rf bin
