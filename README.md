# ActionKit

ActionKit is a GitHub CLI extension for finding, checking, and validating GitHub Actions.

The Go port is under development. The first available command looks up the current stable release and its major-version tag:

```console
gh actionkit version actions/checkout
```

## Install for development

ActionKit requires the [GitHub CLI](https://cli.github.com/) with an authenticated account and Go 1.25 or newer.

```console
just install
```

Run the extension from any directory:

```console
gh actionkit version actions/checkout
gh actionkit version actions/checkout --json
```

## Planned commands

The Python prototype also provides `search`, `check`, `lint`, and `validate`. These commands will move into ActionKit as native Go implementations or thin adapters.

## Development

```console
just check
```

## License

ActionKit is licensed under the [MIT License](LICENSE).
