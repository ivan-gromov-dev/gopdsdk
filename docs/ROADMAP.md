# Product roadmap

Status: `v1.2.0` is prepared locally but unpublished. The completed IDE tooling,
structured command, analyzer-administration, and device-disk scope is recorded
in [CHANGELOG.md](../CHANGELOG.md) and [COMPATIBILITY.md](../COMPATIBILITY.md).
Publication and its exact-commit CI, post-tag proxy, cross-platform SDK, USB,
and physical-device gates remain separate work.

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
