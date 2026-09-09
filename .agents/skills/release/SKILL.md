---
name: release
description: Audit, prepare, document, or explicitly publish gopdsdk releases and post-tag verification.
---

# Release

Follow `AGENTS.md` and `RELEASING.md`.

## Workflow

1. Resolve version/scope from `docs/ROADMAP.md`; inspect status, previous-tag
   diff, tags, and CI for the exact commit.
2. Run `RELEASING.md` gates with workspace Go caches. Keep Simulator,
   device-build, USB, and physical evidence distinct.
3. Synchronize routed docs plus `MIGRATING.md`/`RELEASING.md` when applicable;
   recheck the final diff and evidence claims.

## Publishing boundary

Commits, tags, pushes, pull requests, and hosted releases each require explicit
authorization.

When authorized, verify the commit, create/push only requested refs, verify the
hosted release, then run clean post-tag proxy and external-consumer checks without
`replace`. Claim availability only after they pass.

## Handoff

Report gates, evidence, publication state, and material failures.

