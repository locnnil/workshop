# GitHub Copilot Instructions

This file provides general project context for GitHub Copilot. For review-specific guidance, see:
- **Code Review**: `.github/agents/code-review.agent.md`
- **Documentation Review**: `.github/agents/doc-review.agent.md`

## Project Overview

Workshop is a tool for defining and handling ephemeral development environments. It uses a client-server architecture where `workshopd` daemon exposes a RESTful API to clients. The project is written in Go and distributed as a Snap package.

## Tech Stack

- **Language**: Go 1.21+
- **Container Backend**: LXD (primary abstraction target)
- **Packaging**: Snap (Snapcraft)
- **CLI Framework**: Cobra
- **Testing**: Go unit tests + Spread (integration/e2e)
- **Documentation**: Sphinx (reStructuredText)
- **CI**: GitHub Actions

## Repository Structure

### Core Directories
- `cmd/` — CLI entry points (`workshop`, `workshopd`, `workshopctl`, `sdk`)
- `client/` — Go client library for RESTful API communication
- `internal/` — Private packages (daemon, overlord, workshop core, interfaces, SDK handling)
- `tests/` — E2E test suites using Spread (`main/`, `integration/`, `docs-*/`)
- `snap/` — Snap packaging configuration
- `docs/` — Sphinx documentation (tutorial, how-to, explanation, reference)

### Build Configuration
- `go.mod` / `go.sum` — Go module dependencies
- `.golangci.yaml` / `.golangci.errcheck.yaml` — Linting configuration
- `.spread.yaml` — E2E testing with Spread framework
- `snap/snapcraft.yaml` — Snap package definition

## Coding Guidelines

See [`contributing.rst`](../contributing.rst) for detailed standards. Key points:

- **Error messages**: Lowercase, no trailing punctuation, actionable (`what was attempted: why it went wrong`)
- **Error handling**: Consistent `if err := f(); err != nil { return err }` pattern
- **Code organization**: Early returns over nested conditions; keep coupled elements adjacent
- **Testing**: Unit tests (`*_test.go`) adjacent to implementation; Spread tests for integration

## Available Resources

- **Contributing Guide**: [`contributing.rst`](../contributing.rst) — Setup, standards, workflow
- **Documentation Style**: [`docs/doc-style-guide.md`](../docs/doc-style-guide.md) — reST/Markdown conventions
- **PR Template**: [`.github/pull_request_template.md`](.github/pull_request_template.md) — Self-review checklist
- **Code Review Agent**: [`.github/agents/code-review.agent.md`](.github/agents/code-review.agent.md) — For PR code reviews
- **Docs Review Agent**: [`.github/agents/doc-review.agent.md`](.github/agents/doc-review.agent.md) — For PR documentation reviews

## GitHub Actions Workflows

- `lint.yaml` — golangci-lint on Go code
- `unit-tests.yaml` — Go unit tests
- `spread.yaml` — E2E tests with Spread
- `cover.yaml` — Coverage reports
- `automatic-doc-checks.yml` — Sphinx builds (fail on warnings)
- `doc-cover.yaml` — Documentation coverage map generation
- `release.yaml` — Builds release snaps + generates CLI reference PR
- `fixup.yaml` — Commit message format validation
- `scanning.yml` — Security scanning
- `sphinx-python-dependency-build-checks.yml` — Ensures Sphinx venv builds
- `markdown-style-checks.yml` — Checks style, spelling, and links

## Evolution Note

These instructions are living documentation. When Copilot misbehaves:
1. Note the specific failure mode
2. Identify which instruction file should address it
3. Propose minimal, high-signal edits (avoid essay-style additions)
4. Test with focused prompts before committing changes
