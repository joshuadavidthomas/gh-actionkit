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

fmt-check:
    @test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }

lint *ARGS:
    golangci-lint run {{ ARGS }}

run *ARGS:
    go run . {{ ARGS }}

test *ARGS:
    go test ./... -race {{ ARGS }}

tidy:
    go mod tidy

tidy-check:
    go mod tidy -diff

vet:
    go vet ./...

check: test lint vet fmt-check tidy-check

install: build
    gh extension install .
