# `lifetime-bitmap-data-escape`

Callback-scoped bitmap data escapes its callback.

- Rule version: `v1`
- Category: `lifetime`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`, `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `bitmap-data-callback-scope`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; the required copy and dirty-row policy is application-specific. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore lifetime-bitmap-data-escape -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
