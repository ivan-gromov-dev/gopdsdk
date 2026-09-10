# `ownership-close-order`

Owned handles are closed outside their documented dependency order.

- Rule version: `v1`
- Category: `ownership`
- Default severity: `error`
- Confidence: `proven`
- Targets: `shared`
- Suppressible: `true`

## Contract

This diagnostic enforces: `bitmap-close-semantics`, `bitmap-table-close-semantics`, `sprite-close-semantics`, `audio-close-semantics`, `font-close-semantics`, `file-close-semantics`, `video-close-semantics`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

None; ownership and cleanup order require application-specific restructuring. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore ownership-close-order -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
