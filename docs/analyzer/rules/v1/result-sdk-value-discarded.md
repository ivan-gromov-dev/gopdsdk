# `result-sdk-value-discarded`

A contract-significant SDK result is discarded.

- Rule version: `v1`
- Category: `result`
- Default severity: `warning`
- Confidence: `proven`
- Targets: `shared`
- Suppressible: `true`

## Contract

This diagnostic enforces: `sdk-significant-results`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; the required full, empty, partial, or refresh behavior is application-specific. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore result-sdk-value-discarded -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
