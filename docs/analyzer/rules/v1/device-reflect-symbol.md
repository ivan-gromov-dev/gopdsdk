# `device-reflect-symbol`

A reflect symbol is outside the audited device subset.

- Rule version: `v1`
- Category: `device`
- Default severity: `error`
- Confidence: `proven`
- Targets: `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `device-reflection-profile`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; use direct typed code or generated accessors. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore device-reflect-symbol -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
