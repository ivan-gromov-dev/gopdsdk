# `workspace-missing-resource`

A statically named SDK resource is absent or has mismatched case.

- Rule version: `v1`
- Category: `workspace`
- Default severity: `error`
- Confidence: `proven`
- Targets: `simulator`, `device`
- Suppressible: `true`

## Contract

This diagnostic enforces: `workspace-static-contract`. See the matching public contract in [API.md](../../../../API.md).

## Safe fix policy

Replace a literal only when the uniquely matching packaged path is unambiguous. The CLI applies only edit groups emitted by the rule; use `gopdsdk check --fix preview` before `--fix apply`.

## Suppression

Prefer correcting the contract violation. For a reviewed exception, place `//gopdsdk:ignore workspace-missing-resource -- reason` on the declaration or statement reported by the diagnostic, or adopt it through the versioned baseline. Unknown rules and missing reasons are rejected.
