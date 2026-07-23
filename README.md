# ActionKit

ActionKit is a GitHub CLI extension for finding, checking, and validating GitHub Actions.

The Go port is under development. It can look up action versions, search for actions, and audit workflows with zizmor:

```console
gh actionkit version actions/checkout
gh actionkit search "docker build"
gh actionkit lint
```

## Install for development

ActionKit requires the [GitHub CLI](https://cli.github.com/) with an authenticated account and Go 1.25 or newer. The `lint` command also requires [zizmor](https://docs.zizmor.sh/installation).

```console
just install
```

Run the extension from any directory:

```console
gh actionkit version actions/checkout
gh actionkit version actions/checkout --json
gh actionkit search checkout --limit 5
gh actionkit lint --pedantic
```

## Planned commands

The Python prototype also provides `check` and `validate`. These commands will move into ActionKit as native Go implementations.

## Development

```console
just check
```

## License

ActionKit is licensed under the [MIT License](LICENSE).
