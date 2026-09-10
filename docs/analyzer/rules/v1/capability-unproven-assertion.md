# `capability-unproven-assertion`

A related capability check cannot be connected to this assertion.

- Rule version: `v1`
- Category: `capability`
- Default severity: `warning`
- Confidence: `likely`
- Targets: `shared`, `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `context-optional-capability`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; pass a checked capability or establish a local fallback explicitly. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore capability-unproven-assertion -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
