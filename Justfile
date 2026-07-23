set unstable := true

[private]
default:
    @just --list --list-submodules

build *ARGS:
    go build {{ ARGS }} -o gh-actionkit .

coverage *ARGS:
    go test ./... -race -cover {{ ARGS }}

fmt *ARGS='.':
    gofmt -w {{ ARGS }}

lint *ARGS:
    golangci-lint run {{ ARGS }}

run *ARGS:
    go run . {{ ARGS }}

test *ARGS:
    go test ./... -race {{ ARGS }}

tidy:
    go mod tidy

vet:
    go vet ./...

check: test lint vet

install: build
    gh extension install .
