# Contributing

## Setup

Everything is pinned in `mise.toml`. With [mise](https://mise.jdx.dev/) installed:

```bash
mise install
mise run build
mise run dev        # a golem server with this checkout built in, via golem-launcher
```

## Before you open a pull request

```bash
mise run fmt        # gofmt
mise run fix        # go fix's modernizations
mise run tidy       # go mod tidy
mise run lint       # golangci-lint
mise run test:changed   # repeats the tests your branch adds or changes
```

`mise run check` runs build, test, lint and the format, fix and tidy checks, as CI does
(jobs Build, Lint, Test, Fix, Format, Tidy).

## Commit messages and releases

Pull requests are squash-merged with their title as the commit message, and
[release-please](https://github.com/googleapis/release-please) builds the changelog and
the next version from those messages. A title must therefore be a
[Conventional Commit](https://www.conventionalcommits.org/): `feat: ...`, `fix(economy): ...`,
`feat!: ...` for a breaking change.
