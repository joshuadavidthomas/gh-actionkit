# ActionKit

ActionKit is a GitHub CLI extension for finding GitHub Actions and keeping workflow files current, valid, and safe.

## Installation

Install the extension from GitHub:

```console
gh extension install joshuadavidthomas/gh-actionkit
```

ActionKit uses the GitHub CLI's existing authentication. The `lint` command also requires [zizmor](https://docs.zizmor.sh/installation).

## Commands

| Command | Purpose |
| --- | --- |
| `version OWNER/REPO` | Show the latest stable release, major tag, and commit SHAs |
| `search QUERY` | Find repositories that contain a root Action manifest |
| `check` | Find outdated Action refs in a repository's workflows |
| `lint` | Audit workflows with zizmor |
| `validate` | Validate workflow syntax with the embedded actionlint library |

### Look up a version

```console
gh actionkit version actions/checkout
gh actionkit version actions/checkout --json
```

### Search for Actions

```console
gh actionkit search "docker build"
gh actionkit search checkout --limit 5
gh actionkit search checkout --fast --json
```

ActionKit normally verifies that each result has an `action.yml` or `action.yaml` file. `--fast` skips that check.

### Check a repository

```console
gh actionkit check
gh actionkit check --repo ../another-repository --json
```

`check` exits with status 1 when it finds an update. Unresolved branches and other non-version refs are reported as unknown.

### Lint and validate workflows

```console
gh actionkit lint --pedantic
gh actionkit validate
gh actionkit validate --json
```

`lint` preserves zizmor's exit status. `validate` exits with status 1 when actionlint finds a problem.

## Development

ActionKit requires Go 1.25 or newer. Install the development tools declared in `mise.toml`, then build and install the local checkout as a GitHub CLI extension:

```console
mise install
just install
```

Run all checks:

```console
just check
actionlint
```

Other development commands include `just coverage`, `just fmt`, `just run -- --help`, and `just tidy`.

## Releasing

See [`docs/releasing.md`](docs/releasing.md) for the tag-and-publish workflow.

## License

ActionKit is licensed under the [MIT License](LICENSE).
