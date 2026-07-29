---
name: gh-actionkit
description: Use when finding GitHub Actions, choosing or pinning action versions, checking workflow action refs for updates, validating GitHub Actions workflow syntax, or auditing workflows with actionlint or zizmor. Handles gh-actionkit, `gh actionkit`, workflow YAML, full commit SHA pins, stale actions, unpinned actions, and GitHub Actions security checks.
license: MIT
compatibility: Requires the GitHub CLI (`gh`) and gh-actionkit v0.5.0 or newer. GitHub-backed commands need `gh` authentication; linting needs either zizmor or uv.
metadata:
  author: joshuadavidthomas
  version: "0.5.0"
---

# gh-actionkit

gh-actionkit is a GitHub CLI extension for finding Actions and checking workflow files. Run it as `gh actionkit`.

## Set up

Require gh-actionkit v0.5.0 or newer. Run only the needed setup commands:

```console
gh actionkit --version
gh extension install joshuadavidthomas/gh-actionkit  # if missing
gh extension upgrade actionkit                       # if older than v0.5.0
gh auth status                                       # before GitHub-backed commands
```

Stop if the installed version remains too old. `version`, `search`, `inspect`, `check`, and online `lint` use the active GitHub CLI account. Set `GH_HOST` when the user needs a non-default authenticated host.

`lint` uses an installed `zizmor` binary. If zizmor is absent but `uv` is present, gh-actionkit runs its pinned zizmor package through uv. If both are absent, install zizmor from <https://docs.zizmor.sh/installation>.

## Choose the command

| Task | Command |
| --- | --- |
| Find an Action by purpose | `gh actionkit search "QUERY" --json` |
| Inspect an Action's repository, manifest, and stable pin | `gh actionkit inspect OWNER/REPO[/PATH] --json` |
| Resolve an Action's stable tags and full SHAs | `gh actionkit version OWNER/REPO[/PATH] --json` |
| Print a full-SHA-pinned `uses:` line | `gh actionkit version OWNER/REPO[/PATH] --snippet` |
| Find stale or unknown Action refs | `gh actionkit check -C PATH --json` |
| Enforce Action ref and owner policies | `gh actionkit check -C PATH --require-sha --fail-on-unknown --allow-owner actions --allow-owner github --json` |
| Validate workflow syntax and expressions | `gh actionkit validate -C PATH --json` |
| Audit workflow security | `gh actionkit lint -C PATH --json` |

Use `--json` when another tool or the agent will read the result. Human output may shorten SHAs; JSON keeps the full value.

## Find and pin an Action

1. Run `search` when the user has a capability in mind but no repository. Search only proves that a repository has a root `action.yml` or `action.yaml`; it does not discover subdirectory Actions.
2. Run `inspect OWNER/REPO[/PATH] --json` for repository ownership, archived state, last push, license, and the latest stable release's manifest inputs, outputs, runtime, and pinned reference. `PATH` names the directory containing `action.yml` or `action.yaml`. Inspect does not fall back to the root manifest when that directory has none. Inspect permissions and source code separately before adding it.
3. Run `version OWNER/REPO[/PATH] --json` when only version data is needed, or add `--snippet` for a copy-ready `uses:` line. Version data has repository scope, but output and snippets keep the full Action identifier; `version` does not verify a subpath manifest.
4. With JSON output, confirm that `latest.sha` is a non-null, 40-character hexadecimal commit SHA. Stop without editing if it is missing or malformed.
5. Use that SHA for an exact pin and put `latest.tag` in a comment. Snippet output does this for you:

```yaml
- uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
```

Use `major.sha` only when the task calls for the commit currently named by the stable major tag. Never replace a full SHA with a moving tag when the user asks for secure pins.

After editing, run `validate`, `lint`, and `check` against the repository. Inspect each reported location before changing it; the same Action may be pinned to different release lines on purpose.

## Audit a repository

Run each check separately so a finding status from one does not skip the others:

```console
gh actionkit validate -C /path/to/repository --json
gh actionkit lint -C /path/to/repository --json
gh actionkit check -C /path/to/repository --json
```

Add `--pedantic` to `lint` for stricter zizmor audits. Online audits are the default and use the active GitHub CLI token. Use `--offline` only when the user asks to disable zizmor's GitHub access or accepts reduced coverage; do not silently turn off online checks after an authentication error. If gh-actionkit falls back to uv and its pinned zizmor package is not cached, uv may still contact PyPI.

## Read output and status together

- `check` exits 1 when an update or requested policy violation is present. Each JSON result has a `status` of `up_to_date`, `update_available`, or `unknown`. Its JSON output is still the result, not a command failure. When it returns `[]`, read stderr to distinguish no workflows or no remote Action uses from a clean scan.
- Add `--require-sha` to reject moving refs, `--fail-on-unknown` to reject unclassified refs, and repeat `--allow-owner OWNER` to restrict remote Actions by owner.
- `validate` exits 1 when actionlint finds a problem. `--json` emits JSON Lines, not one JSON array.
- `lint` preserves zizmor's exit status and output.
- Authentication, API, path, and other command errors are real failures. Read stderr before deciding what happened.
- JSON uses `null` when a SHA or tag cannot be resolved. Do not invent a value.

Keep stdout for JSON. Put notes and diagnostics on stderr or outside captured command output.

## Scope and edge cases

- `check`, `lint`, and `validate` accept `-C PATH` or `--repo PATH`; both default to the current directory.
- Action identifiers use `OWNER/REPO[/PATH...]`. Every slash-delimited segment must be nonempty; `PATH` is the manifest directory.
- Workflow scans cover `.yml` and `.yaml` files directly inside `.github/workflows`.
- `check` reads job-level reusable workflows and step-level Actions, including Actions in repository subpaths. It ignores local paths and `docker://` uses.
- Branches, unresolved refs, and full SHAs that differ from the current stable refs appear as unknown rather than outdated. A SHA alone does not prove commit order.
- `version` prefers the latest stable release, then a stable semantic tag. If neither exists, it may fall back to a non-semantic tag; it rejects semantic prerelease tags.
- If a documented command is absent, verify gh-actionkit is v0.5.0 or newer instead of improvising.
