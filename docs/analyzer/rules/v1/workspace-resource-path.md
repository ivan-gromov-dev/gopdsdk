# `workspace-resource-path`

Packaged resource path is non-portable, escaped, excluded, or case-colliding.

- Rule version: `v1`
- Category: `workspace`
- Default severity: `error`
- Confidence: `proven`
- Targets: `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `workspace-static-contract`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; renaming resources can affect every loader and release artifact. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore workspace-resource-path -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
