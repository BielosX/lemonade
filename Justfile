os := if os() == "macos" { "osx" } else { "linux" }
arch := if arch() == "aarch64" { "aarch_64" } else { "x86_64" }
version := "31.0"
protoc := env("PROTOC", "bin/protoc")
tools-dir := justfile_directory() + "/bin"
golangci-lint-version := "v2.3.0"
golangci-lint-install-url := "https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh"

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
    {{ protoc }} --proto_path=protobuf \
        --go_opt=default_api_level=API_OPAQUE \
        --go_out={{ justfile_directory() }} \
        protobuf/*.proto

go-tools:
    mkdir -p "{{tools-dir}}"
    GOBIN="{{tools-dir}}" go install golang.org/x/tools/cmd/goimports@latest
    GOBIN="{{tools-dir}}" go install github.com/segmentio/golines@latest
    curl -sSfL "{{golangci-lint-install-url}}" | sh -s -- -b "{{tools-dir}}" "{{golangci-lint-version}}"

go-qual: go-tools
    {{tools-dir}}/goimports -w .
    {{tools-dir}}/golines -w .
    {{tools-dir}}/golangci-lint run cmd internal