os := if os() == "macos" { "osx" } else { "linux" }
arch := if arch() == "aarch64" { "aarch_64" } else { "x86_64" }
version := "31.0"

get-protoc:
    #!/bin/bash
    mkdir -p bin
    if ! [[ -f bin/protoc && $(bin/protoc --version 2>/dev/null) == "libprotoc {{ version }}" ]]; then
       curl -o protoc.zip -OL "https://github.com/protocolbuffers/protobuf/releases/download/v{{ version }}/protoc-{{ version }}-{{ os }}-{{ arch }}.zip"
       unzip -o -j protoc.zip bin/protoc -d bin
       rm -rf protoc.zip
    fi

run:
    go run cmd/main.go

lint:
    golangci-lint run cmd internal

fmt:
    gofmt -w cmd
    gofmt -w internal
