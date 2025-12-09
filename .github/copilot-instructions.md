# GitHub Copilot Review Instructions

This file provides custom instructions for GitHub Copilot when reviewing pull requests for the Workshop project.

## Project Overview

Workshop is a tool for defining and handling ephemeral development environments. It uses a client-server architecture where `workshopd` daemon exposes a RESTful API to clients. The project is written in Go and distributed as a Snap package.

## Repository Structure

### Core Directories
- `cmd/` - Command-line entry points
  - `cmd/workshop/` - Main CLI client
  - `cmd/workshopd/` - Daemon server
  - `cmd/workshopctl/` - Control utility
  - `cmd/sdk/` - SDK management tool
- `client/` - Go client library for RESTful API communication
- `internal/` - Private application packages
  - `internal/daemon/` - REST API implementation (`api.go` defines routes)
  - `internal/overlord/` - State management and orchestration
  - `internal/workshop/` - Core workshop logic and backend abstractions
  - `internal/interfaces/` - Plugin system for SDK connections
  - `internal/sdk/` - SDK handling and management
- `tests/` - End-to-end test suites using Spread
  - `tests/main/` - Core workshop scenarios
  - `tests/integration/` - LXD integration tests
  - `tests/docs-tutorial/` and `tests/docs-how-to/` - Documentation validation
- `snap/` - Snap packaging configuration
- `docs/` - Documentation source (reStructuredText)
  - `docs/tutorial/` - Step-by-step learning materials
  - `docs/how-to/` - Task-oriented guides
  - `docs/explanation/` - Conceptual information
  - `docs/reference/` - Technical specifications and API details

### Build Configuration
- `go.mod` / `go.sum` - Go module dependencies
- `.golangci.yaml` / `.golangci.errcheck.yaml` - Linting configuration
- `.spread.yaml` - End-to-end testing with Spread framework
- `snap/snapcraft.yaml` - Snap package definition

## Code Review Requirements

### 1. Commit Message Standards
In Workshop, commit messages differ from conventional commits in capitalization:

```
Ensure correct permissions and ownership for the content mounts

 * Work around an LXD issue regarding empty dirs:
   https://github.com/canonical/lxd/issues/12648

 * Ensure the source directory is owned by the user running a workshop.

Links:
- ...
- ...
```

- **No type prefixes** in commit messages (e.g., `fix`, `feat`) - these are used for **branch naming** instead:
  - `canonical/feat/workspace-start`
  - `canonical/fix/spread-tests-github`
  - `canonical/chore/update-lxd`
- **Exception**: Documentation commits must use `Doc:` type prefix with optional scope:
  - `Doc[chore]: Align references`

### 2. Code Quality Standards

#### Avoid Nested Conditions
Refrain from nesting conditions to enhance readability and maintainability. Use early returns instead.

#### Eliminate Dead Code and Redundant Comments
Remove unused or obsolete code and comments to promote a cleaner codebase and reduce confusion.

#### Normalize Symmetries
Handle identical operations consistently using a uniform approach:

```go
// one way to handle errors
if err := f(); err != nil {
   ...
}

// one way to handle multiple returns
val, err := f()
if err != nil {
   ...
}
```

#### Code Organization
- **Keep coupled elements adjacent**: Test data should be stored as close as possible to the test
- **Variable declaration**: Put variable declaration and initialization together
- **Expression clarity**: Divide large expressions into digestible and self-explanatory ones
- **Logical separation**: Put a blank line between two logically different chunks of code

### 3. Error Handling

Follow the established error handling pattern consistently:

```go
// Preferred error handling
if err := f(); err != nil {
   return err
}

// For multiple returns
val, err := f()
if err != nil {
   return err
}
```

#### Error Messages
- Start in lowercase (not sentence case)
- No trailing punctuation
- Provide actionable and specific context
- Follow the style guide in `docs/contributing.rst`
- Common template: `what was attempted: why it went wrong`

### 4. Architecture and Design

#### Client-Server Pattern
- `workshopd` daemon exposes a RESTful API (see `internal/daemon/api.go`)
- Client library in `client/` package communicates via Unix domain socket
- State management handled by `internal/overlord/state/`

#### Backend Abstraction
- Workshop uses backend abstraction for container management (primarily LXD)
- Backends implement common interface in `internal/workshop/workshop.go`

#### Interface System
- Plugin-like architecture in `internal/interfaces/` for SDK connections
- Each interface defines connection types and policies

### 5. Testing Requirements

#### Unit Tests
- Go unit tests: `go test ./...`
- Must maintain existing test coverage
- Test files located adjacent to implementation (`*_test.go`)

#### Integration Tests
- Spread framework used for end-to-end tests (`.spread.yaml`)
- Test suites in `tests/main/`, `tests/integration/`, `tests/docs-tutorial/`, `tests/docs-how-to/`
- Install Spread from the custom fork: https://github.com/dmitry-lyfar/spread  
  The custom fork is required because it includes patches and features necessary for Workshop's integration tests (such as improved LXD support and bug fixes) that are not yet available in the official Spread release.

#### Linting
- `golangci-lint` configured in `.golangci.yaml`
- Additional errcheck configuration in `.golangci.errcheck.yaml`
- Run locally before submitting: `golangci-lint run`

### 6. API Changes

When modifying the REST API:
- Update route definitions in `internal/daemon/api.go`
- Maintain API versioning (`/v1/...` prefix)
- Update client methods in `client/` package
- Consider backward compatibility
- Document changes in release notes

### 7. CLI Changes

For command-line interface modifications:
- Commands built using Cobra framework
- Auto-generate CLI reference documentation: `go run ./cmd/workshop generate-docs`
- Uses `gencodo` module for converting command metadata to `.rst` files
- CLI reference must be updated as part of release workflow

### 8. Reversibility

When making decisions that might be costly to reverse, explicitly state the rationale in the PR description. This helps understand the reasoning and facilitates better collaboration.

## Review Output Format

When providing pull request feedback, structure your response to include:

1. **Code Impact Summary**: Overview of code changes and affected components
2. **Architecture Review**: Assess whether changes align with the client-server architecture and backend abstractions
3. **Code Quality**: Evaluate adherence to coding standards (nested conditions, error handling, code organization)
4. **Testing Coverage**: Verify appropriate unit and integration tests are included
5. **Documentation Impact**: Check if code changes require documentation updates
6. **API/CLI Changes**: Flag any breaking changes or version compatibility issues
7. **Coverage Analysis**: For documentation changes, reference `coverage.md` and suggest missing `.. @artefact` comments
8. **Recommendations**: Specific actionable suggestions for improvement

Keep reviews constructive and aligned with the project's standards as defined in `docs/contributing.rst` and this guide.

## Documentation Review Requirements

**When reviewing changes to `docs/` directory, README files, docstrings, or other documentation-related areas**, apply the following comprehensive documentation standards.

The Workshop project follows a structured documentation approach based on the [Canonical documentation starter pack](https://github.com/canonical/sphinx-docs-starter-pack).

### Documentation Style Requirements
- **Markup**: reStructuredText (reST) is the preferred format
- **Style Guide**: Follow the [Workshop documentation style guide](../docs/doc-style-guide.md) for project-specific conventions, and the [Canonical reST style guide](https://canonical-starter-pack.readthedocs-hosted.com/stable/reference/style-guide/) for general patterns
- **Quoting**: When making suggestions related to style, you MUST quote the specific section from `docs/doc-style-guide.md` that supports your suggestion.
- **Building**: Documentation is built using a custom Workshop in-project SDK located in `.workshop/starter-pack`
- **Testing**: All documentation changes must pass Sphinx build without warnings

For comprehensive documentation guidelines, refer to `docs/doc-style-guide.md` and the Documentation section (search for "contributing_doc") in the contributing guide.

### 1. Documentation Impact Assessment
- **Existing Documentation**: Assess whether the PR affects any existing documentation files
- **Cross-references**: Check if internal links, references, and navigation remain intact
- **Coverage Gaps**: Identify if the PR introduces new functionality that lacks documentation coverage

### 2. Artefact Coverage Analysis
The project uses an automated coverage mechanism that tracks documentation coverage of key concepts and commands:

- **Coverage File**: Leverage the automated coverage report in `docs/coverage.md` to evaluate documentation completeness
- **Coverage Script**: The coverage is generated by `docs/coverage.py` which scans for `.. @artefact` comments
- **Missing Artefacts**: For any non-trivial functionality introduced in the PR, suggest adding appropriate `.. @artefact` comments

#### Artefact Comment Format
The `.. @artefact` comments should be placed in documentation files to mark important concepts, commands, or interfaces:

```restructuredtext
.. @artefact workshop command-name
.. @artefact interface-name interface
.. @artefact SDK concept
```

### 3. Documentation Quality Checklist

Check adherence to the Workshop documentation style guide (`docs/doc-style-guide.md`):

#### File Structure and Naming
- **File names**: Lowercase with dashes (e.g., `connect-vscode.rst`, not `ConnectVSCode.rst`)
- **Metadata block**: Every page must have `.. meta::` block with description after anchor label
- **Anchor labels**: Use prefixes (`tut_`, `how_`, `exp_`, `ref_`) with underscores (e.g., `.. _how_add_actions:`)

#### Content Structure
- **Consistency**: Ensure new documentation follows existing style and formatting patterns
- **Completeness**: Verify that new features are adequately documented with examples
- **Navigation**: Check that new documentation is properly integrated into the table of contents (`toctree`)
- **Cross-references**: Validate that internal links use proper reST reference format (`:ref:` preferred, `:doc:` only for `index.rst` and `release-notes/index.rst`)
- **Code Examples**: 
  - Use `console` lexer with `$` prompts (non-selectable)
  - Include captions for configuration examples
  - Show `sudo` explicitly when needed
  - Indent output with two spaces

#### Formatting Conventions
- **Product names**: Workshop, SDKcraft, LXD (proper capitalization); use `|ws_markup|` and `|sdk_markup|` substitutions
- **Inline markup**: Use semantic roles (`:program:`, `:command:`, `:file:`, `:envvar:`, `:samp:`)
- **Placeholders**: Uppercase in angle brackets (e.g., `:samp:`workshop launch {WORKSHOP}`)
- **Admonitions**: Use `.. note::` and `.. warning::` appropriately
- **Spacing**: Two-line gaps after major sections, code samples, lists, and tables

### 4. Specific Documentation Types
- **CLI Changes**: Verify that command-line interface changes are reflected in the auto-generated CLI reference
- **Configuration**: Check that new configuration options are documented in the reference section
- **Tutorials**: Ensure tutorial content follows the established progressive learning structure
- **API Changes**: Validate that API modifications are properly documented in the reference materials

### 5. Documentation PR Requirements
- **PR Title**: Prefix documentation PRs with `Doc:`
- **Scope**: Documentation PRs should be limited to the `docs/` directory where possible
- **Author Review**: Include the technical author in the review process

## CI/CD and Automation

### GitHub Actions Workflows (`.github/workflows/`)
- `lint.yaml` - Runs golangci-lint on Go code
- `unit-tests.yaml` - Runs Go unit tests
- `spread.yaml` - Executes end-to-end tests with Spread
- `cover.yaml` - Generates test coverage reports
- `automatic-doc-checks.yml` - Builds documentation and fails on Sphinx warnings
- `doc-cover.yaml` - Updates documentation coverage map
- `release.yaml` - Builds release snaps and generates CLI reference PR
- `fixup.yaml` - Validates commit message format
- `scanning.yml` - Security scanning
- `sphinx-python-dependency-build-checks.yml` - Ensures Sphinx venv can be built
- `markdown-style-checks.yml` - Checks style, spelling, and links

### Release Process
When reviewing release-related changes, ensure:
1. Unit, integration, and documentation tests pass
2. Documentation and release notes are updated
3. Release tag follows semantic versioning
4. Release workflow will build snaps for supported architectures
5. Auto-generated CLI reference PR will be created
6. Schema files updated (`docs/reference/definition-files/schema*.json`)
7. Coverage map refreshed via `doc-cover.yaml` workflow
