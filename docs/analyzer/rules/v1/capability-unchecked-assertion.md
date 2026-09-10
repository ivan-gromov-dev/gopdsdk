# `capability-unchecked-assertion`

An optional Context capability is asserted without a check.

- Rule version: `v1`
- Category: `capability`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`, `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `context-optional-capability`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; a fallback must be chosen explicitly; never generate a panicking quick fix. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore capability-unchecked-assertion -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
