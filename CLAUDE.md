# Team Operator

Kubernetes operator for managing Posit Team deployments.

## Project Structure

- **`api/`**: Kubernetes API/CRD definitions (core, product, keycloak, templates)
- **`cmd/`**: Main operator entry point
- **`internal/`**: Core operator logic and controllers
- **`config/`**: Kubernetes manifests and Kustomize configurations
- **`dist/chart/`**: Helm chart for deployment
- **`flightdeck/`**: Landing page dashboard component (separate Go module)
- **`client-go/`**: Generated Kubernetes client code
- **`docs/`**: User and contributor documentation

## Build and Development

```bash
just build          # Build operator binary to ./bin/team-operator
just test           # Run go tests
just run            # Run operator locally from source
just deps           # Install dependencies
just mgenerate      # Regenerate manifests after API changes
just helm-lint      # Lint Helm chart
just helm-template  # Render Helm templates locally
just helm-install   # Install operator via Helm
just helm-uninstall # Uninstall operator via Helm
```

## Namespaces

- **`posit-team-system`**: Where the operator runs
- **`posit-team`**: Where Site CRs and products are deployed

## Helm Installation

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace
```

## Git Worktrees

**Always use git worktrees instead of plain branches.** This enables concurrent Claude sessions in the same repo.

### Creating a Worktree

This repo is expected to live at `ptd-workspace/team-operator/`. The `../../.worktrees/` relative path resolves to `ptd-workspace/.worktrees/` in that layout.

```bash
# New branch
git worktree add ../../.worktrees/team-operator-<branch-name> -b <branch-name>

# Existing remote branch
git worktree add ../../.worktrees/team-operator-<branch-name> <branch-name>
```

Always prefix worktree directories with `team-operator-` to avoid collisions with other repos.

### After Creating a Worktree

No special setup needed. The `Justfile` and `Makefile` use relative paths, so they work in worktrees out of the box:

```bash
cd ../../.worktrees/team-operator-<branch-name>
just build    # builds to ./bin/team-operator
just test     # runs tests
```

### Cleaning Up

```bash
# From the main checkout
git worktree remove ../../.worktrees/team-operator-<branch-name>
# Prune stale worktree references
git worktree prune
```

### Rules

- **NEVER** use `git checkout -b` for new work — always `git worktree add`
- **NEVER** put worktrees inside the repo directory — always use `../../.worktrees/team-operator-<name>`
- Branch names: kebab-case, no slashes, no usernames (slashes break worktree directory paths)

## roborev Code Review

This repo uses [roborev](https://www.roborev.io/) for AI-assisted code review. If you haven't already, install the commit hook:

```bash
roborev install-hook
```

This enables automatic review submissions after each commit.

## Contributing

- **PR titles must follow conventional commit format** (`feat:`, `fix:`, `docs:`, etc.) - this is enforced by CI
- The repo uses squash merge, so PR title becomes the commit message
- semantic-release uses commit prefixes for version bumps: `feat:` = minor, `fix:` = patch, `feat!:` = major
- Run `just test` before committing
- See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines

## License

MIT License - see LICENSE file
