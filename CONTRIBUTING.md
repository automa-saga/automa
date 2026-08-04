# Contributing to Automa

Thanks for your interest in contributing! Feedback, bug reports, documentation
fixes, and code contributions are all very welcome.

This document explains how to set up your environment, the standards a change is
expected to meet, and how the pull-request process works.

## Ways to contribute

- **Report a bug** — open an issue with a minimal reproduction (ideally a small
  `main.go` or a failing test) and the observed vs. expected behavior.
- **Suggest a feature** — open an issue describing the use case first. Because
  automa follows a [normative spec](docs/spec), behavioral changes are discussed
  against the spec before implementation (see [Specs & conformance](#specs--conformance)).
- **Improve docs or examples** — README, GoDoc comments, and the
  [`examples/`](examples) are all fair game.
- **Submit code** — see the workflow below.

## Prerequisites

- **Go 1.26+** (see [`go.mod`](go.mod)).
- **[Task](https://taskfile.dev/installation/)** — the task runner used for all
  common commands (`Taskfile.yaml`).
- **[golangci-lint](https://golangci-lint.run/)** — installed automatically by
  `task lint` if it is not already on your `PATH`.

## Getting started

```sh
# Fork and clone your fork, then:
git clone https://github.com/<your-username>/automa.git
cd automa

# Build, lint, and test to confirm a clean baseline.
task build
task lint
task test
```

Common tasks:

| Command | What it does |
| --- | --- |
| `task build` | `go build -v ./...` |
| `task fmt` | Format the code (`go fmt ./...`) |
| `task lint` | Check formatting and run `golangci-lint run` |
| `task test` | Run the unit tests (`-race -cover`) and write `unit-test-report.md` |
| `task test:coverage` | Run tests with coverage gating (`.testcoverage.yml`) |
| `task clean` | Remove test artifacts and clear the Go build/test cache |

You can also run an example directly, e.g. `go run ./examples/quickstart`.

## Making a change

1. Create a branch off `main`.
2. Make your change, keeping commits focused.
3. Add or update tests. Bug fixes should come with a test that fails before the
   fix and passes after; new behavior needs coverage.
4. Run `task fmt`, `task lint`, and `task test` locally — CI runs the same
   checks and will fail on formatting or lint issues.
5. Update documentation (README, GoDoc, examples) when behavior or the public
   API changes.

### Definition of done

- [ ] `task build` succeeds.
- [ ] `task lint` is clean (formatting + `golangci-lint`).
- [ ] `task test` passes, including new/updated tests.
- [ ] Public API changes are documented and reflected in the spec/fixtures where applicable.

## Specs & conformance

Automa is spec-first. The behavior is defined by the normative, language-neutral
specs and pinned by cross-implementation conformance fixtures:

- [`docs/spec/core-spec.md`](docs/spec/core-spec.md) — the core saga model.
- [`docs/spec/durability-spec.md`](docs/spec/durability-spec.md) — crash recovery.
- [`docs/spec/conformance`](docs/spec/conformance) — behavior, journal, and
  serialization fixtures.

If your change alters observable behavior (execution/rollback semantics, the
report tree, the journal, or serialization), it must also update the relevant
spec section and conformance fixtures in the same PR. Since `v1.0.0` the public
API is stable ([semver](https://semver.org/)); breaking changes are reserved for
a new major version — please open an issue to discuss before starting one.

## Commits & sign-off

- **DCO sign-off is required.** Every commit must be signed off, certifying the
  [Developer Certificate of Origin](https://developercertificate.org/). Add the
  `Signed-off-by` trailer with `git commit -s`.
- **Signed commits are encouraged.** Please use verified, cryptographically
  signed commits where possible.
- **Commit messages** should follow the
  [Conventional Commits](https://www.conventionalcommits.org/) /
  [commitizen](https://commitizen-tools.github.io/commitizen/) format.

## Pull requests

- Open your PR against `main`.
- PRs are **squash-merged**, and the **PR title becomes the squashed commit
  message** — so the PR title **must** follow the commitizen/Conventional Commits
  format (e.g. `feat: add parallel step group`, `fix: correct rollback ordering`),
  even if individual commits do not. This drives automated release versioning.
- Keep PRs focused; smaller, self-contained changes are reviewed faster.
- Fill in what changed and why, and link any related issue.
- Ensure CI is green before requesting review.

New to open source? [First Contributions](https://github.com/firstcontributions/first-contributions)
is a friendly starting point.

## License

By contributing, you agree that your contributions will be licensed under the
project's [Apache License 2.0](LICENSE).
