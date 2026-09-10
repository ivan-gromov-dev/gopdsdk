# `result-menu-image-offset`

Menu image offset is outside the accepted range.

- Rule version: `v1`
- Category: `result`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`
- Suppressible: `true`

## Contract

This diagnostic enforces: `menu-image-offset-bound`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

Replace only a constant value when intent is unambiguous. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore result-menu-image-offset -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
