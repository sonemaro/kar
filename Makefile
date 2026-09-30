.PHONY: build test vet fmt

build:
	go build -o bin/kar .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .
