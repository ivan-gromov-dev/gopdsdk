# `capability-video-availability`

Video capability use is not valid for the selected target contract.

- Rule version: `v1`
- Category: `capability`
- Default severity: `error`
- Confidence: `proven`
- Targets: `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `video-capability-availability`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; target fallback and feature gating are application-specific. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore capability-video-availability -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
