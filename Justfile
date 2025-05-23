run:
    go run cmd/main.go

lint:
    golangci-lint run cmd internal

fmt:
    gofmt -w cmd
    gofmt -w internal
