# Contributing to iFlowKit CLI

Thanks for your interest in contributing!

This repository contains **iFlowKit CLI**, a Go-based command-line tool focused on syncing SAP CPI Integration Packages with a Git repository.

## Ground rules

- Be respectful and constructive. See [Code of Conduct](CODE_OF_CONDUCT.md).
- Keep security and secrets in mind. See [Security Policy](SECURITY.md).
- By contributing, you agree that your contributions will be licensed under the project's [Apache-2.0 license](LICENSE).

## What you can contribute

- Bug reports and reproducible test cases
- Documentation improvements (both `docs/docs-en` and `docs/docs-tr`)
- New features / commands
- Refactors that improve readability, testability, and correctness
- Test coverage and CI improvements

## Development setup

### Prerequisites

- Go **1.22+** (see `go.mod`)
- Git
- (Optional) `golangci-lint`

### Clone & build

```bash
git clone <your-fork-url>
cd iflowkit-cli

go mod download
go build ./...
```

### Run locally

```bash
go run ./cmd/iflowkit --help
```

## Project layout (high level)

- `cmd/iflowkit/` – CLI entrypoint
- `internal/app/` – CLI wiring, parsing, global flags
- `modules/*/` – feature modules (e.g. `modules/sync`)
- `internal/common/*/` – reusable toolkits (git/cpi/files/logging/errors)
- `docs/` – documentation (language folders)

## Coding standards

### Go formatting and style

- Always run `gofmt` (and `goimports` if you use it).
- Prefer small packages with clear responsibilities.
- Avoid hidden side effects; keep I/O boundaries obvious.

### Error handling & logging

- Wrap errors with context (what operation failed, and why).
- Never log secrets (tokens, client secrets, private keys).
- Prefer actionable error messages: *what failed* + *what to do next*.

### CLI behavior guidelines

- Commands should be script-friendly:
  - non-zero exit codes on failure
  - stable, predictable output
- Avoid breaking flags lightly; when a breaking change is needed, document it in `CHANGELOG.md`.

## Testing

Run all tests:

```bash
go test ./...
```

If you add or change behavior, include tests whenever possible:
- Unit tests for pure logic
- Interface-based tests for external services (CPI / Git)
- Regression tests for fixed bugs

## Documentation expectations

If you change CLI behavior, update docs in **both** languages:

- `docs/docs-en/...`
- `docs/docs-tr/...`

If the change affects user workflows, add at least:
- a short example command
- expected output or behavior
- important caveats (auth, deletes, retries, etc.)

## Commit & PR conventions

### Suggested commit style

We recommend **Conventional Commits**:

- `feat(sync): ...`
- `fix(cpix): ...`
- `refactor(gitx): ...`
- `docs: ...`
- `chore: ...`

### Pull request checklist

Before opening a PR, please ensure:

- [ ] `go test ./...` passes
- [ ] `gofmt` has been applied
- [ ] Docs updated (EN + TR) if user-facing behavior changed
- [ ] `CHANGELOG.md` updated under **Unreleased** (if applicable)
- [ ] No secrets were added (keys, tokens, tenant URLs with credentials)

## Reporting bugs

Please open a GitHub issue with:

- iFlowKit CLI version (or commit hash)
- OS and Go version
- Exact command and flags used (redact secrets)
- Logs (use `--log-level debug`), again **redacting secrets**
- Expected vs actual result
- If possible: a minimal repro repo / sample package (sanitized)

## Proposing changes

For larger changes, start with an issue describing:

- Problem statement
- Proposed approach
- Alternatives considered
- Impact on commands/config/transports
- Testing strategy
