default: build

build:
    go build ./...

run:
    go run ./cmd/server

test:
    go test ./...

test-pkg pkg:
    go test ./{{pkg}}/...

lint:
    golangci-lint run

fmt:
    gofmt -w .

tidy:
    go mod tidy
