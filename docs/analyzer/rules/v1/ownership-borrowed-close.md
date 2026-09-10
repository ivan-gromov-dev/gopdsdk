# `ownership-borrowed-close`

A borrowed handle is closed by a non-owner.

- Rule version: `v1`
- Category: `ownership`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`
- Suppressible: `true`

## Contract

This diagnostic enforces: `bitmap-table-borrowed-frame`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; close the owning aggregate instead. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore ownership-borrowed-close -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
