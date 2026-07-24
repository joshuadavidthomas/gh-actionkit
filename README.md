# gh-actionkit

gh-actionkit is a GitHub CLI extension for finding GitHub Actions and keeping workflow files current, valid, and safe.

## Installation

Install the extension from GitHub:

```console
gh extension install joshuadavidthomas/gh-actionkit
```

gh-actionkit uses the GitHub CLI's existing authentication. The `lint` command also requires [zizmor](https://docs.zizmor.sh/installation), either as the official binary or through [`uv`](https://astral.sh/uv).

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
gh actionkit search checkout --json
```

ActionKit verifies that each result has an `action.yml` or `action.yaml` file.

### Check a repository

```console
gh actionkit check
gh actionkit check --repo ../another-repository --json
```

`check` exits with status 1 when it finds an update. Unresolved branches and other non-version refs are reported as unknown.

### Lint and validate workflows

```console
gh actionkit lint --pedantic
gh actionkit lint --offline
gh actionkit validate
gh actionkit validate --json
```

`lint` uses the active GitHub CLI token for zizmor's online audits. Pass `--offline` to disable repository fetches and online audits. If `gh` has credentials for more than one host, select one with `GH_HOST`, such as `GH_HOST=github.example.com gh actionkit lint`. `lint` preserves zizmor's exit status. `validate` exits with status 1 when actionlint finds a problem.

## Development

gh-actionkit requires Go 1.25 or newer. Install the development tools declared in `mise.toml`, then build and install the local checkout as a GitHub CLI extension:

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

gh-actionkit is licensed under the MIT license. See the [`LICENSE`](LICENSE) file for more information.
