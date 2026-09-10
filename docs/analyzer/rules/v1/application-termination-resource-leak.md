# `application-termination-resource-leak`

A successfully acquired resource is lost on a visible termination path.

- Rule version: `v1`
- Category: `application`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`, `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `bitmap-owned-handle`, `audio-close-semantics`, `sprite-close-semantics`, `font-close-semantics`, `file-close-semantics`, `video-close-semantics`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; cleanup and error handling require application-specific ownership decisions. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore application-termination-resource-leak -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
