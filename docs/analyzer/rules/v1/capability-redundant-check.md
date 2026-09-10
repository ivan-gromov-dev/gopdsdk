# `capability-redundant-check`

A capability check has a statically known result.

- Rule version: `v1`
- Category: `capability`
- Default severity: `information`
- Confidence: `proven`
- Targets: `shared`, `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `context-optional-capability`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; removing a branch can change application behavior. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore capability-redundant-check -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
