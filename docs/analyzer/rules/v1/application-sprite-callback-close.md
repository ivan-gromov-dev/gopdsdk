# `application-sprite-callback-close`

An active sprite callback cannot close its participating sprites.

- Rule version: `v1`
- Category: `application`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`, `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `sprite-close-semantics`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; defer cleanup to application logic after delivery. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore application-sprite-callback-close -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
