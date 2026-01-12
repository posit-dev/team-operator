# PTD Code Review Guidelines

## Core Principles

### Simplicity
- Prefer explicit over clever
- Functions should do one thing
- Names should reveal intent
- Avoid premature abstraction

### Maintainability
- Follow existing patterns in the codebase
- New code should look like it belongs
- Dependencies should be minimal and justified
- Breaking changes need migration paths

### Security (Elevated Scrutiny)

These changes require extra review attention:
- File system operations (paths, permissions)
- Network operations (URLs, ports, proxies)
- Credential handling (secrets, tokens, keys)
- Kubernetes RBAC and network policies
- Cloud IAM policies and roles

## Review Checklist by Area

### Team Operator (`team-operator/`)

#### API Changes (`api/`)
- [ ] Kubebuilder annotations are correct
- [ ] New fields have sensible defaults
- [ ] Validation rules are present
- [ ] Breaking changes have migration strategy

#### Controller Changes (`internal/controller/`)
- [ ] Reconciliation is idempotent
- [ ] Error handling reports status correctly
- [ ] Config flows from Site -> Product correctly
- [ ] Both unit and integration tests exist

### PTD CLI (`cmd/`)
- [ ] Commands support `--verbose` flag
- [ ] Cloud operations use Target interface
- [ ] Auto-completion works for new arguments
- [ ] Error messages are actionable

### Python/Pulumi (`python-pulumi/`)
- [ ] Configuration uses dataclasses
- [ ] Pulumi resources have proper typing
- [ ] Cloud provider abstraction is maintained
- [ ] No hardcoded credentials or regions

## What NOT to Comment On

- Style issues handled by formatters (run `just format`)
- Personal preferences without clear benefit
- Theoretical concerns without concrete impact

## Comment Format

Use clear, actionable language:
- **Critical**: "This will break X because Y. Consider Z."
- **Important**: "This pattern differs from existing code in A. Recommend B for consistency."
- **Suggestion**: "Consider X for improved Y."
