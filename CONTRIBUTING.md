# Contributing to Team Operator

Welcome! We appreciate your interest in contributing to the Team Operator project. This guide will help you get started with development and understand our contribution workflow.

## Table of Contents

- [Project Overview](#project-overview)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Code Review Guidelines](#code-review-guidelines)
- [Getting Help](#getting-help)

## Project Overview

Team Operator is a Kubernetes operator built with [Kubebuilder](https://book.kubebuilder.io/) that automates the deployment, configuration, and management of Posit Team products (Workbench, Connect, Package Manager, and Chronicle) within Kubernetes clusters.

> **Note**: This repository is under active development and is not yet ready for production use.

## Development Setup

### Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.25+** (version specified in `go.mod`)
- **Docker** (for building container images)
- **kubectl** (configured to access a Kubernetes cluster)
- **Kubernetes cluster** (1.29+ for testing; k3d works well for local development)
- **Just** command runner (`brew install just` on macOS, or see [installation guide](https://github.com/casey/just))
- **Helm** (for chart development and testing)

On macOS, you may also need `gsed` for some Makefile targets:

```bash
brew install gnu-sed
```

### Cloning the Repository

```bash
git clone https://github.com/posit-dev/team-operator.git
cd team-operator
```

### Installing Dependencies

```bash
just deps
```

### Building the Operator

```bash
just build
```

This compiles the operator binary to `./bin/team-operator`.

### Running Locally

To run the operator against your current Kubernetes context:

```bash
just run
```

For development with a local Kubernetes cluster:

```bash
# Create a k3d cluster
just k3d-up

# Install CRDs
just crds

# Run the operator
just run
```

### Running Tests

```bash
just test
```

For tests with envtest (Kubebuilder test framework):

```bash
just mtest
```

## Project Structure

| Directory | Description |
|-----------|-------------|
| `api/` | Kubernetes API/CRD definitions |
| `cmd/` | Main operator entry point |
| `internal/` | Core operator logic and controllers |
| `internal/controller/` | Reconciliation controllers for each resource type |
| `config/` | Kubernetes manifests and Kustomize configurations |
| `dist/chart/` | Helm chart for deployment |
| `flightdeck/` | Landing page dashboard component |
| `client-go/` | Generated Kubernetes client code |
| `pkg/` | Shared packages |
| `openapi/` | Generated OpenAPI specifications |
| `hack/` | Build and development scripts |

## Making Changes

### Branching Strategy

1. Create a feature branch from `main`:
   ```bash
   git checkout main
   git pull origin main
   git checkout -b your-feature-name
   ```

2. Keep branch names descriptive and use hyphens (avoid slashes):
   - Good: `add-workbench-scaling`, `fix-database-connection`
   - Avoid: `feature/workbench`, `fix/db`

### Commit Message Conventions

We use [Conventional Commits](https://www.conventionalcommits.org/). Each commit message should follow this format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation only changes
- `refactor:` - Code change that neither fixes a bug nor adds a feature
- `test:` - Adding or correcting tests
- `chore:` - Changes to the build process or auxiliary tools

**Examples:**
```
feat(connect): add support for custom resource limits
fix(workbench): resolve database connection timeout
docs: update installation instructions
refactor(controller): simplify reconciliation logic
```

### Code Style Guidelines

1. **Run formatters before committing:**
   ```bash
   just format
   ```
   Or using make:
   ```bash
   make fmt
   ```

2. **Run linting:**
   ```bash
   go vet ./...
   ```

3. **Follow existing patterns** - New code should look like it belongs in the codebase.

4. **Keep functions focused** - Each function should do one thing well.

5. **Use descriptive names** - Names should reveal intent.

### Adding New CRD Fields

When adding new fields to Custom Resource Definitions:

1. **Update the API types** in `api/`:
   - Add the new field with appropriate JSON tags and Kubebuilder annotations
   - Include validation rules where appropriate

2. **Regenerate manifests:**
   ```bash
   just mgenerate
   ```

3. **Update controller logic** in `internal/controller/` if needed.

4. **Add tests** for the new functionality.

5. **Update Helm chart** in `dist/chart/` if the change affects deployment.

Example of adding a new field:

```go
// +kubebuilder:validation:Optional
// +kubebuilder:default:=1
// Replicas is the number of instances to deploy
Replicas *int32 `json:"replicas,omitempty"`
```

## Testing

### Unit Tests

Run all unit tests:

```bash
just test
```

Or run Go tests directly:

```bash
go test -v ./...
```

### Running Specific Tests

To run tests for a specific package:

```bash
go test -v ./internal/controller/...
```

To run a specific test:

```bash
go test -v ./internal/controller/... -run TestReconcile
```

### Integration Tests with envtest

The project uses Kubebuilder's envtest for integration testing:

```bash
just mtest
```

This sets up a local Kubernetes API server for testing without requiring a full cluster.

### Helm Chart Testing

```bash
# Lint the chart
just helm-lint

# Render templates locally (useful for debugging)
just helm-template
```

### Coverage Reports

After running tests, view coverage:

```bash
go tool cover -func coverage.out
```

## Pull Request Process

### Before Submitting

1. **Ensure all tests pass:**
   ```bash
   just test
   ```

2. **Format your code:**
   ```bash
   just format
   ```

3. **Verify no uncommitted changes from generation:**
   ```bash
   git diff --exit-code
   ```

4. **Lint the Helm chart:**
   ```bash
   just helm-lint
   ```

### PR Description

Include the following in your PR description:

1. **Summary** - What does this PR do?
2. **Motivation** - Why is this change needed?
3. **Testing** - How was this tested?
4. **Breaking changes** - Does this introduce any breaking changes?
5. **Related issues** - Link to any related GitHub issues

### CI Checks

The following checks must pass:

- **Build** - The operator must compile successfully
- **Unit tests** - All tests must pass
- **Kustomize** - Kustomization must build without errors
- **Helm lint** - Chart must pass linting
- **Helm template** - Templates must render correctly
- **No diff** - Generated files must be committed

### Review Expectations

- PRs require at least one approval before merging
- Address all review comments or explain why you disagree
- Keep PRs focused - smaller PRs are easier to review
- Respond to feedback promptly

## Code Review Guidelines

We follow specific guidelines for code review. For detailed review standards, see [`.claude/review-guidelines.md`](.claude/review-guidelines.md).

### Core Principles

- **Simplicity** - Prefer explicit over clever
- **Maintainability** - Follow existing patterns in the codebase
- **Security** - Extra scrutiny for credential handling, RBAC, and network operations

### Review Checklist by Area

**API Changes (`api/`):**
- Kubebuilder annotations are correct
- New fields have sensible defaults
- Validation rules are present
- Breaking changes have migration strategy

**Controller Changes (`internal/controller/`):**
- Reconciliation is idempotent
- Error handling reports status correctly
- Config flows from Site -> Product correctly
- Both unit and integration tests exist

**Helm Chart (`dist/chart/`):**
- Values have sensible defaults
- Templates render correctly
- RBAC permissions are minimal
- CRDs are up to date

**Flightdeck (`flightdeck/`):**
- Go templates render correctly
- Static assets are properly served
- Configuration options are documented

### What NOT to Comment On

- Style issues handled by formatters (run `make fmt`)
- Personal preferences without clear benefit
- Theoretical concerns without concrete impact

## Getting Help

If you have questions or need help:

1. **Check existing documentation** - README.md, this guide, and inline code comments
2. **Search existing issues** - Your question may have been answered before
3. **Open an issue** - For bugs, feature requests, or questions
4. **Contact Posit** - For production use inquiries, [contact Posit](https://posit.co/schedule-a-call/)

## Quick Reference

| Task | Command |
|------|---------|
| Build | `just build` |
| Test | `just test` |
| Run locally | `just run` |
| Format code | `just format` |
| Regenerate manifests | `just mgenerate` |
| Install CRDs | `just crds` |
| Helm lint | `just helm-lint` |
| Helm template | `just helm-template` |
| Helm install | `just helm-install` |
| Create k3d cluster | `just k3d-up` |
| Delete k3d cluster | `just k3d-down` |

Thank you for contributing to Team Operator!
