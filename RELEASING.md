# Release procedure

The latest stable release is `v1.0.0`. Go module versions come from Git
tags; no separate VERSION file is maintained. The procedure below applies to
the next planned release selected from the roadmap.

## Release gates

1. Select the intended version, confirm the worktree contains only reviewed
   release changes, and confirm no local or remote tag for that version exists.
2. Run formatting, `go test ./...`, `go vet ./...`, `git diff --check`, and
   `gopdsdk doctor` with workspace-local Go caches.
3. Observe green native CI on Windows, macOS, and Linux, including the external
   consumer and Linux race detector.
4. On every host advertised at SDK-integration level, run `doctor --probe`, then
   build and launch the release's focused consumers in the official Simulator.
5. Build the focused consumers for conservative hard-float device, install
   through USB, and run the release-specific physical acceptance matrix.
   Record every skipped interaction explicitly.
6. Run the required physical regression soak and memory-growth checks. Inspect
   post-run `crashlog.txt` and `errorlog.txt` only when explicitly requested for
   that release run, and record which files were actually checked.
7. Review [API.md](API.md), [COMPATIBILITY.md](COMPATIBILITY.md),
   [MIGRATING.md](MIGRATING.md), README commands, public Go documentation, and
   [CHANGELOG.md](CHANGELOG.md). Confirm the API snapshot contains only intended
   additions and no readiness claim exceeds its evidence. Move the released
   scope's durable evidence to the changelog or compatibility matrix, then
   remove its completed row and milestone section from
   [docs/ROADMAP.md](docs/ROADMAP.md).

## Version and consumer check

Before a new tag exists, acceptance uses the intended version with a local
`replace` to the release checkout. After publication, verify from a clean
directory without a `replace`. Run this check only after the intended tag is
published; for example:

```sh
go mod init example.com/release-check
go get github.com/ivan-gromov-dev/gopdsdk@v0.9.0
go run github.com/ivan-gromov-dev/gopdsdk/cmd/gopdsdk@v0.9.0 doctor
```

Then compile and dry-run a standalone application against both targets. This
post-tag check is mandatory because a local replacement cannot prove module
proxy availability or tag contents.

Creating a commit, tag, push, or hosted release is an explicit publishing
action and is not part of routine implementation verification.

For the `v1.x` line, release notes must classify public API changes as compatible
additions, deprecations, or fixes. A deprecation names its replacement and
remains available throughout `v1.x` unless an explicitly documented security or
correctness exception applies. Source- or behavior-breaking changes to the
documented stable contract require a new major version.
