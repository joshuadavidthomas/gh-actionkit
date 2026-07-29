# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Replace `check --json` fields `up_to_date` and `update_available` with `status`.

## [0.4.0]

### Added

- Add `skill` to print the exact bundled `SKILL.md` content to stdout.

### Changed

- Require the agent skill to verify gh-actionkit v0.3.0 or newer and upgrade older extension installs before use.
- Name every GitHub-backed command and limit repository path guidance to `check`, `lint`, and `validate` in the agent skill.
- Use one workflow file discovery path for Action checking and validation.

### Removed

- Remove the no-op `lint --no-pedantic` flag.

### Fixed

- Include allowed owners in the agent skill's Action policy command.
- Explain empty `check --json` scans on stderr while keeping stdout valid JSON.
- Report full commit SHAs that differ from current stable refs as unknown instead of assuming an update is available.

## [0.3.0]

### Added

- Added `check` policies for full-SHA pins, unknown refs, and allowed Action owners.
- Added `inspect` for repository facts, Action manifest details, and stable pinned references.
- Added `version --snippet` for copy-ready, full-SHA-pinned `uses:` lines.

## [0.2.0]

### Added

- Added prek and pre-commit hooks for checking, linting, and validating GitHub Actions workflows.
- Added an installable agent skill for gh-actionkit, including Claude Code plugin metadata.

### Changed

- Switched Action search and manifest verification to bounded GraphQL requests.
- Improved GitHub authentication and rate-limit errors, including retry times when GitHub provides them.
- Enabled zizmor online audits by default with the active GitHub CLI credentials.

### Removed

- Removed `search --fast`; search results now always contain a root `action.yml` or `action.yaml`.

## [0.1.0]

### Added

- Added `version`, `search`, `check`, `lint`, and `validate` commands.
- Added precompiled GitHub CLI extension releases for Linux, macOS, and Windows.

[Unreleased]: https://github.com/joshuadavidthomas/gh-actionkit/compare/v0.4.0...HEAD
[0.1.0]: https://github.com/joshuadavidthomas/gh-actionkit/releases/tag/v0.1.0
[0.2.0]: https://github.com/joshuadavidthomas/gh-actionkit/releases/tag/v0.2.0
[0.3.0]: https://github.com/joshuadavidthomas/gh-actionkit/releases/tag/v0.3.0
[0.4.0]: https://github.com/joshuadavidthomas/gh-actionkit/releases/tag/v0.4.0
