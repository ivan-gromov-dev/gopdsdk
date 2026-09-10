# `lifetime-microphone-samples-escape`

Callback-scoped microphone samples escape their callback.

- Rule version: `v1`
- Category: `lifetime`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`, `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `microphone-samples-callback-scope`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; the bounded destination and buffering policy is application-specific. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore lifetime-microphone-samples-escape -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
