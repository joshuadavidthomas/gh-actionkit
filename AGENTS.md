## Commands

- `just test`: Run tests with the race detector
- `just coverage`: Run tests with race detection and coverage
- `just lint`: Run golangci-lint
- `just fmt`: Format Go code
- `just build`: Build the extension binary
- `just run -- ARGS`: Run the CLI from source
- `just tidy`: Tidy go.mod and go.sum
- `just install`: Build and install the local GitHub CLI extension

## Validation

Run these after implementing changes:

- Tests: `just test`
- Lint: `just lint`
- Format: `just fmt`
- Workflow syntax: `actionlint`

## Architecture

- CLI entry point: `go run . [COMMAND] [ARGS]`
- Module: `github.com/joshuadavidthomas/gh-actionkit`
- `internal/cli/` owns Cobra commands, output, flags, and process status mapping
- `internal/actions/` owns action search and version policy
- `internal/githubapi/` adapts `go-gh` to typed ActionKit interfaces
- `internal/workflow/` parses workflow structure and source locations
- `internal/tools/` wraps zizmor and the embedded actionlint library

## Codebase patterns

- Keep GitHub API payloads typed; do not use `map[string]any`.
- Thread `context.Context` through API and child-process calls.
- Keep JSON output on stdout and diagnostics on stderr.
- Treat API, authentication, and rate-limit failures as command failures.
- Preserve child exit statuses when a wrapped tool has already produced output.
- Parse workflow YAML structurally and retain node line numbers.
- Resolve annotated Git tags to commit SHAs before comparing or displaying them.
- Use `nil` for unknown JSON values; human output may render them as `unknown`.
