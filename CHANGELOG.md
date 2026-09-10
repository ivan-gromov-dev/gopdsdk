# Changelog

All notable release changes are documented here. Beginning with `v1.0.0`, the
documented public API and behavior follow semantic versioning. Deprecations
remain available throughout `v1.x` unless a security or correctness guarantee
requires an explicitly documented exception.

## Unreleased

- Completed analyzer Step 9 bounded interprocedural analysis on 2026-09-10.
  The `deep` profile now propagates owned and borrowed results plus parameter
  close and transfer effects through direct, known method/generic, small
  uniform interface, and exported dependency calls. Recursive summaries use an
  eight-round fixed point, opaque boundaries invalidate proofs, helper closes
  carry related call information, and budget exhaustion is an explicit likely-
  confidence information diagnostic. Existing callback escape and capability
  guard summaries complete the lifetime/capability portion. Default ownership
  behavior remains intraprocedural. Windows analyzer and full repository unit
  suites pass; no Simulator or physical-device evidence is claimed.

- Completed analyzer Step 8 intraprocedural ownership and retention analysis on
  2026-09-09. New default ownership rules cover locally proven leaks, double
  close, use after close, borrowed close, retained close, and invalid close
  order, including related retaining-call locations. Unknown transfers remain
  silent; deferred cleanup, aliases, branches, menu-image clearing, and
  aggregate table/tilemap ownership have Windows unit coverage.

- Completed analyzer Step 7 error, result, and value-contract analysis on
  2026-09-09. Six package-identity-aware default rules diagnose discarded SDK
  errors and inventoried significant results, direct wrapped-sentinel and typed
  diagnostic matching, and proven integer-range violations through constants,
  conversions, and finite branch joins. The rules do not emit automatic fixes
  or match unrelated application APIs. Windows analyzer unit coverage passes;
  no SDK, Simulator, USB, or physical-device runtime evidence is claimed.

- Completed analyzer Step 6 callback-lifetime flow on 2026-09-09. The four
  default lifetime rules now diagnose definite return, escaping-store,
  container, closure, channel, goroutine, alias, subslice, and bounded local
  helper escapes for framebuffer, bitmap-data, microphone-sample, PCM, and
  generator buffers. Explicit `copy` and nil-destination `append` operations
  break the borrowed-data flow. Four opt-in `likely` warning rules cover calls
  whose retention behavior cannot be proved locally. The Windows analyzer and
  full repository unit suites pass; no runtime expiry was observed or claimed.

- Updated analyzer infrastructure to `golang.org/x/tools v0.50.0` and
  `golang.org/x/mod v0.41.0`; the resolved graph now uses
  `golang.org/x/sync v0.23.0`. The full Windows unit suite and `go vet ./...`
  pass with Go 1.26.5.

- Completed Step 5 optional-capability and availability analysis on 2026-09-09.
  Four flow rules report
  unchecked assertions, proven failed assertions, and already known comma-ok
  results. Local guards, early returns, type switches, bounded predicate
  helpers, wrappers, and immutable captured cells carry capability facts;
  reassignment and opaque/mutable paths invalidate them. No automatic fixes
  are generated. Assertions with related checks in another function produce
  `likely` warnings instead of claiming a proven error. Lifecycle, bounded
  caller, fallback-state, and exported boolean-helper facts preserve proven
  guards without accepting opaque cross-package behavior. The versioned
  contract inventory and `capability-video-availability` rule validate declared
  gopdsdk compatibility floors and configured official SDK versions for the
  Simulator/device availability contract. Windows unit and external-consumer
  CLI fixtures cover target selection, suppression, and multi-version outcomes;
  the full repository suite and vet pass. No Simulator or hardware readiness is
  claimed.

- Added the initial Step 4 application/lifecycle pack on 2026-09-02: eleven
  default error rules cover application entry, undeliverable lifecycle methods,
  scheduler update boundaries, nested scheduler/stencil execution, active sprite
  close, transient framebuffer/bitmap/microphone/render-buffer escapes, and
  locally proven owned acquisitions lost during termination. Rules run for
  shared, Simulator, and device analysis targets. Unit fixtures and external
  consumer CLI entry checks cover receiver shapes, helpers, generic factories,
  unknown dispatch, successful cleanup, and all target selections. Windows
  targeted race tests and `go vet ./...` pass; repository examples produce no
  application/lifetime findings. The bounded local Step 4 pack also traces
  private owned fields and PCM callback owners into termination, native callback
  update boundaries, and scalar/lifecycle arguments through local helpers.
  Generated factory-expression tests execute real `NewApplication` lifecycle
  dispatch with a substituted native context and hook. Microphone aggregate
  cleanup is excluded from leak reports. General ownership/retention graphs and
  opaque aliases remain outside this local acceptance scope. No new native SDK
  or hardware acceptance is claimed.

- Completed the local implementation and acceptance matrix for the fifteen
  Step 3 device rules on 2026-09-02. The real external-consumer CLI tests cover
  positive/negative sources, generated-source and build-tag inclusion/exclusion,
  test variants, target selection, inline suppression, and baseline identity
  for every rule. Cgo-only packages are now retained under `./...` using
  temporary in-memory overlays, without a C compiler or source-file writes.
  Dependency call and initialization paths now also carry goroutine, channel,
  `select`, and `recover` violations. Uncalled closures remain excluded.
- Added opt-in `TestAnalyzerDeviceBuildAcceptance` comparisons using official
  SDK 3.1.1, TinyGo 0.41.1/LLVM 20.1.1, and Arm GCC 15.3.1 on Windows:
  the valid normal-defer/Duration fixture produced an ARM hard-float ELF and
  PDX (280,052 bytes static RAM; 1,331,404-byte ELF; 59,809-byte PDX);
  Go assembly failed ARM linking with an undefined `Fast` reference; and
  `reflect.MakeSlice` failed the linked-symbol audit. An unsupported
  `reflect.Value.Call` fixture was diagnosed by the analyzer but packaged
  successfully because TinyGo eliminated its symbol in favor of a panic trap.
  That discrepancy is covered explicitly and does not establish runtime support.
  All four acceptance cases passed on 2026-09-02; JSON reports, sources, and
  build logs are retained in the local analyzer evidence cache. The earlier
  reflection fixture initially failed its expected-rejection assertion and was
  separated into the trap and retained-symbol cases. These are SDK/device-build
  results, without Simulator execution, deployment, device-log inspection,
  hardware execution, or GC soak. Native three-platform CI remains outstanding
  for these uncommitted changes.
- Added four device source rules: `device-panic-cleanup` warns about explicit
  panic with potentially pending deferred calls; `device-go-assembly` identifies
  bodyless declarations backed by selected Go assembly; `device-compiler-directive`
  detects linkname aliases to forbidden public functions; and informational
  `device-build-constraint` identifies host-selected files excluded by fixed
  device constraints. Channel diagnostics now cover generic type arguments,
  including inferred and imported named channel types. These are bounded
  checks, not blanket prohibitions on defer, generics, or compiler directives.
- Changed the canonical module, import, documentation, release, and repository
  paths to `github.com/ivan-gromov-dev/gopdsdk` after the GitHub account rename.
- Began the first production static-analyzer device rule pack. `gopdsdk check`
  now reports the initial stable syntax, import, and typed-symbol violations for
  goroutines, channels, `select`, clock and scheduling APIs, `fmt`,
  `encoding/json`, panic recovery and finalizers, application cgo, unsupported
  reflection, and runtime-control hooks while accepting the documented
  `time.Duration` and reflection subsets. Statically resolved calls through
  dependency wrappers now carry the shortest known path to `fmt` or
  `encoding/json`, and device cgo diagnostics no longer depend on a host C
  compiler successfully loading the application package.
- Extended device dependency-call diagnostics to package variable initializers
  and package-level function literals. Parenthesized calls, explicit generic
  function instantiations, and methods on instantiated generic types now retain
  their static dependency paths to `fmt` and `encoding/json`. Windows unit
  regression coverage checks source locations, safe calls, function references,
  and unresolved interface calls.
- Added device initialization-path diagnostics at imports: dependency `init`
  functions and package variable initializers propagate their shortest known
  static paths to `fmt` and `encoding/json` through package facts, including
  blank, renamed, and dot imports. Creating or returning a closure no longer
  adds its uncalled body to a function's reachability fact; immediately invoked
  literals remain followed. On 2026-09-02, Windows unit and command-level
  regression tests, `go test ./...`, `go vet ./...`, and the device-profile CLI
  check of maintained game examples passed. Subsequent SDK/device-build
  comparisons are recorded above; physical-device acceptance was not run.
- Device reachability now follows declaration-initialized local function
  variables with a single write and no address escape, including alias chains,
  generic functions, and concrete method values. These resolved calls also
  participate in dependency function and initialization facts. Reassigned,
  addressed, global, parameter, and interface values remain unresolved.
  Windows regression coverage checks captured writes, range assignments,
  shadowed names, unused aliases, and safe calls.
- Device reachability follows invoked local closures held in immutable
  declaration-initialized variables, including alias chains, nested closures,
  and deferred calls. Closure calls contribute to dependency function and
  initialization facts, while merely creating or returning a closure does not.
  Each reached closure body is visited once per traversal. Windows regression
  coverage checks `fmt` and `encoding/json` paths, repeated nested calls,
  unused/returned closures, and reassigned or addressed closure variables.
- Extended dependency-call and initialization paths to forbidden `time`,
  reflection, finalizer, and runtime-control functions. These paths use the
  same symbol policy as direct diagnostics. Audited `time`, `reflect`, and
  `runtime` APIs form terminal boundaries so host implementation details do not
  invalidate allowed device operations. Windows regression coverage checks
  per-rule selection, wrappers, aliases, closures, dependency initialization,
  and allowed duration, reflection, and garbage-collection operations.

## v1.0.0 (2026-08-18)

- Released the stable `v1.0.0` contract. The exported `playdate` APIs,
  ownership and error identities, callback ordering and lifetime rules, CLI
  commands and flags, compatibility matrix, migration guidance, support policy,
  and deprecation policy are frozen for the `v1.x` line.
- The release passed Windows formatting, the full Go suite, vet,
  `git diff --check`, the
  external-consumer CLI tests, official SDK 3.1.1 Simulator and conservative
  device build probes, and read-only Playdate detection on COM3. GitHub Actions
  run 161 passed native jobs on Windows, macOS, and Linux plus the Linux race
  detector. The user confirmed the final combined Simulator interaction and
  physical-device regression, conservative-GC soak, bounded memory and resource
  behavior, lifecycle cleanup, and unchanged post-run `crashlog.txt` and
  `errorlog.txt` checks. Exact additional measurements were not recorded.
- From the release worktree, `examples/schedule`,
  `examples/defercleanup`, `examples/reflection`, and `examples/synthesis` all
  pass official SDK 3.1.1 Simulator builds and conservative hard-float device
  builds. Their device static-RAM/ELF/PDX measurements are respectively
  283,324/1,198,472/48,898 bytes, 284,156/1,377,284/67,994 bytes,
  282,940/1,289,892/59,495 bytes, and 285,856/1,539,220/61,460 bytes.

- Added borrowed `AudioChannel.DryLevelSignal` and `WetLevelSignal` values for
  modulation driven by the channel level before and after effects. Explicit
  wrapper close and owner-channel close detach tracked edges without freeing
  SDK-owned signal state; retained wrappers fail with `ErrAudioGraphClosed`.
  Deterministic ownership and generated-bridge tests pass, as do the official
  Windows SDK 3.1.1 Simulator build and TinyGo 0.41.1 conservative hard-float
  device build of the updated `examples/synthesis` consumer. The device artifact
  uses 285,856 bytes of static RAM and produces a 1,539,220-byte ELF and a
  61,460-byte PDX; COM3 installation and launch pass. On 2026-08-18 the user
  confirmed initialization, repeated note playback, audible dry/wet
  level-driven pan and volume modulation, curvature changes, and responsive
  behavior in the Simulator and on a physical Playdate. The physical
  conservative-GC soak, bounded memory growth, and unchanged post-run
  `crashlog.txt` and `errorlog.txt` also passed by user confirmation.
- Added nested audio-channel routing through borrowed post-effects,
  pre-volume-and-pan outputs. Outputs expire and detach from downstream routes
  when their owning channel closes; the runtime rejects direct and transitive
  cycles before native graph mutation.
- Completed the official LFO controls with distinct initial phase,
  reproducible sample-and-hold random seeds, and continuous global update. Both
  generated native adapters and the `examples/synthesis` acceptance consumer cover the
  new contracts. Deterministic runtime and generated-source tests, the official
  Windows SDK 3.1.1 Simulator build and launch, and the TinyGo 0.41.1
  conservative hard-float device build pass. The final device artifact uses
  285,376 bytes of static RAM and produces a 1,504,256-byte ELF and a
  60,456-byte PDX; COM3 installation and launch pass. On 2026-08-18 the user
  confirmed audible nested `synth bus` to `master bus` routing, note playback,
  curvature control, and responsive behavior in the Simulator and on a physical
  Playdate. The physical conservative-GC soak, bounded memory growth, and
  unchanged post-run `crashlog.txt` and `errorlog.txt` also passed by user
  confirmation. The
  similarly named envelope global-update method exists only in the official Lua
  API and is therefore outside the C SDK contract.
- Enabled the audited bounded public `reflect` subset for conservative device
  builds with a fail-closed linked-symbol allowlist. Metadata and tag access,
  `Interface`, numeric conversion, and struct, slice, and map mutation have
  deterministic fixtures; dynamic calls and construction, reflected methods
  and channels, unsupported function Type APIs, and new TinyGo reflection paths
  remain rejected.
- Added `examples/reflection`, including per-operation allocation measurement
  and a repeated update-frame memory-growth soak. Pure-Go tests, the official
  Windows SDK 3.1.1 Simulator build, and the TinyGo 0.41.1 conservative device
  build pass. The artifact uses 282,684 bytes of static RAM and produces a
  1,279,604-byte ELF and a 59,359-byte PDX. The Simulator launch and COM3
  installation and launch commands pass. On 2026-08-18 the user confirmed
  physical-device `Operations PASS`, a stable 12,544-byte aggregate allocation
  measurement, `Memory PASS`, and `Soak PASS` after 60 seconds. Unchanged
  post-run `crashlog.txt` and `errorlog.txt` were also confirmed.
- Enabled normal-return `defer` for conservative device builds while retaining
  the linked-symbol rejection for `recover` and the legacy non-collecting
  profile. Added deterministic coverage for normal return, early return, LIFO
  order, argument evaluation, named results, repeated dynamic defer, and the
  audited `time.Duration` parse/format fixture.
- Added `examples/defercleanup`, a repository-owned cleanup and repeated-defer
  soak consumer. Pure-Go tests, official Windows SDK 3.1.1 Simulator build and
  launch, and the TinyGo 0.41.1 conservative device build pass; the device
  artifact uses 283,900 bytes of static RAM and produces a 1,367,000-byte ELF
  and a 67,781-byte PDX. On 2026-08-18 a user-provided Simulator screenshot
  confirmed `PASS` for defer semantics, 1,288 cleanup calls, duration parsing
  and formatting, and the current memory bound. The screenshot preceded soak
  completion. The same conservative artifact was installed on COM3 and launched
  on a physical Playdate on 2026-08-18. The user confirmed all five physical
  `PASS` markers for defer semantics, repeated resource cleanup, duration
  parsing and formatting, bounded memory growth, and the completed 60-second
  soak. The user also confirmed that the post-run `crashlog.txt` and
  `errorlog.txt` remained unchanged.
- Added `playdate/schedule`, a stackless cooperative scheduler with fixed task
  capacity, deterministic FIFO frame ordering, cancellation-safe task IDs,
  explicit yield/completion, wrap-safe delayed/deadline/repeating work, an
  optional secondary elapsed-time guard, bounded non-blocking generic queues,
  and deterministic round-robin multi-queue polling. Scheduler updates perform
  no scheduler-owned allocation.
- Added `examples/schedule`, a repository-owned incremental-work consumer.
  Pure-Go tests, the official Windows SDK 3.1.1 Simulator build and launch, and
  the TinyGo 0.41.1 conservative device build, COM3 installation, and launch
  pass. The final device artifact uses 283,068 bytes of static RAM and produces
  a 1,188,160-byte ELF and a 48,729-byte PDX. On 2026-08-18 the user confirmed
  physical-device `PASS` markers for a two-step peak, exactly 40 task steps,
  completion in 20 frames, and equal 30-item progress across four tasks. The
  physical frame-time, bounded-memory, memory-growth, soak, and unchanged
  post-run device-log gates also passed by user confirmation; exact measurement
  values were not recorded.

## v0.11.0 (2026-08-18)

- Added `playdate/json`, a dependency-free bounded JSON replacement for the
  official C callback API. It decodes from `io.Reader` or bytes into an ordered
  reflection-free value tree, preserves duplicate names and exact number
  spelling, validates UTF-8 and surrogate pairs, reports byte/path syntax
  errors, and streams validated compact or pretty output to `io.Writer`.
- The `examples/jsoncodec` consumer passed pure-Go tests, the full unit suite,
  `go vet ./...`, the official Windows SDK 3.1.1 Simulator build and launch,
  and the conservative TinyGo 0.41.1 device link gate without forbidden runtime
  symbols. Its installed hard-float artifact uses 283,900 bytes of static RAM
  and produces a 1,279,000-byte ELF and a 62,029-byte PDX. It was installed and
  launched over COM3 on 2026-08-18; the user-confirmed physical-device output
  `JSON: Crank & Key L3 flags 2 bytes 96` covers packaged-file reading, bounded
  decoding, schema lookup, tree mutation, and fixed-buffer encoding. Simulator
  visual behavior remains unverified. The final P11 soak, bounded-memory,
  memory-growth, and unchanged post-run log checks passed by user confirmation
  on 2026-08-18.
- Audited the undocumented SDK 3.1.1 `playdate_videostream` file source. On
  2026-08-18 a direct C API diagnostic in the official Windows Simulator and
  the generated Go adapter produced the same result for the repository-owned
  valid PDV: all 8,155 bytes were read, a native video-player pointer existed,
  `getError` returned no error, but `update` drew nothing and the decoder
  reported zero frames and zero buffered frames. Ordinary PDV playback remains
  supported through `LoadVideo`.
- Removed the experimental public `VideoStream`, its errors, runtime ownership
  layer, generated native bridges, and diagnostic consumer. A conservative
  TinyGo 0.41.1 physical-device build passed the device link gate with 284,460
  bytes of static RAM, but both launches crashed at the first `VideoStream`
  interface method dispatch. The two fresh device crash records symbolize to
  `runtime.nilPanic`/`runtime.runtimePanicAt`; native `videostream->update` was
  not reached. Videostream is now post-v1.0 `v1.1 networking` research until a
  source container and protocol are documented and both adapters can execute
  a real stream safely. A valid streaming-source Simulator run, native device
  behavior, soak, memory growth, and unchanged device-log gates remain
  unverified.
- Hardened the P11.2 scoreboard callback boundary on both native adapters.
  SDK-owned results are copied during native completion and game callbacks are
  deferred through a fixed four-slot queue to the next update. One request per
  operation kind remains pending at a time; immediate failures release their
  slot, and termination cancels retained callbacks, clears queued results, and
  rejects later requests. Live configured-service Simulator and physical-device
  acceptance still requires external board configuration.
- On 2026-08-17 the P11.2 consumer and generated-adapter tests, full unit suite,
  and `go vet ./...` passed. The consumer compiled and packaged for the official
  Windows SDK 3.1.1 Simulator. Its conservative hard-float device build used
  282,420 bytes of static RAM and produced a 1,177,872-byte ELF and a
  48,645-byte PDX. User-confirmed Simulator interaction on the same date covered
  asynchronous failure delivery for board discovery and score submission with
  authentication-credential diagnostics, followed by a personal-best failure
  reporting an unregistered player. The scene remained responsive across the
  sequential requests, exercising update-boundary delivery and release of each
  pending operation slot. The same conservative artifact was then installed and
  launched over COM3; user-confirmed physical-device interaction covered board
  discovery, score submission, personal-best retrieval, and score-list
  retrieval, with the scene remaining responsive across all four paths.
  Successful configured-service responses and termination during a pending live
  request remain unverified because they require external board configuration.
  The final P11 soak, bounded-memory, memory-growth, and unchanged post-run
  `crashlog.txt` and `errorlog.txt` checks passed by user confirmation on
  2026-08-18.
- Completed P11.1 without widening the public filesystem API. `File.Seek`
  already preserves the official current-position behavior by returning
  `tell` after a successful native seek; `Seek(0, io.SeekCurrent)` therefore
  provides position reporting. Deterministic coverage now includes the
  current-position path, `tell` failure classification, and adapter-side
  copying of transient `geterr` diagnostics at the failing operation boundary.
- Completed the residual SDK 3.1.1 header reconciliation. No additional
  materially useful offline capability remains outside the public Go
  equivalents and documented intentional omissions accumulated through P1-P11.

## v0.10.0 (2026-08-17)

- Completed the P10.3 public-system audit without adding low-level runtime
  plumbing to the Go contract. System delay, allocator, instruction-cache,
  printf-style formatting/parsing, fatal-error, and raw userdata entry points
  remain intentionally omitted. Native console error reporting remains an
  adapter development facility, while measured FPS stays with the existing
  read-only `playdate.Display` capability.
- On 2026-08-17 the `v0.10.0` release checkout passed `go test ./...`,
  `go vet ./...`, `git diff --check`, green Windows/macOS/Linux native CI and the
  Linux race detector, plus Windows SDK 3.1.1 `doctor --probe`. The focused
  consumers compiled and packaged for the official Simulator and conservative
  hard-float device target. The current `systemcontrol` device artifact uses
  283,244 bytes of static RAM and produces a 1,200,176-byte ELF and a
  49,327-byte PDX; `systemenvironment` uses 294,788 bytes of static RAM and
  produces a 1,241,272-byte ELF and a 70,370-byte PDX.
- User-confirmed final Simulator and physical-device acceptance covered restart
  and changed launch arguments, menu-image and system-setting control,
  lifecycle cleanup, epoch/calendar conversion, elapsed-time reset, and copied
  system information. The current combined artifacts installed and ran on a
  physical Playdate; the required conservative-GC soak, bounded-memory check,
  and unchanged post-run `crashlog.txt` and `errorlog.txt` checks passed.

- Implemented P10.2 through the optional `playdate.SystemEnvironment`
  capability on both generated native adapters and through `NewApplication`:
  copied seconds and milliseconds since the January 1, 2000 Playdate epoch,
  epoch/calendar conversion, the independent high-resolution elapsed timer,
  and copied OS version, language, and game PDX version. Calendar conversion
  rejects invalid dates and values beyond the `uint32` epoch; weekday is
  derived output metadata. Server time remains reserved for the online scope.
- `Input.DeltaSeconds` now uses the wrapping monotonic millisecond clock instead
  of resetting the SDK's single elapsed timer every frame, allowing games to
  use `ResetElapsedTime` and `ElapsedTime` independently. The new
  `examples/systemenvironment` consumer exercises time conversion, elapsed
  reset, and device information.
- On 2026-08-17 the P10.2 pure-Go consumer, runtime, public-API snapshot,
  generated Simulator/device ABI tests, full unit suite, and `go vet ./...`
  passed. The consumer compiled and packaged with the official Windows SDK
  3.1.1. Its conservative hard-float device build used 294,788 bytes of static
  RAM and produced a 1,241,272-byte ELF and a 70,370-byte PDX. User-confirmed
  Simulator interaction covered current epoch/calendar display, exact seconds
  round-trip, elapsed-time growth and A-button reset, and copied OS, PDX, and
  English-language (`LanguageEnglish`, value 0) information. The same build
  installed over COM3, launched on a physical Playdate, and passed the same
  user-confirmed interaction matrix. Soak, memory-growth measurement, and
  final release soak, bounded-memory, and post-run log evidence is recorded
  above.
- Implemented P10.1 launch and lifecycle control through the optional
  `playdate.SystemControls` capability on both generated native adapters:
  copied launch arguments and loaded-game path, restart arguments, owned
  400×240 menu images, auto-lock control, crank-sound control, and automatic
  termination restoration. Menu images reject invalid dimensions and offsets,
  remain protected from close while retained, and are released before the game
  receives `LifecycleTerminate`.
- Added mirror-started and mirror-ended lifecycle events plus bounded button
  callbacks. The callback path preserves multiple ordered transitions between
  frame snapshots without native-to-Go re-entry, uses a fixed 64-event bridge
  queue, drops newest on bridge overflow, exposes the dropped count, and is
  disabled before termination. The new `examples/systemcontrol` consumer
  exercises launch state, restart, menu image, settings, mirror events, and
  button delivery.
- On 2026-08-13 the P10.1 pure-Go consumer, runtime, public-API snapshot, and
  generated Simulator/device ABI tests passed. The focused consumer compiled
  and packaged with the official Windows SDK 3.1.1. Its conservative hard-float
  device build used 283,228 bytes of static RAM and produced a 1,196,892-byte
  ELF and a 49,261-byte PDX. On 2026-08-17 user-confirmed Simulator interaction
  covered the menu image, auto-lock and crank-sound settings, ordered button
  delivery with zero overflow, and mirror lifecycle state. Invoking restart
  closed the Simulator application instead of demonstrating a restarted
  instance. The same build installed over COM3, launched on a physical
  Playdate, and passed the user-confirmed full interaction matrix, including
  empty initial launch arguments, restart with `p10-restarted` arguments, the
  menu image, settings, ordered button delivery with zero overflow, and mirror
  lifecycle state. Final release soak, bounded-memory, and post-run log
  evidence is recorded above.

## v0.9.0 (2026-08-12)

- Declared the offline sound release complete after user-confirmed final audible Simulator and
  physical-device acceptance for routing, headphone states, samples,
  wavetable and custom synthesis, callbacks, underruns, and lifecycle cleanup.
  The confirmed conservative-GC device run also covered the required soak,
  bounded memory growth, and unchanged post-run `crashlog.txt` and
  `errorlog.txt` on the verified Windows/Playdate SDK 3.1.1 profile.
- Fixed the generated Simulator bridge's custom-generator parameter callback
  symbol collision. The release checkout passes the official SDK 3.1.1
  `doctor --probe` c-shared Simulator build and packaging probe and the TinyGo
  hard-float device build, link, relocation, and packaging probe.

- Completed the remaining sound implementation with one-pole filtering, wavetable
  and custom-generator synthesis, eight generator parameter slots, sample/file
  rate modulation, channel pan/volume modulation, synth-envelope controls,
  signal/controller lookup, and sequence/track/note introspection on Simulator
  and device adapters. `CallbackAudio.NewPCMCallbackSource` adds four fixed
  native PCM source slots with 4,096-frame rings; `NewGeneratorSynth` adds eight
  fixed native userdata/voice slots with independent 4,096-frame rings and
  native `PDSynth.copy` semantics. Go render callbacks run only during update;
  native audio callbacks consume rings and emit silence on underrun. Unit,
  public-API snapshot, forwarding, ownership, bounded-refill, generated ABI,
  and full repository tests pass.

- Deliberately omitted raw generator userdata/function pointers and
  `setMP3StreamSource`. The bounded Go generator contracts preserve the former.
  SDK 3.1.1 declares the latter in its C header but supplies no official
  documentation or example establishing shipping-device behavior, so header
  discovery alone is not treated as device readiness; packaged MP3 playback
  remains available through `FilePlayer`.

- Added focused `examples/callbackpcm` and `examples/generatorsynth` acceptance
  scenes without extending the already dense `examples/audio`. On 2026-08-11
  both passed audible acceptance in the official Windows SDK 3.1.1 Simulator,
  then conservative-GC hard-float device builds, COM3 installation, launch, and
  audible acceptance on a physical Playdate. The accepted `callbackpcm` artifact
  uses 352,340 bytes of static RAM and produces a 1,376,984-byte ELF and a
  57,677-byte PDX; `generatorsynth` uses 416,012 bytes of static RAM and produces
  a 1,463,812-byte ELF and a 61,781-byte PDX. Device acceptance covered stereo
  routing, deliberate PCM underrun recovery, direct and copied generator voices,
  1-based custom parameters, distinct triangle/square timbres, and stable audio
  with 4,096-frame rings. Final release soak, bounded-memory, lifecycle, and
  unchanged-log evidence is recorded above.

- Added synth-owned envelope curvature, velocity sensitivity, and
  note-range rate scaling on both native adapters, plus the isolated
  `examples/synthesis` audible acceptance scene. On 2026-08-11 the official
  Windows SDK 3.1.1 Simulator passed audible comparison of `-1.0` and `+1.0`
  curvature with a long attack/decay. The user then confirmed the same behavior
  on a physical Playdate after conservative hard-float build, USB installation
  through COM3, and launch. The accepted device artifact uses 280,776 bytes of
  static RAM and produces a 1,211,620-byte ELF and a 42,434-byte PDX. Final
  Release soak, bounded-memory, lifecycle, and unchanged-log evidence is recorded
  above.

- Implemented owned `AudioSample` buffers, packaged and caller-data
  loading, in-place reload, borrowed data inspection/copying, length and
  decompression, sample attachment and frame ranges, sample/file loop
  callbacks, and file-player reload, buffering, loop ranges, and underrun
  status/control. Caller data is synchronously copied into native-owned memory;
  borrowed views expire with their sample, and attached samples reject close.
  The new isolated `examples/samples` game generates and plays its own PCM
  without extending `examples/audio`. Unit, public-API snapshot, runtime, and
  generated Simulator/device adapter tests pass. On 2026-08-09 the scene built
  with the official Windows SDK 3.1.1. Its conservative-GC hard-float device
  build uses 282,280 bytes of static RAM and produces a 1,346,904-byte ELF and
  a 53,617-byte PDX. On 2026-08-09 the user confirmed visible initialization,
  audible range playback with A, and stop with B in the official Windows
  Simulator. Final loop-callback, physical-device, soak, bounded-memory, and
  unchanged-log evidence is recorded in the release acceptance summary above.

- Added the optional `AudioOutputs` capability on Simulator and
  device native adapters. Games can access the borrowed default channel, query
  headphone and headset-microphone presence, and activate headphone and speaker
  outputs. Closing the default-channel wrapper cannot remove or free the native
  default channel. Deterministic public-API, runtime forwarding, ownership, and
  generated-adapter tests pass. On 2026-08-09 the `examples/audio` scene built
  with the official Windows SDK 3.1.1 and visibly initialized in Simulator;
  it reported no connected headphone or headset microphone, accepted enabling
  both outputs, and kept the existing music graph playing. The conservative-GC
  hard-float device build uses 286,860 bytes of static RAM and produces a
  1,823,280-byte ELF and a 174,806-byte PDX. Installation and launch through
  COM3 then passed on a physical Playdate. On 2026-08-09 the user confirmed
  audible routing into connected headphones and live `Output H` changes after
  insertion/removal. Final headset-microphone, simultaneous-output, soak,
  bounded-memory, and unchanged-log evidence is recorded in the release
  acceptance summary above.

## v0.8.0 (2026-08-09)

- Implemented P8.3 per-sprite procedural draw, update, and pair-specific
  collision-response callbacks on both native adapters. Callback registration
  is bounded to 64 per context, clearing and `Close` deterministically release
  slots, and lifecycle termination suppresses later delivery. The expanded
  `examples/spritepresentation` acceptance scene exercises all three callback
  kinds. Deterministic tests, the official Windows SDK 3.1.1 Simulator build,
  and the conservative hard-float device build pass; the device artifact uses
  283,104 bytes of static RAM and produces a 1,479,780-byte ELF and a
  73,656-byte PDX. On 2026-08-09 the official Windows SDK 3.1.1 Simulator
  displayed `P8.3 PASS`; the startup collision-response self-check succeeded
  and the visible update-callback counter increased continuously. USB
  installation through COM3 and launch then passed on a physical Playdate;
  user-confirmed execution showed matching `P8.3 PASS`, a continuously
  increasing update counter, and the procedural draw. A user-confirmed
  60-second conservative-GC physical-device soak passed on 2026-08-09. The
  user also confirmed bounded memory growth and unchanged post-run
  `crashlog.txt` and `errorlog.txt` for the accepted device run.

- Completed P8.2 with line and detailed sprite-hit queries, non-mutating
  collision checks, sprite count, bulk add/remove, remove-all, and
  collision-world reset across Simulator and device adapters. Batch operations
  validate every sprite before mutation, preserve input order, and synchronize
  owned wrapper state after native remove-all operations. On 2026-08-09 the
  expanded `examples/spritepresentation` startup self-check and visual PASS
  status succeeded in the official Windows SDK 3.1.1 Simulator and on a
  physical Playdate. The conservative-GC hard-float device artifact uses
  282,512 bytes of static RAM and produces a 1,398,940-byte ELF and
  69,893-byte PDX. The final P8 device acceptance covers the required
  conservative-GC soak, bounded memory growth, and unchanged post-run device
  logs.

- Added sprite geometry and presentation controls for center, bounds,
  position getters, image flip, draw mode, opacity, stencil images and patterns,
  clip rectangles, draw-offset policy, and deterministic update/collision
  enable state across Simulator and device native adapters.
- Added `examples/spritepresentation`, a visual acceptance scene for the new
  P8.1 geometry, presentation, getter, and enable-state facilities. On
  2026-08-08 it passed the official Windows SDK 3.1.1 Simulator and physical
  Playdate checks for all image flips, stencil pattern/image enable and clear,
  clipping enable and clear, draw mode, opacity, draw-offset policy, and state
  transitions, native tilemap attachment, and live tile mutation. The final
  conservative-GC hard-float device artifact uses 281,880 bytes of static RAM
  and produces a 1,304,340-byte ELF and 61,792-byte PDX.
  The final P8 device acceptance covers the required conservative-GC soak,
  bounded memory growth, and unchanged post-run device logs.
- Worked around the SDK 3.1.1 sprite-stencil clear path rejecting its own null
  image by installing a fully open pattern with equivalent drawing behavior.
- Completed P8.1 with explicitly owned native sprite tilemaps, bitmap-table
  retention, validated size/tile access, sprite attach/detach/getter behavior,
  in-use close protection, and C-owned tile-index storage across Simulator and
  device adapters. The existing portable `TileMap` remains a separate
  immediate-mode layer rather than masquerading as an `LCDTileMap` handle.

## v0.7.0 (2026-08-08)

- Added P7.4 logical display width/height, nominal refresh-rate, and measured
  FPS introspection through `playdate.Display`, `NewApplication`, and both
  native adapters. The existing sprite/display acceptance scene now reports
  all four live values. Debug-framebuffer and explicit flush entry points
  remain intentionally outside the public contract.
- Passed deterministic tests and official Windows SDK 3.1.1 Simulator visual
  execution for the P7.4 acceptance scene. Built it with the conservative-GC
  hard-float profile, installed it over COM3, launched it, and confirmed
  matching logical dimensions, nominal refresh rate, measured FPS, redraw,
  and presentation behavior on a physical Playdate on 2026-08-08. The device
  artifact uses 292,016 bytes of static RAM and produces a 1,018,708-byte ELF
  and a 69,505-byte PDX. Bounded-memory, soak, and post-run device-log gates
  passed on 2026-08-08; full graphics performance regression remains
  unverified.
- Added the P7.3 text-layout and font-metrics surface on both native adapters:
  bounded rectangle drawing, character/word wrapping, alignment, tracking,
  leading, wrapping-height measurement, glyph advance and kerning, and
  font-owned borrowed glyph bitmaps. Glyph bitmaps now expire with their font.
- Kept custom fonts package-backed. The official `makeFontFromData` API consumes
  opaque `LCDFontData` and does not provide a portable ownership contract for
  Go-owned bytes across Simulator and device; packaged `.fnt` loading preserves
  offline behavior without retaining Go memory in native code.
- Passed deterministic tests, vet, official SDK 3.1.1 Simulator compilation,
  and conservative hard-float device compilation for the expanded
  `examples/fontsui` P7.3 scene. The device artifact uses 278,688 bytes of
  static RAM and produces an 826,340-byte ELF and a 41,203-byte PDX; Simulator
  visual execution passed on Windows; physical-device installation, launch,
  and visual execution passed on COM3. Soak, memory-growth measurement, and
  device-log inspection passed on 2026-08-08.

- Added the P7.2 owned-bitmap data surface, copying and in-place loading,
  owned bitmap-table creation/loading, mask lifetime and collision operations,
  rotated-bitmap creation, and persistent display-buffer snapshots on both
  native adapters. Callback-scoped image and mask bytes expire deterministically;
  owned mask views remain tied to their parent bitmap, and attached masks cannot
  be closed while retained.
- Added `examples/bitmapdata`, a repository-owned P7.2 acceptance scene covering
  every new bitmap-data, table, mask, collision, rotation, and display-snapshot
  operation in one deterministic lifecycle.
- Passed deterministic tests and official Windows SDK 3.1.1 Simulator visual
  execution for the P7.2 acceptance scene. Built it with the conservative
  hard-float device profile, installed it over COM3, and confirmed visual
  execution on a physical Playdate. The artifact uses 275,580 bytes of static
  RAM and produces an 872,964-byte ELF and a 38,768-byte PDX; extended soak,
  memory-growth measurement, and post-run device-log inspection passed on
  2026-08-08.

- Added the P7.1 drawing and graphics-state surface: value-typed filled
  polygons, rounded rectangles, line-cap style, background color, and
  screen-coordinate clipping on both native adapters.
- Passed official Windows SDK 3.1.1 Simulator execution and conservative-GC
  physical-device build, COM3 installation, launch, and execution for the P7.1
  acceptance scene. The device artifact uses 274,720 bytes of static RAM and
  produces an 813,972-byte ELF and a 38,355-byte PDX. Extended soak,
  memory-growth measurement, and post-run device-log inspection passed on
  2026-08-08; isolated visual assertions for every new operation remain
  unverified.

## v0.6.0 (2026-08-08)

- Released the accepted P6.1–P6.4 graphics, media, diagnostics, and performance
  scope.
- Accepted P6.2 display presentation and sprite redraw behavior in the official
  Windows Simulator and on physical Playdate hardware, including comparative
  full-redraw and dirty-region measurements.
- Completed the P6 physical regression run, including bounded-memory checks,
  soak, and post-run device-log inspection.

- Added the allocation-free `playdate/diagnostics` P6.4 collector for bounded
  frame-time distributions, live-heap growth, and owned native-resource counts
  in external games.
- Measured the external `gopdsdkgame` consumer for 1,800 frames in the official
  Windows Simulator and on physical Playdate hardware, with stable native
  resource counts and separately reported frame, heap, and artifact metrics.
- Added the optional P6.3 PDV video capability with explicitly owned players,
  metadata, validated frame rendering, screen or owned-bitmap targets, native
  decoder errors, and Simulator/device ABI regression coverage.
- Added `examples/video` with reproducible four-second project-owned PDV and
  audio fixtures plus pause, frame stepping, and screen/offscreen target controls.
- Accepted P6.3 animated and synchronized-audio playback in the official
  Windows Simulator and on physical Playdate hardware, including pause/resume,
  looping, stepping, target switching, conservative hard-float build, COM3
  installation, and launch.

### Added

- Optional display presentation controls and sprite dirty-region/redraw
  controls, validated before forwarding through `NewApplication` and both
  native ABI contexts.
- An interactive `examples/sprites` P6.2 acceptance scene covering dirty versus
  full redraw, per-sprite partial invalidation, global dirty rectangles, all
  display presentation controls, and reset behavior.
- A read-only `gopdsdk errorlog` command alongside `crashlog`, backed by one
  cohesive device-log feature and the shared cross-platform host tool policy.
- Optional `playdate.BitmapCompositor` transformed-bitmap and callback-scoped
  stencil composition, forwarded through `NewApplication` and both native ABI
  contexts.
- Shared finite-transform, positive-scale, live-bitmap, nested-stencil, and
  tiled-stencil-width validation before native calls.
- A deterministic `examples/composition` P6.1 scene with owned offscreen source
  and screen-aligned stencil bitmaps, a transparent composition canvas,
  crank-controlled rotation, and termination cleanup.

### Changed

- Project-owned Simulator and device Go/C/assembly/target bridge sources now
  ship as package-owned `go:embed` assets while official headers, setup code,
  and linker scripts remain inputs from the selected Playdate SDK.
- Public `playdate` sentinel declarations are organized by API-area files, and
  `playdate/store` separates its operations, error domain, and binary codec.
  Exported identifiers, error identity, and import paths are unchanged.
- Device connection, log retrieval, build, and deployment features now resolve
  `pdc` and `pdutil` names through the shared cross-platform host policy.

### Compatibility

- P6.1 unit and generated-ABI tests pass. An equivalent official Lua diagnostic
  reproduced SDK 3.1.1 direct rotated draws disappearing through a stencil at
  non-cardinal angles, so the accepted path rotates into a transparent canvas
  before the stencil-clipped draw. Official Windows Simulator visual interaction
  and conservative hard-float build, USB deployment, launch, and physical
  Playdate interaction pass. The device artifact uses 275,312 bytes of static
  RAM and produces a 36,970-byte PDX. P6 regression performance,
  bounded-memory, soak, and post-run device-log checks pass on the verified
  Windows profile.

## v0.5.0 (2026-08-03)

### Added

- Optional `playdate.SamplePlayers` and explicitly owned `SamplePlayer` APIs
  with bounded repeats, forward/reverse rates, duration, and playback offset.
- Optional `playdate.VariableRatePlayer` support for streaming `FilePlayer`
  pitch and speed control without widening the base `FilePlayer` contract;
  negative streaming rates return `ErrAudioReverseUnsupported`.
- Simulator and device ABI forwarding for advanced sample and file-player rate
  operations, including forwarding through `NewApplication`.
- A P5.1 audio acceptance scene with independent sample and music rate controls,
  reverse-start positioning, and state-driven redraws.
- Optional `CompletionPlayer`, streaming `FadingPlayer`, and `AudioClock`
  capabilities with explicit callback replacement, one-shot fade completion,
  close-time release, and 44.1 kHz frame timing.
- P5.2 Simulator/device ABI forwarding and acceptance-scene controls for
  play/fade/stop, completion counters, and sampled audio time.
- Optional owned `AudioChannel`, `Synth`, `LFO`, `Envelope`, and
  `ControlSignal` APIs with explicit routing/modulation edge lifetimes.
- P5.3 forwarding through `NewApplication` and both native ABI contexts,
  including channel volume/pan, waveform synth scheduling, ADSR parameters,
  and frequency/amplitude modulation.
- A full P5.3 acceptance matrix covering routed sample/file/synth sources, all
  synth waveforms and LFO shapes, amplitude/frequency LFO modulation, envelope
  and control signals, transpose, scheduled note-off, channel volume, and pan.
- Optional owned instruments, sequence tracks, note/controller events,
  sequences, and typed filter, bit-crusher, ring-modulator, delay, and
  overdrive effects, forwarded through both native ABI contexts.
- P5.4 MIDI loading, programmatic track construction, and one-shot sequence
  completion callbacks with replacement, stop, and close-time release.
- Explicit LFO arpeggiation steps through `LFO.SetArpeggiation`; the acceptance
  arpeggiator uses the major-chord offsets `0, 4, 7, 12`.
- A bounded device-native audio completion FIFO that defers sample, fade, and
  other audio-thread delivery to the next Go update frame.
- Optional `Microphones` permission and recording APIs with a separate
  microphone error domain, explicit input-source selection, owned recorder
  lifetime, callback-scoped samples, and bounded caller-buffer copying.
- Simulator/device microphone ABI forwarding and `examples/microphone`, which
  displays permission, recording state, live peak level, delivered blocks, WAV
  persistence, and audible playback through native-owned copied PCM.
- Optional `PCMPlayers` construction that copies mono signed 16-bit caller PCM
  into an owned native sample without retaining the caller slice.

### Fixed

- Reject reverse `FilePlayer` rates before entering the native API. The official
  streaming player supports positive rates only; reverse remains available to
  PCM `SamplePlayer` assets and is unsupported for ADPCM.
- Treat `addChannel` and `removeChannel` as graph mutations rather than channel
  allocation status; channel creation fails only when `newChannel` returns nil.
- Log Simulator lifecycle/update errors and draw initialization failures instead
  of leaving a silent gray screen.
- Defer device microphone delivery through a bounded native FIFO instead of
  re-entering Go from the native audio thread.

### Compatibility

- The full Go test and vet suites pass on Windows; Simulator and conservative
  hard-float device builds pass with the official SDK.
- Audible variable-rate sample and streaming-music interaction passed in
  Windows Simulator and on a physical Playdate on 2026-08-02. The device build
  used 268,940 bytes of static RAM and produced a 125,340-byte PDX; USB install
  on COM3 and launch succeeded.
- macOS/Linux native SDK integration, extended hardware soak, memory-growth,
  lifecycle stress, and post-run crashlog inspection remain pending.
- P5.2 is unit-tested and passes official Windows Simulator and hard-float
  device builds. Its device artifact uses 269,876 bytes of static RAM and
  produces a 130,127-byte PDX. Audible sample completion, the half-second music
  fade, completion counters, and advancing audio-clock display passed in
  Windows Simulator and on a physical Playdate on 2026-08-02; installation
  through COM3 and launch succeeded. Extended soak, memory-growth measurement,
  lifecycle stress, and post-run crashlog inspection remain pending.
- P5.3 is unit-tested and passes official Windows Simulator and conservative
  hard-float device builds. Its full acceptance matrix passed audible interaction
  in Windows Simulator and on a physical Playdate on 2026-08-02. The device
  artifact uses 273,456 bytes of static RAM and produces a 146,771-byte PDX;
  installation through COM3 and launch succeeded. macOS/Linux SDK integration,
  extended soak, memory-growth measurement, lifecycle stress, and post-run
  crashlog inspection remain pending.
- P5.4 unit tests, official Windows Simulator build, and conservative hard-float
  device build pass. On 2026-08-03 its full sequence, completion-counter,
  source-routing, five-effect, waveform, and modulation matrix passed audible
  interaction in Simulator and on a physical Playdate. The accepted artifact
  uses 278,900 bytes of static RAM and produces a 168,558-byte PDX; USB install
  through COM3 and launch succeeded. Extended soak, memory-growth measurement,
  lifecycle stress, and post-run crashlog inspection remain pending.
- P5.5 unit tests and official Windows Simulator/device integration builds pass.
  On 2026-08-03 permission, changing peak/block counters, recorder stop/start,
  one-second WAV save, and audible native-owned PCM playback passed in Simulator
  and on a physical Playdate. The hard-float artifact uses 282,824 bytes of
  static RAM and produces a 50,601-byte PDX; USB install through COM3 and launch
  succeeded. Denial/revocation, long-run overflow/memory measurement, lifecycle
  stress, post-run crashlog inspection, and macOS/Linux SDK integration remain
  pending.

## v0.4.0 (2026-08-02)

### Added

- Owned Playdate filesystem operations and portable file-error categories.
- `playdate/store` bounded versioned persistence with checksums, atomic
  replacement, backup recovery, and stepwise migration.
- Owned System Menu items, localized string lookup, accelerometer, power and
  system-preference capabilities.
- Optional bounded scoreboards and debug-message capabilities.
- A P1-P4 Crank Caverns consumer with persisted settings, progress, score,
  explicit save/load checkpoints, migrations, battery status, and Launcher
  exit.

### Changed

- Tile-map draw failures retain both `ErrTileMapDraw` classification and their
  underlying cause without `errors.Join`, keeping the device runtime sequential.
- Store migration failures retain `ErrMigration` and the direct cause through
  `errors.Is` without formatting arbitrary errors in device code.
- Device validation distinguishes unsupported channel operations from
  reflectlite-retained `runtime.chanLen`/`runtime.chanCap` query helpers.

### Compatibility

- The full Go test and vet suites pass on the Windows candidate checkout.
- Crank Caverns passed Windows Simulator interaction, conservative-GC hard-
  float device build at 277,524 bytes of static RAM, USB installation on COM3,
  and the device launch command. Physical multi-session durability, soak,
  memory-growth, crashlog, native CI, and post-tag proxy checks remain pending.
- See [MIGRATING.md](MIGRATING.md) and [COMPATIBILITY.md](COMPATIBILITY.md) for
  consumer guidance and evidence limits.

## v0.3.0 - 2026-08-02

### Added

- An optional `playdate.Launcher` capability that forwards the official
  `exitToLauncher` operation through Simulator and device adapters; Playdate
  delivers normal termination cleanup before returning to the Launcher.
- A deterministic `examples/navigation` scene covering `Play`, an in-process
  return from gameplay to the main menu, and `Exit` through `Launcher`.
- Documented launcher artwork packaging through `pdxinfo.imagePath` and the
  existing `resources/` staging boundary.
- A copied-data tile layer and clamped camera with per-frame work bounded to
  visible cells, observable draw statistics, and separate static tile overlap.
- A deterministic `examples/tilemap` P3.3 acceptance scene using owned bitmap
  tiles without coupling tile geometry to sprite collision.

- Optional immediate-mode line, rectangle, ellipse, and triangle
  drawing on Simulator and device adapters.
- Value-owned solid, XOR, and 8x8 pattern paints, plus clipping, draw offsets,
  and bitmap draw modes through narrow graphics capability interfaces.
- Portable sentinel errors for invalid primitive geometry, colors, and draw
  modes.
- A deterministic `examples/primitives` acceptance scene covering every P3.1
  drawing and state operation.
- Callback-scoped zero-copy framebuffer access with explicit dirty-row
  reporting, plus drawing into explicitly owned offscreen bitmaps with drawing
  context restoration.
- A deterministic `examples/framebuffer` scene covering portable pixel layout,
  dirty-range aggregation, callback lifetime, and owned offscreen drawing.
- The external [Crank Caverns](https://github.com/ivan-gromov-dev/gopdsdkgame)
  acceptance game, currently in a private repository, integrating every
  implemented P1-P3 gameplay slice through public `gopdsdk` API.

### Changed

- `playdate` sentinel errors remain centralized in `errors.go` but are now
  classified by bitmap, graphics, framebuffer, offscreen, tile-map, animation,
  sprite, audio, and font domains instead of reusing the bitmap error type.
- P3.4 retained scene-local single ownership after its audit found no two real
  consumers sharing loading, caching, rollback, transition, and shutdown
  semantics; no speculative resource manager or reference counting was added.

### Compatibility

- The P3 navigation scene passed Windows SDK 3.1.1 Simulator Play/menu-return/
  Exit interaction. Its launcher artwork packaged successfully; conservative-GC
  hard-float build, USB deployment on COM3, and device launch passed. Matching
  Play/menu-return/Exit interaction and correct card, icon, and launch-image
  display were confirmed on the physical Playdate.
- The P3.3 scene passed Windows SDK 3.1.1 Simulator visual acceptance and a
  conservative-GC physical-device build, USB deployment, controls, jump,
  collision, camera, and matching scene execution. Soak remains unverified.
- Windows SDK 3.1.1 Simulator and physical-device visual acceptance passed for
  P3.1 with matching output. Hard-float build and USB deployment passed; soak
  and memory-growth evidence remain unverified.
- P3.2 framebuffer and offscreen contracts are unit-tested through the portable
  implementation and generated adapters. The complete Crank Caverns consumer
  exercises both capabilities; separate visual and physical-device evidence for
  `examples/framebuffer` remains unverified.
- Crank Caverns completes the integrated P3 consumer boundary and has portable
  deterministic gameplay and render-plan coverage. Fixed frame-time,
  bounded-heap, extended soak, and post-run device-log measurements for the
  complete game remain unverified.

## v0.2.0 - 2026-08-01

### Added

- Explicitly owned sprites with display-list membership, positioning,
  visibility, z-index, and deterministic cleanup.
- Collision rectangles, slide/freeze/overlap/bounce responses, resolved
  movement, and sprite point/rectangle/overlap queries.
- Owned bitmap tables, borrowed frames, and an allocation-free animation helper
  supporting delta time, fixed frames, and pause/resume.
- Explicitly owned short sound effects and streaming file players with volume,
  playback state, lifecycle pause/resume, rollback, and cleanup.
- Custom-font loading, native measurement, selected-font drawing, and a
  deterministic game-UI example.
- A snapshot test that makes every exported `playdate` API change explicit.

### Changed

- `playdate.Context` now composes sprite and audio capabilities in addition to
  the P1 system, input, and graphics capabilities. This is an intentional
  pre-v1 public API expansion.
- Simulator and device runtime adapters now expose the P2 graphics, sprite,
  collision, audio, and font slices.

### Compatibility

- The verified toolchain remains Go 1.26.5, Playdate SDK 3.1.1, TinyGo 0.41.1,
  and Arm GNU Toolchain GCC 15.3.1.
- Windows remains the only host with verified official SDK and physical-device
  execution. See [COMPATIBILITY.md](COMPATIBILITY.md) for per-feature evidence
  and the P2 acceptance work that remains unverified.

## v0.1.1

- Corrected release CI and version-aware external-consumer handling after the
  first public release.

## v0.1.0

- First public release, covering the P0 foundation and P1.0 through P1.4.
