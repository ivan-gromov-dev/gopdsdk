# `device-panic-cleanup`

Explicit device panic bypasses potentially pending deferred calls.

- Rule version: `v1`
- Category: `device`
- Default severity: `warning`
- Confidence: `likely`
- Targets: `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `device-panic-profile`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; terminal failure and cleanup policy require an explicit decision. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore device-panic-cleanup -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
