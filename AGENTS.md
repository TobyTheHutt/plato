# Agent Guidance for Plato

This file defines how non-human contributors should work in this repository.

## Scope

Use only documentation that exists in this repository:
- `README.md`
- `LICENSE`
- `AGENTS.md`

If a requested reference is missing, say so and continue with available context.

## Priority

Use this order when instructions conflict:
1. Direct user instruction
2. `AGENTS.md`
3. `README.md`
4. `LICENSE`

If the conflict is unclear, ask the user before risky changes.

## Working Rules

- Keep changes small and focused on the request.
- Preserve behavior unless the user asks for behavior changes.
- Explain tradeoffs when there is more than one valid option.
- Run relevant checks when possible and report results.
- For any relevant change, test coverage above 90 percent is mandatory.
- Do not invent project docs, folders, or workflows that are not present.

## Writing Style

- Maintain natural human language at all times.
- Structural elements are welcome, including tables, bullet lists, and clear line breaks.
- Do not use semicolons.
- Do not use em dashes.

## Allowed Without Extra Approval

- Bug fixes scoped to requested files
- Test or documentation updates that match code changes
- Refactors that do not change behavior
- Formatting or lint cleanup

## Require Explicit User Approval

- Dependency changes
- Public API or data format changes
- Security, authentication, or permission model changes
- License changes
- Destructive git or filesystem operations

## Reporting

Summarize:
- Files changed
- Commands run
- Validation completed
- Open risks or follow ups
