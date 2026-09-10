# `ownership-retained-close`

A handle is closed while retained by another live object.

- Rule version: `v1`
- Category: `ownership`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`
- Suppressible: `true`

## Contract

This diagnostic enforces: `menu-image-retention`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; detach or clear the retaining edge before close. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore ownership-retained-close -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
