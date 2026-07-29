# gh-actionkit

gh-actionkit is a GitHub CLI extension for finding GitHub Actions and keeping workflow files current, valid, and safe.

## Installation

Install the extension from GitHub:

```console
gh extension install joshuadavidthomas/gh-actionkit
```

gh-actionkit uses the GitHub CLI's existing authentication. The `lint` command also requires [zizmor](https://docs.zizmor.sh/installation), either as the official binary or through [`uv`](https://astral.sh/uv).

## Use with prek or pre-commit

Install [prek](https://prek.j178.dev/) (recommended for faster installs and runs) or [pre-commit](https://pre-commit.com/#install). Both use the same `.pre-commit-config.yaml`. Add gh-actionkit without installing the GitHub CLI extension, and pin `rev` to a released tag:

```yaml
repos:
  - repo: https://github.com/joshuadavidthomas/gh-actionkit
    rev: "vX.Y.Z"
    hooks:
      - id: validate
```

Three hooks are available: `validate`, `lint`, and `check`. They run when a file in `.github/workflows` changes. The validate hook is self-contained. The lint hook requires zizmor or uv; add `args: [--offline]` to skip its GitHub API audits. The check hook requires GitHub credentials.

With prek, install the Git hook and prepare its Go environment up front:

```console
prek install --prepare-hooks
prek run validate --all-files
```

The equivalent pre-commit commands are:

```console
pre-commit install --install-hooks
pre-commit run validate --all-files
```

## Agent skill

The [`gh-actionkit` agent skill](skills/gh-actionkit/SKILL.md) teaches coding agents how to find, pin, check, lint, and validate GitHub Actions. It requires gh-actionkit v0.5.0 or newer. Agents check the installed version and upgrade older extensions before using commands introduced in newer releases. Install the skill with any Agent Skills-compatible client.

With [dotagents](https://github.com/getsentry/dotagents), initialize the target project once, then add the skill:

```console
npx @sentry/dotagents init
npx @sentry/dotagents add joshuadavidthomas/gh-actionkit --skill gh-actionkit
```

With [skills](https://skills.sh):

```console
npx skills add joshuadavidthomas/gh-actionkit --skill gh-actionkit
```

As a Claude Code plugin:

```console
claude plugin marketplace add joshuadavidthomas/gh-actionkit
claude plugin install gh-actionkit@gh-actionkit
```

## Commands

| Command | Purpose |
| --- | --- |
| `version OWNER/REPO[/PATH]` | Show the repository's latest stable release, major tag, and commit SHAs |
| `search QUERY` | Find repositories that contain a root Action manifest |
| `inspect OWNER/REPO[/PATH]` | Show repository, manifest, runtime, input, output, and stable version details |
| `check` | Find outdated Action refs in a repository's workflows |
| `lint` | Audit workflows with zizmor |
| `validate` | Validate workflow syntax with the embedded actionlint library |
| `skill` | Print the bundled gh-actionkit agent skill |

### Print the agent skill

```console
gh actionkit skill
gh actionkit skill > SKILL.md
```

`skill` writes the exact bundled `SKILL.md` content to stdout.

### Look up a version

```console
gh actionkit version actions/checkout
gh actionkit version github/codeql-action/init --snippet
gh actionkit version actions/checkout --json
```

An Action identifier has the form `OWNER/REPO[/PATH...]`, where `PATH` names the directory that contains the manifest. `version` resolves releases and tags for the repository and keeps the full identifier in its output; it does not check whether a subpath has a manifest. Use `--snippet` to print a copy-ready `uses:` line pinned to the latest stable release's full commit SHA.

### Search for Actions

```console
gh actionkit search "docker build"
gh actionkit search checkout --limit 5
gh actionkit search checkout --json
```

ActionKit verifies that each result has a root `action.yml` or `action.yaml` file. Search does not discover Actions in subdirectories.

### Inspect an Action

```console
gh actionkit inspect actions/checkout
gh actionkit inspect github/codeql-action/init --json
```

`inspect` shows repository ownership and maintenance facts, then parses `action.yml` or `action.yaml` in the named Action directory at the latest stable release. It prefers `action.yml` and does not fall back to the repository root when a subpath has no manifest. The full manifest path and ref appear in both output formats. When that release resolves to a full commit SHA, the output includes a copy-ready pinned `uses:` line with the requested subpath. Repositories without releases or tags are inspected at `HEAD`.

### Check a repository

```console
gh actionkit check
gh actionkit check --repo ../another-repository --json
gh actionkit check --require-sha --fail-on-unknown
gh actionkit check --allow-owner actions --allow-owner github
```

`check` exits with status 1 when it finds an update. Use `--require-sha` to reject moving refs, `--fail-on-unknown` to reject refs that cannot be classified, and repeat `--allow-owner` to restrict remote Actions to named owners. Unresolved branches and full SHAs that do not match the current stable or major ref are reported as unknown. A SHA alone does not reveal whether its commit is older or newer than a release. With `--json`, an empty result remains `[]` on stdout while stderr states whether the repository has no workflow files or no remote Action uses.

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
