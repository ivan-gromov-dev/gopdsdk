---
name: update-docs
description: Synchronize gopdsdk public docs, roadmap, changelog, migrations, and evidence after implemented changes; not code or releases.
---

# Update Docs

Follow `AGENTS.md`. Change documentation only; treat verification output as the
evidence ceiling.

## Workflow

1. Inspect status, relevant diff/docs, and exact verification results.
2. Identify changed public behavior, active roadmap scope, and evidence level.
3. Follow `AGENTS.md` routing. Use `MIGRATING.md` only for required action after
   an intentional break; include exact versions/dates when known.
4. Record physical results only when user-confirmed. Name relevant unrun gates.
5. Search for stale or contradictory claims; inspect the diff and run
   `git diff --check`.

## Boundaries

Use `$release` for release declarations, release-only evidence, or publishing.

## Handoff

Report changed docs, evidence level, and unrun gates.
