---
name: review
description: Review gopdsdk Go changes for correctness and risk without fixing them.
---

# Review

Follow `AGENTS.md`. Remain read-only unless fixes are requested.

## Workflow

1. Scope from the request, status, and diff; inspect changed code and callers.
2. Confirm suspected defects with focused read-only diagnostics.
3. Check relevant contracts: behavior/errors/cleanup, exit semantics, platform
   differences, architecture, deterministic coverage, probes, provenance,
   security, and destructive behavior.
4. Report findings by severity with precise lines, execution path, and impact.
   If none, state that and list material verification gaps.

Omit style preferences and claims lacking a concrete path or violated invariant.
