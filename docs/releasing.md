# Releasing ActionKit

ActionKit publishes precompiled GitHub CLI extension binaries with `cli/gh-extension-precompile`.

## What gets published

Each `v*` tag creates a GitHub release with binaries for supported Linux, macOS, and Windows architectures. The release includes a `checksums.txt` manifest, and the workflow creates an artifact attestation for every binary.

GitHub CLI chooses the correct asset when a user runs:

```console
gh extension install joshuadavidthomas/gh-actionkit
```

## How to cut a release

1. Update `CHANGELOG.md` with the release version and date.
2. Ensure the main bookmark has passed CI.
3. Create and push a lightweight tag from the release commit:

```console
jj tag set v0.1.0 -r main
jj git push --remote origin --tag v0.1.0
```

The release workflow tests the tagged commit, builds each platform binary, injects the tag into `gh actionkit --version`, creates attestations, and publishes the GitHub release.

## Local version

Local builds report `dev`:

```console
just build
./gh-actionkit --version
```

Release builds set `main.version` through the precompile action's Go build options.
