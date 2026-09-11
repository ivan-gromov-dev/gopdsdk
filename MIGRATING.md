# Migration guides

## Migrating to v1.1.0

`v1.1.0` is a compatible CLI expansion. It adds `gopdsdk check` for batch and
CI analysis and `gopdsdk lsp` for editor clients; it does not remove, rename,
or intentionally change the behavior of the stable `v1.0.x` native API.
Existing games can update their module requirement without source changes.

The default analyzer profile reports only high-confidence contract findings.
Experimental and deep ownership, interprocedural, and performance analysis
remain opt-in. Teams adopting the analyzer can use its versioned configuration,
inline suppressions, or baseline support to stage enforcement without changing
runtime behavior.

## Migrating to v1.0.0

`v1.0.0` freezes the public offline-game contract accumulated through
`v0.11.0` and adds bounded cooperative scheduling, the conservative-device
normal-return `defer` and reflection subsets, nested audio-channel routing,
complete LFO controls, and borrowed dry/wet channel-level signals. No released
`v0.11.0` API is intentionally removed or renamed, so existing games can update
their module requirement without source changes.

## Stable v1 contract

From `v1.0.0`, compatible additions may ship in minor releases and compatible
fixes in patch releases. Breaking documented source or behavior changes require
a new major version. Deprecated API remains for the `v1.x` line unless a
security or correctness issue makes that impossible; every deprecation names a
replacement and migration path.

The stable contract covers documented ownership, sentinel and typed errors,
callback ordering and lifetime, CLI commands and flags, and the exported
`playdate` packages. It does not expand the device Go profile: goroutines,
blocking channels and select, `recover`, finalizers, application cgo, and the
documented unsupported standard-library paths remain unavailable. Use
`playdate/schedule`, explicit ownership and error returns, and the bounded
replacement packages described in [API.md](API.md).

## New v1 facilities

- Use `playdate/schedule` for fixed-capacity frame-spanning tasks, delayed work,
  bounded queues, and deterministic multi-queue polling.
- Conservative device builds may use documented normal-return `defer`; panic
  remains a fail-stop trap and does not unwind deferred calls.
- Only the reflection operations listed in [API.md](API.md) are accepted by the
  fail-closed device link gate.
- Audio channels expose borrowed outputs plus dry/wet level signals. Retained
  wrappers expire when their owner closes and never transfer native ownership.

## System control

Capability-assert `playdate.SystemControls` only where launch arguments,
restart, menu-image ownership, auto-lock, crank-sound control, or ordered
button callbacks are needed. Applications implementing `LifecycleGame` may now
receive mirror-started and mirror-ended events. Retained menu images and system
settings are cleared or restored before termination cleanup.

## Clock and device information

Capability-assert `playdate.SystemEnvironment` for the offline Playdate epoch,
calendar conversion, the independent elapsed timer, and copied OS, language,
and PDX version information. `Input.DeltaSeconds` now uses the wrapping
monotonic millisecond clock and no longer resets the SDK elapsed timer.

## Removed experimental video streams

The experimental `VideoStream`, `Videos.NewVideoStream`, and stream-specific
errors were removed after SDK 3.1.1 acceptance showed that ordinary PDV files
produce no frames and the first stream interface method dispatch crashed under
TinyGo 0.41.1 on physical hardware. Use `Videos.LoadVideo` and an explicitly
owned `VideoPlayer` for packaged PDV playback. There is no offline replacement
for the undocumented stream container; the capability is deferred to
post-v1.0 `v1.1 networking` research.

## Published-module verification

After publication, remove any local `replace`, require
`github.com/ivan-gromov-dev/gopdsdk v1.0.0`, and
repeat the clean module-proxy check from [RELEASING.md](RELEASING.md).
