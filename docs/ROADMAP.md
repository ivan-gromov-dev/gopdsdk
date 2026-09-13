# Product roadmap

Status: `v1.1.0` is prepared with the static-analyzer CLI and editor API;
post-1.1 IDE tooling contracts are implemented, updated 2026-09-13. Windows
unit and external-consumer CLI evidence is complete; remaining platform and
hardware evidence is listed below.

This is the canonical post-1.0 planning document. Released capability and
evidence live in [COMPATIBILITY.md](../COMPATIBILITY.md) and
[CHANGELOG.md](../CHANGELOG.md); public contracts live in
[API.md](../API.md); current implementation rules live in
[AGENTS.md](../AGENTS.md).

## Released v1.0 baseline

An external Go module can create, test, build, optimize, and release an offline
Playdate game using only public gopdsdk API, without a game-owned C bridge or
imports from `internal` packages. Every official C SDK capability that can
materially enable an offline game has a public Go equivalent or a documented,
functionality-preserving replacement.

The stable contract covers exported API, ownership, errors, callback ordering
and lifetime, the device Go profile, CLI commands and flags, semantic
versioning, support, deprecation, migration, compatibility, and evidence
policies. Native CI is verified on Windows, macOS, and Linux. Windows is
additionally verified with the official SDK 3.1.1 Simulator, conservative
hard-float device builds, USB deployment, and physical Playdate acceptance.
macOS and Linux SDK, Simulator, device-build, USB, and hardware levels remain
explicitly unverified.

The device profile supports sequential Go, conservative GC, normal-return
`defer`, the documented bounded reflection subset, and pure duration/value
operations. `playdate/schedule` replaces useful device cases of goroutines,
channels, `select`, sleep, timers, and tickers. `playdate/json` replaces the
reflection-heavy standard JSON path. `fmt`, `recover`, finalizers, application
cgo, blocking concurrency, and undocumented reflection paths remain
unsupported or replaced as described in [API.md](../API.md).

This baseline does not promise unlimited game complexity or compatibility with
every Go standard-library package. Games remain bounded by Playdate hardware,
the official SDK, and the documented device profile. Symbol-for-symbol C API
coverage is not required for deprecated entry points, duplicate
representations, implementation plumbing, Lua integration, or facilities
replaced without loss of game functionality.

## Post-1.1 IDE tooling contracts

The VS Code and GoLand roadmaps both require the same additional gopdsdk
contracts for build/run workflows, project health, analyzer configuration, and
device operations. These are compatible CLI and tooling-protocol additions;
they do not change the native public `playdate` API. Console-backed Simulator
tasks and a basic `gopdsdk init` wizard may use the existing CLI, but an editor
must not parse incidental human-readable output to provide rich or stable UI.

The shared SDK work was implemented once in this order:

1. **Completed — protocol foundation and capability negotiation.** Define a versioned
   command-result envelope and, where applicable, a versioned stream of
   cancellable progress events. Add an explicit machine-readable capability
   query that reports supported commands, result/event schemas, and optional
   fields so an older gopdsdk binary degrades to a known subset. Preserve the
   existing human-readable CLI by default; structured mode must have a
   documented stdout/stderr and exit-code contract, deterministic ordering,
   forward-compatible decoding rules, and no secrets in diagnostics.
2. **Completed — read-only health and discovery.** Add versioned structured results for
   `doctor` and the Simulator, device-toolchain, and USB-connection probes.
   Records must distinguish discovery from readiness, identify each check and
   evidence level, carry typed failure categories, and expose focused
   remediation data without embedding editor-specific presentation. Executable
   discovery alone must never be reported as Simulator, device, or USB
   readiness.
3. **Completed — Simulator build and run.** Add structured final results for `build` and
   `run`, including target identity, source locations for build failures,
   artifact paths, typed failure categories, and progress stages for planning,
   compilation, packaging, launch, and cleanup where they apply. Cancellation
   must terminate owned child processes and leave artifact replacement rules
   deterministic. Validate through an external game and the official Simulator
   separately on each claimed host.
4. **Completed — device deployment and logs.** Extend the same schemas to device build,
   connection, install, launch, `crashlog`, and `errorlog` operations. Report
   build, connection, deployment, launch, and retrieval as distinct stages;
   never infer connection from tool discovery. Log retrieval remains an
   explicit user action, and its structured form must separate metadata and
   typed failures from verbatim device content. Device-build, USB, and physical
   execution evidence remain separate gates.
5. **Completed — analyzer administration.** Expose the existing version-matched rule
   registry through a versioned machine-readable catalog rather than requiring
   clients to copy rule metadata. Add deterministic baseline
   create/update/validate operations around the existing
   `gopdsdk-check-baseline/v1` contract, including stale-entry reporting and
   atomic writes. CLI, LSP, VS Code, and GoLand must continue to share rule
   identifiers, configuration semantics, and safe-fix behavior.

Cross-cutting acceptance for every phase includes unit coverage of schemas,
unknown fields and versions, path normalization, cancellation, deterministic
output, and redaction; external-consumer CLI tests using a separately built
gopdsdk binary; and native CI on Windows, macOS, and Linux. Fixture-driven
editor tests establish client integration only. SDK integration, Simulator,
device build, USB, log retrieval, and physical-device claims require their
corresponding separately named evidence.

Windows unit and external-consumer CLI acceptance covers all five phases,
including the analyzer catalog and baseline administration. The installed SDK
3.1.1 Simulator probe also passed on Windows as SDK-integration evidence.
Native CI confirmation on macOS and Linux, consistent structured device-build
acceptance, USB connection and log retrieval, and physical-device execution
remain separate open evidence gates; they are not inferred from discovery or
fixture results.

The stable protocol and command contracts belong in [API.md](../API.md) when
implemented, with unreleased behavior and evidence in
[CHANGELOG.md](../CHANGELOG.md). Stable analyzer contracts and rule references
remain in [API.md](../API.md); editor-specific UI and distribution remain in
the extension repositories.

## Future — networking and multiplayer feasibility

Networking was deliberately not a `v1.0.0` or `v1.1.0` gate. The official Playdate 3.1.1 C
API exposes permission-gated HTTP and outbound TCP connections, so online
multiplayer is technically possible, but it adds server operations, protocol
design, latency, reconnection, security, privacy, and long-lived compatibility
obligations unrelated to the offline SDK foundation.

Investigate before committing a stable public networking API:

1. An HTTP leaderboard or asynchronous turn/ghost exchange with offline queue
   and explicit network permission.
2. A small authoritative server and a two-device TCP game with sequence
   numbers, bounded messages, timeouts, reconnection, and state interpolation.
3. Physical acceptance with two Playdates across disconnect, pause, lock, and
   server-version transitions.

The likely topology is two outbound clients connected to an external server;
the current C API does not provide a general inbound listen/server socket for
hosting a peer session on one Playdate. USB serial messaging remains a
development facility, not a consumer multiplayer transport.

This is a feasibility hypothesis, not a promised feature. No public networking
API should be designed until an end-to-end prototype has measured permission
flow, callback behavior, memory, bandwidth, latency, and failure recovery
against the normative official C API and hardware.

## Roadmap rules

- Official Playdate headers, documentation, examples, and observable tools are
  normative; third-party implementations remain behavioral references only.
- Every scope grows through narrow vertical slices on both runtime adapters.
- Exported API changes update the snapshot, `API.md`, compatibility evidence,
  migrations, and release notes in the same change.
- Resource ownership is explicit and deterministic; finalizers never carry
  correctness.
- A capability is not ready because its executable or symbol was discovered.
  Evidence is labeled unit, external-consumer CLI, native CI, SDK integration,
  Simulator, device build, USB, or physical device.
- Scope may change when a real consumer or hardware measurement contradicts
  the plan; the reason and affected release boundary must be documented here.
