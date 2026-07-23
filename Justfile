set unstable := true

[private]
default:
    @just --list

fmt:
    gofmt -w .

test:
    go test -race ./...

vet:
    go vet ./...

check: test vet

build:
    go build -o dist/gh-actionkit .

install:
    go build -o gh-actionkit .
    gh extension install .
