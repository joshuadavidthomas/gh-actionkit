# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Switched Action search and manifest verification to bounded GraphQL requests.
- Improved GitHub authentication and rate-limit errors, including retry times when GitHub provides them.

### Removed

- Removed `search --fast`; search results now always contain a root `action.yml` or `action.yaml`.

## [0.1.0] - 2026-07-23

### Added

- Added `version`, `search`, `check`, `lint`, and `validate` commands.
- Added precompiled GitHub CLI extension releases for Linux, macOS, and Windows.

[Unreleased]: https://github.com/joshuadavidthomas/gh-actionkit/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/joshuadavidthomas/gh-actionkit/releases/tag/v0.1.0
