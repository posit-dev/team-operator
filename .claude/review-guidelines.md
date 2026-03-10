# Team Operator Code Review Guidelines

## Reviewer Responsibilities

Your job is to review the **code itself**, not just the description or intent. In a world of agentic coding, the author may not have hand-written every line. That means:

- Read the actual diff, not just the PR summary
- Verify the code does what the description claims
- Look for correctness issues, not just style preferences

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

### Code Quality Fundamentals
- **Encapsulation**: Is internal state properly hidden? Are interfaces clean?
- **DRY**: Is there duplicated logic that should be extracted? But don't over-abstract — three similar lines can be better than a premature helper
- **Naming**: Do names reveal intent? Would a future reader understand this?
- **Complexity**: Is this more complicated than it needs to be?

### Security (Elevated Scrutiny)

These changes require extra review attention:
- File system operations (paths, permissions)
- Network operations (URLs, ports, proxies)
- Credential handling (secrets, tokens, keys)
- Kubernetes RBAC and network policies
- Cloud IAM policies and roles

## Review Checklist by Area

### API Changes (`api/`)
- [ ] Kubebuilder annotations are correct
- [ ] New fields have sensible defaults
- [ ] Validation rules are present
- [ ] Breaking changes have migration strategy

### Controller Changes (`internal/controller/`)
- [ ] Reconciliation is idempotent
- [ ] Error handling reports status correctly
- [ ] Config flows from Site -> Product correctly
- [ ] Both unit and integration tests exist

### Helm Chart (`dist/chart/`)
- [ ] Values have sensible defaults
- [ ] Templates render correctly
- [ ] RBAC permissions are minimal
- [ ] CRDs are up to date

### Flightdeck (`flightdeck/`)
- [ ] Go templates render correctly
- [ ] Static assets are properly served
- [ ] Configuration options are documented

## What NOT to Comment On

- Style issues handled by formatters (run `make fmt`)
- Personal preferences without clear benefit
- Theoretical concerns without concrete impact

## Comment Format

Use clear, actionable language:
- **Critical**: "This will break X because Y. Consider Z."
- **Important**: "This pattern differs from existing code in A. Recommend B for consistency."
- **Suggestion**: "Consider X for improved Y."

## Self-Review Norm

PR authors are expected to review their own diff before opening the PR and add inline comments to draw attention to:
- Lines they don't fully understand
- Areas of concern or uncertainty
- Key decision points reviewers should weigh in on

This balances effort between author and reviewer. If the author hasn't commented their PR, ask them to.
