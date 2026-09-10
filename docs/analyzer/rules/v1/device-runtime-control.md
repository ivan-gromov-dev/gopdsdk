# `device-runtime-control`

An application runtime-control hook is unavailable on device.

- Rule version: `v1`
- Category: `device`
- Default severity: `error`
- Confidence: `proven`
- Targets: `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `device-runtime-control-profile`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; the device runtime profile is fixed. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore device-runtime-control -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
