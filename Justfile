os := if os() == "macos" { "osx" } else { "linux" }
arch := if arch() == "aarch64" { "aarch_64" } else { "x86_64" }
version := "31.0"
protoc := env("PROTOC", "bin/protoc")

get-protoc:
    #!/bin/bash
    if ! [[ -f {{ protoc }} && $({{ protoc }} --version 2>/dev/null) == "libprotoc {{ version }}" ]]; then
       mkdir -p bin
       curl -o protoc.zip -LSs "https://github.com/protocolbuffers/protobuf/releases/download/v{{ version }}/protoc-{{ version }}-{{ os }}-{{ arch }}.zip"
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

generate-protobuf: get-protoc
    {{ protoc }} --proto_path=protobuf --go_out={{ justfile_directory() }} protobuf/*.proto
