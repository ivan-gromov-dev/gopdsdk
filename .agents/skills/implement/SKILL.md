---
name: implement
description: Implement scoped gopdsdk Go changes and tests under its architecture and portability contract.
---

# Implement

Follow `AGENTS.md`.

## Workflow

1. Inspect status, affected contracts, callers, and tests.
2. State the feature/package boundary; implement the smallest complete behavior
   and error paths.
3. Add deterministic tests. Abstract I/O/processes only at boundaries.
4. Verify proportionally; inspect the final diff and evidence claims.

Do not add dependencies, generated bindings, public API, or shared packages
speculatively.

## Checks

Use workspace `.cache` for Go caches. Run applicable checks and report failures
or skipped capabilities:

```powershell
gofmt -w cmd internal
go test ./...
go vet ./...
git diff --check
go run ./cmd/gopdsdk doctor
```

Use `$release` for release work.
