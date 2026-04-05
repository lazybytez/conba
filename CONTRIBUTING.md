# Contributing

Thank you for your interest in contributing to Conba.

## Code of Conduct

This project follows the [Lazy Bytez Code of Conduct](https://github.com/lazybytez/.github/blob/main/docs/CODE_OF_CONDUCT.md).
Please read it before participating.

## Questions, Bugs and Feature Requests

Use [GitHub Issues](https://github.com/lazybytez/conba/issues) to report bugs or
request features. Search existing issues before opening a new one.

### Security Issues

Do **NOT** open a public issue for security vulnerabilities. See [SECURITY.md](SECURITY.md)
for responsible disclosure instructions.

## Contributing Code

### Process

1. Open or pick an issue and comment to claim it.
2. Branch from `main` using the conventions below.
3. Implement your changes with tests.
4. Open a pull request against `main`.

**PR requirements:** CI passes, at least one maintainer approval, linked to an issue.

### Branching

| Branch | Purpose |
|--------|---------|
| `main` | Stable — all PRs target here |
| `feature/*` | New features |
| `fix/*` | Bug fixes |

### Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/) enforced
by [commitlint](https://commitlint.js.org/).

Format: `prefix(scope): subject` (max 50 character subject)

**Prefixes:** `feat`, `fix`, `build`, `chore`, `ci`, `docs`, `perf`, `refactor`, `revert`,
`style`, `test`, `sec`

**Scopes:** `deps`, `devops`, `cli`, `backup`, `config`, `docker`, `restic`, `filter`,
`retention`, `logging`

### Development Setup

All build operations run inside Docker containers — no local Go installation required:

```sh
make build       # Build the binary
make test        # Run tests with race detector
make lint        # Run golangci-lint
make fmt         # Format code
```

See the [README](README.md) for full details.
