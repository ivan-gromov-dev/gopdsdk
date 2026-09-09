# Repository contract

Build `gopdsdk`, an independent cross-platform Go SDK for Playdate. Treat
official SDK headers, docs, examples, and observable tools as normative. Use
pdgo only as behavioral reference; never copy or mechanically translate it.

Use `$implement` for code changes and `$review` for reviews. Preserve unrelated
edits. Do not commit or publish unless requested.

## Code

- Keep `cmd/<binary>` to parsing, wiring, and exit codes. Put owned logic and
  tests in `internal/features/<feature>`; extract to `internal/shared` only for
  a second real consumer. Do not create generic helper packages.
- Keep native public APIs and sentinel errors in feature-owned `playdate` files.
  Add subpackages only for distinct higher-level layers such as `playdate/store`.
- Optional `playdate.Context` capabilities must exist in every native ABI
  context, forward through runtime `applicationContext`, and have regression
  coverage through `NewApplication`, not only ABI tests.
- Keep owned Simulator/device bridges as package-owned `go:embed` assets with
  deterministic ABI tests. Resolve official headers, setup sources, and linker
  scripts from the installed SDK.
- Put package comments in primary implementation files. Accept `context.Context`
  at process/I/O boundaries; represent commands and paths structurally; avoid
  business-logic shell strings and global mutable configuration.
- Default to the standard library and justify dependencies. Test platform logic
  separately for Windows, macOS, and Linux. Readiness requires the relevant probe,
  not executable discovery.

## Evidence

Name the evidence level: unit, external-consumer CLI, native CI, SDK integration,
or physical device. CI, cross-compilation, dry-runs, and Docker do not prove SDK,
Simulator, USB, or hardware readiness. Inspect device logs only when requested.

## Documentation

Route product examples to `README.md`, public contracts to `API.md`, unreleased
changes/evidence to `CHANGELOG.md`, active scope to `docs/ROADMAP.md`, and only
released compatibility to `COMPATIBILITY.md`. On release, remove completed scope
from the roadmap and retain durable evidence in changelog/compatibility docs.
