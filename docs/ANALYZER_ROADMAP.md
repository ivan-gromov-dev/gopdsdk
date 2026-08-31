# Static analyzer roadmap

Status: implementation started at Step 0. Updated 2026-08-31.

This document is the canonical implementation plan for the gopdsdk static
analyzer. The product boundary remains in [ROADMAP.md](ROADMAP.md), the public
SDK contract in [API.md](../API.md), and verified compatibility evidence in
[COMPATIBILITY.md](../COMPATIBILITY.md). Completing an analyzer step does not by
itself prove a device build, Simulator load, USB deployment, frame or memory
budget, or physical-device behavior.

## Goal

Build the broadest practical version-matched analyzer for gopdsdk games without
duplicating the Go compiler, `go vet`, `staticcheck`, or `gopls`. The analyzer
must turn documented SDK and device contracts into precise diagnostics, usable
first from CI and later as live VS Code and GoLand feedback.

The implementation grows through independently useful vertical slices. A later
stage may depend on facts produced by an earlier stage, but no stage may weaken
the precision or evidence requirements of the default rule set.

## Design constraints

- Use `golang.org/x/tools/go/analysis` as the rule contract. The dependency is
  justified by its standard analyzer model and the required type, control-flow,
  SSA, and fact infrastructure.
- Keep command parsing and exit codes in `cmd/gopdsdk`. Put analyzer-owned
  orchestration, models, rules, protocols, and tests under
  `internal/features/analyzer`; extract a shared component only after a second
  real feature consumes it.
- Keep one rule implementation for CLI, CI, language-server, VS Code, and
  GoLand consumers.
- Accept `context.Context` at package loading, analysis, workspace I/O, and
  editor request boundaries. Cancellation must stop superseded editor work.
- Treat Simulator, device, and shared code as distinct targets. Simulator
  success never suppresses a device finding.
- Default diagnostics must be high precision. Heuristic ownership,
  interprocedural, and performance rules remain experimental or opt-in until
  external-game evidence establishes acceptable noise.
- Unknown program facts remain unknown. Dynamic dispatch, reflection, build
  tags, generated code, unanalyzed dependencies, and native state must not be
  converted into false proof.
- Never copy or mechanically translate third-party analyzer code. Official Go,
  TinyGo, Playdate, and gopdsdk contracts are normative for analyzer behavior.

## Planned user surfaces

The batch surface is planned around `gopdsdk check [packages]`. Exact flags are
frozen in Step 2, but the command must support:

- Simulator, device, shared, and both-target analysis;
- Go package patterns, build tags, tests, generated-source policy, and
  changed-file filtering;
- default, experimental, and deep rule profiles;
- per-rule and per-category selection and severity overrides;
- repository configuration, command-line overrides, inline suppressions, and
  an adoption baseline;
- human-readable text and a versioned structured format;
- deterministic ordering and paths relative to the analyzed module; and
- reporting independently from the severity threshold that controls exit code.

The structured record must carry the schema, analyzer and SDK versions, rule
identifier, category, severity, confidence, target, message, primary and
related source ranges, documentation reference, suppression state, and zero or
more safe edits.

The editor surface later adds diagnostics, code actions, rule help, refresh,
progress, and cancellation through a thin language-server facade. Integrated
build, run, logs, and debugging are separate IDE-tooling scopes.

## Diagnostic policy

Use stable rule families and never recycle an identifier for different
semantics. Each rule page must contain its normative contract, target, default
severity, confidence, bad and corrected examples, limitations, suppressibility,
and safe-fix policy.

- `error`: the selected target contract is statically proven impossible.
- `warning`: a likely correctness, ownership, lifecycle, portability, or bound
  defect still depends on runtime flow.
- `performance`: structural hot-path or memory risk requiring measurement.
- `information`: migration, redundancy, or a safer documented SDK idiom.

Inline suppressions must name a stable rule and include a reason. Repository
configuration and baseline entries must not silently hide unknown rules,
malformed directives, expired locations, or configuration conflicts. Generated
source may be diagnosed according to policy but is never edited automatically.

## Development plan

### Step 0 — Freeze the contract inventory

In progress: `internal/features/analyzer` now defines the versioned contract
inventory and an immutable rule catalog with deterministic stable and
experimental selection. It validates identifier families and normative
document anchors, checks referenced exported declarations against the source
tree, and supplies type-checking external positive/negative fixture packages
spanning every planned contract shape. Expanding and reviewing the inventory
against the complete public API and official SDK sources remains to complete
this step.

Create the source-of-truth inventory from `API.md`, exported Go declarations,
official SDK documentation, the device profile, and existing examples.

Deliverables:

- a rule registry template and proposed identifier families;
- a machine-readable contract table for supported replacements, optional
  capabilities, callback-scoped values, owned and borrowed handles, retention
  edges, close semantics, update-only operations, argument bounds, and target
  availability;
- a corpus of minimal compliant and noncompliant external game packages; and
- an explicit list of facts that require runtime measurement rather than
  static claims.

Verification:

- every planned default rule points to a normative contract;
- every inventory entry has at least one intended positive and negative case;
- public API snapshot changes can be compared against the inventory; and
- undocumented assumptions are rejected or first documented in `API.md`.

Exit criterion: the initial rule catalog can be reviewed without reading rule
implementation code, and no rule depends solely on observed third-party
behavior.

### Step 1 — Build the analyzer kernel

Introduce the analyzer feature without reporting SDK diagnostics yet.

Deliverables:

- package loading with explicit module root, package patterns, build tags,
  tests, target, and context cancellation;
- normalized file and source-position models;
- a registry that runs selected `go/analysis.Analyzer` values and collects
  deterministic results;
- target and source-role classification for production, test, example,
  generated, vendored, dependency, host-only, Simulator, device, and shared
  files;
- syntax, type, control-flow, SSA, and fact-provider boundaries that rules can
  request without rebuilding the world independently; and
- kernel unit tests for partial packages, syntax/type errors, overlays,
  build-tag changes, cancellation, and deterministic parallel execution.

Verification:

- unit tests on Windows path forms and platform-independent fixture paths;
- race-enabled tests for registry and cancellation when available; and
- a no-rule external module run that loads the same packages deterministically.

Exit criterion: an empty analyzer run is cancellable, deterministic, target
aware, and does not mutate the analyzed workspace.

### Step 2 — Freeze CLI, protocol, configuration, and suppressions

Make the analyzer usable as stable infrastructure before adding a large rule
set.

Deliverables:

- `gopdsdk check` command routing with text and structured output;
- versioned structured schema and forward-compatibility rules;
- documented exit codes for success, findings, invalid configuration, package
  load failure, internal analyzer failure, and cancellation;
- repository configuration precedence and validation;
- inline suppression grammar, baseline format, and stale-baseline detection;
- deterministic path, message, related-range, and edit serialization; and
- a synthetic rule used only to exercise every protocol field.

Verification:

- golden text and structured-output tests;
- external-consumer CLI tests for clean, finding, malformed configuration,
  load-error, and cancellation cases;
- schema round-trip and unknown-field compatibility tests; and
- identical platform-independent records on Windows, macOS, and Linux native
  CI.

Exit criterion: editor and CI consumers can integrate against the protocol
without depending on human-readable output.

### Step 3 — Ship the first useful device-profile rule pack

Implement fast, high-confidence syntax, type, and import rules.

Rule scope:

- `go` statements, channels, channel operations and types, and `select`;
- `recover`, finalizers, application cgo, panic-dependent cleanup, and
  unsupported runtime-control hooks;
- unsupported clocks, sleep, timers, and tickers while permitting documented
  pure `time.Duration` operations;
- `fmt` and `encoding/json` with gopdsdk replacements;
- reflection outside the audited device subset;
- known-incompatible imports, symbols, assembly, compiler directives, build
  constraints, and generic instantiations; and
- the shortest known import or call path that introduces a forbidden reachable
  device feature.

Verification:

- positive, negative, build-tag, test, generated-source, suppression, and both-
  target fixtures for every rule;
- no default findings on maintained repository examples;
- external-consumer CLI coverage on all native CI hosts; and
- comparison against real device linker rejections for selected fixtures,
  labeled device-build rather than hardware evidence.

Exit criterion — **CLI MVP**: an external game can add `gopdsdk check` to CI and
receive actionable device-compatibility diagnostics with stable identifiers.

### Step 4 — Add application-shape and lifecycle rules

Teach the analyzer the gopdsdk game entry and callback model.

Rule scope:

- required entry package and `New() playdate.Game` signature;
- `Game.Init`, `Game.Update`, and optional lifecycle-interface shape;
- lifecycle logic that cannot receive the event it expects;
- statically provable escape of callback-scoped `playdate.Context` or transient
  views;
- `Scheduler.Update` and other update-boundary-only operations outside valid
  update paths;
- forbidden nested scheduler or stencil execution and sprite close from an
  active sprite callback when locally provable; and
- definitely live owned resources or registered callbacks on visible
  termination paths, recognizing documented aggregate cleanup.

Verification:

- interface, pointer/value receiver, embedding, generic, and factory-return
  fixtures;
- direct and bounded-indirect callback path tests;
- negative tests for incomplete interface dispatch and unknown calls; and
- deterministic tests through generated application entry wiring where
  relevant.

Exit criterion: proven application and lifecycle violations are default errors;
incomplete call-path conclusions remain warnings or silent.

### Step 5 — Add optional-capability and availability facts

Model the optional slices exposed through `playdate.Context`.

Rule scope:

- unchecked type assertions and impossible direct assumptions;
- comma-ok checks, exhaustive type switches, guarded helpers, early returns,
  required-capability helpers, and branch fact propagation;
- impossible, redundant, and contradictory capability checks;
- API availability by gopdsdk version, configured official SDK, Simulator,
  device profile, and declared compatibility floor; and
- supported fallback or feature-gating suggestions.

Verification:

- branch, loop, closure, helper, interface, and wrapper fixtures;
- fact invalidation after reassignment or unknown calls;
- multi-version compatibility fixtures; and
- zero panicking quick fixes for single-value assertions.

Exit criterion: local capability misuse is precise by default; interprocedural
capability inference remains bounded and reports its confidence.

### Step 6 — Track borrowed data and callback lifetimes

Introduce lifetime labels and escape checks for transient native data.

Rule scope:

- `Framebuffer`, `BitmapData`, `MicrophoneSamples`, PCM render buffers,
  generator buffers, and slices derived from them;
- returns, field or global assignment, container insertion, closure capture,
  goroutine escape, interface boxing, and helper-call escape;
- explicit copy operations versus slice-header copies and conversions;
- required callbacks versus documented nil-as-clear callbacks;
- owners closed before retained callback delivery or clearing; and
- statically derivable callback-registry and queue bounds.

Verification:

- intraprocedural flow-sensitive escape fixtures;
- callback nesting, branch merge, shadowing, alias, subslice, append, copy, and
  closure fixtures;
- unknown-call and reflection cases that terminate proof; and
- no claim that runtime expiry was observed when only static escape was found.

Exit criterion: definite callback-scope escapes are default errors; uncertain
escapes remain experimental warnings.

### Step 7 — Add error, result, and value-contract rules

Convert public sentinel, typed diagnostic, result, and argument contracts into
flow-sensitive checks.

Rule scope:

- discarded or overwritten SDK errors and cleanup errors lost when the primary
  operation succeeded;
- `errors.Is` and `errors.As` requirements for wrapped sentinels and typed
  diagnostics;
- discarded booleans, counts, overflow values, refresh decisions, partial
  results, and other contract-significant results;
- constant and simple range facts for dimensions, queue sizes, indices,
  volumes, rates, frame limits, paths, identifiers, enum values, and bounded
  configuration; and
- definitely nil, empty, foreign, closed, or mutually incompatible values.

Verification:

- named returns, joins, wrapping, multi-result assignment, blank identifiers,
  defer cleanup, branch ranges, constants, conversions, and generic helpers;
- no duplicate reports from general Go analyzers;
- exact sentinel, accepted range, and result contract in diagnostics; and
- fixes only where evaluation order and error behavior are unchanged.

Exit criterion: the rule pack finds SDK-specific mistakes that ordinary
`errcheck`-style analysis cannot express without producing generic noise.

### Step 8 — Build intraprocedural ownership and retention analysis

Model owned, borrowed, wrapper, aggregate, closed, and unknown handle states
within one function.

Rule scope:

- successful constructors, loads, copies, files, fonts, bitmaps and tables,
  sprites and tilemaps, video, microphone, and audio graph handles;
- definite leaks, double close, use after close, borrowed close, and ignored
  cleanup failure;
- local retention edges for menu images, masks, tables and frames, tilemaps,
  sprites and images, video contexts, samples and players, audio graph nodes,
  instruments, sequences, and callbacks;
- invalid close order with the retaining operation as related information; and
- explicit detach, clear, replace, remove, stop, and aggregate-close operations
  that discharge obligations.

Verification:

- branch, loop, early-return, defer, alias, reassignment, tuple-return, and
  aggregate-owner fixtures;
- ownership tables cross-checked against every exported closeable interface;
- no suggested `defer Close` where panic, callback, or hot-path semantics make
  it misleading; and
- runtime unit tests remain the authority for native state transitions.

Exit criterion: all locally provable ownership violations are covered, and
unknown escapes do not become default leak warnings.

### Step 9 — Add bounded interprocedural ownership summaries

Extend ownership, lifetime, capability, and callback facts across helpers
without claiming complete whole-program proof.

Deliverables:

- summaries for ownership transfer, borrowing, closing, retention, callback
  registration, capability guards, and transient-data escape;
- bounded propagation through direct calls, method calls with known receiver
  types, small interface target sets, and generic instantiations;
- recursion and strongly connected component handling;
- invalidation at reflection, unknown interface targets, globals, external
  dependencies, native calls, or configured analysis budgets; and
- related call paths explaining where an obligation was created or lost.

Verification:

- multi-package fixtures and summary cache invalidation;
- recursive, mutually recursive, interface, generic, and dependency-boundary
  cases;
- explicit confidence downgrade when analysis hits a budget; and
- a measured false-positive corpus before any interprocedural heuristic becomes
  default.

Exit criterion: deep mode adds useful cross-function findings while default
mode retains predictable latency and precision.

### Step 10 — Add frame-loop, audio, and memory-risk analysis

Build an opt-in hot-path map rooted at `Game.Update`, sprite callbacks,
frame-delivered completion callbacks, and audio render callbacks.

Rule scope:

- statically visible allocations, string construction, reflection, dynamic
  `defer` in loops, repeated resource loads, filesystem or network I/O,
  explicit GC, recursion, and unbounded slice or map growth;
- repeated work reachable from frame roots versus one-time initialization;
- fixed-capacity pools, bounded queues, reused buffers, and proven amortized
  operations;
- stricter audio callback checks for blocking, allocation, retained buffers,
  game-code re-entry, and work without a derivable bound; and
- project-declared budgets that recommend runtime probes or
  `playdate/diagnostics` measurements.

Verification:

- hot-path root and call-path fixtures;
- benchmark and allocation fixtures validating only the analyzer's structural
  classification;
- external games used to measure precision and analysis cost; and
- diagnostic wording that never claims an exact frame time, heap, stack, GC,
  underrun, or power result.

Exit criterion: performance findings remain a separate severity and require
measurement before they affect release readiness.

### Step 11 — Add workspace, manifest, asset, and migration checks

Extend the feature beyond Go AST where deterministic workspace metadata can be
validated statically.

Rule scope:

- `go.mod`, gopdsdk version, module and package layout, entry package, build
  tags, target configuration, SDK path intent, and host/device source-set
  collisions;
- required `pdxinfo` fields, identifiers, versions, conflicting metadata,
  unsafe paths, and build/release inconsistencies;
- statically known asset paths, missing packaged resources, cross-platform case
  mismatch, path escape, duplicate case-folded names, excluded resources, and
  supported source/output forms;
- inspectable image, table, font, audio, video, localization, and data metadata;
  and
- deprecated, renamed, or stale gopdsdk API use with mechanically safe
  migrations.

Verification:

- Windows, macOS, and Linux path and case fixtures;
- malformed and adversarial manifest/path fixtures;
- official-format samples where redistribution and deterministic inspection are
  permitted; and
- SDK conversion and runtime loading explicitly left to SDK integration tests.

Exit criterion — **stable analyzer candidate**: source, workspace, and asset
rules cover the documented static contract with CI-ready defaults.

### Step 12 — Complete documentation and safe fixes

Audit the complete diagnostic experience before declaring a stable analyzer.

Deliverables:

- one rule page per stable diagnostic;
- safe single-file and workspace edits with conflict detection;
- previewable fixes, fix-all grouping, idempotence, and formatting integration;
- links from diagnostics to the exact rule version; and
- migration and suppression guidance for external games.

Verification:

- every fix parses, type-checks, preserves target selection, and is idempotent;
- golden before/after fixtures cover comments, formatting, aliases, build tags,
  and evaluation order;
- generated files and uncertain semantic rewrites never receive edits; and
- documentation links and examples are checked for staleness.

Exit criterion — **stable CLI analyzer**: all default rules are documented,
tested, suppressible, deterministic, and usable by external CI.

### Step 13 — Add incremental workspace analysis

Prepare the stable engine for interactive use without changing rule semantics.

Deliverables:

- overlay-aware package snapshots and dependency invalidation;
- caches for parse, type, SSA, facts, summaries, contracts, manifests, and
  assets with bounded memory;
- fast-edit, save, and explicit deep-analysis schedules;
- request cancellation, stale-result rejection, progress, and partial failure;
  and
- instrumentation for cold time, incremental latency, peak memory, cache hit
  rate, invalidation size, and cancellation delay.

Verification:

- edit sequences covering syntax errors, renames, build tags, dependency
  changes, configuration changes, and file creation/deletion;
- cache equivalence against clean batch analysis;
- cancellation and concurrency stress tests;
- measured budgets on small, medium, and commercially realistic games; and
- no live deep analysis by default until latency and memory budgets are met.

Exit criterion: incremental and clean batch analysis produce equivalent active
diagnostics for the same snapshot.

### Step 14 — Add the language-server facade

Expose incremental analysis to editor clients while leaving `gopls` responsible
for general Go language features.

Deliverables:

- initialize and workspace configuration negotiation;
- publish and pull diagnostics as supported by the client;
- code actions, rule help, related locations, progress, refresh, and
  cancellation;
- multi-root workspace and target selection behavior;
- structured logs without source or environment secret leakage; and
- protocol compatibility tests independent of any IDE UI.

Verification:

- scripted LSP sessions and malformed-client requests;
- diagnostic clearing, stale-version rejection, cancellation, restart, and
  configuration-change cases;
- parity with batch output for equivalent snapshots; and
- bounded failure when `gopls` or another language server is active alongside
  the analyzer.

Exit criterion — **editor API**: any LSP-capable client can consume analyzer
diagnostics and safe actions without IDE-specific rule logic.

### Step 15 — Validate VS Code and GoLand consumers

Use thin editor integrations to validate the language-server contract. The
extensions themselves may live in separate repositories and release cycles.

Deliverables:

- installation, executable discovery, version compatibility, target and rule
  configuration, diagnostics, rule help, and safe fixes in both IDEs;
- graceful behavior when the workspace uses a different gopdsdk version;
- logs and restart commands suitable for troubleshooting; and
- no duplicated analysis rules in TypeScript or Kotlin.

Verification:

- automated extension tests where each IDE permits them;
- manual smoke checks on Windows, macOS, and Linux labeled as editor
  integration, not SDK or hardware evidence;
- large-workspace responsiveness, cancellation, and upgrade/downgrade checks;
  and
- equivalent rule identifiers and fixes in CLI, VS Code, and GoLand.

Exit criterion — **live IDE diagnostics**: both clients consume the same stable
engine and differ only in editor presentation and lifecycle wiring.

### Step 16 — External precision hardening

Run the analyzer against commercially realistic games before broadening default
analysis.

Deliverables:

- anonymized or redistributable precision fixtures derived from confirmed
  findings;
- per-rule false-positive and false-negative regression tracking;
- measured batch and incremental resource use;
- default-profile changes justified by evidence; and
- retirement or redesign of noisy experimental rules.

Verification:

- each promoted rule has demonstrated value outside repository examples;
- heuristic findings are never promoted directly to `error`;
- compatibility review covers identifier, severity, configuration, baseline,
  protocol, and fix changes; and
- release notes name evidence and skipped runtime gates accurately.

Exit criterion: the analyzer is suitable for default use on real games without
requiring broad suppressions.

## Rule-family completion matrix

| Family | First implementation | Default eligibility |
| --- | --- | --- |
| Device language and runtime | Step 3 | Proven target violation |
| Application and lifecycle | Step 4 | Proven callback or entry violation |
| Optional capabilities | Step 5 | Local or bounded proven flow |
| Borrowed callback data | Step 6 | Definite escape only |
| Errors, results, and bounds | Step 7 | SDK-specific contract only |
| Local ownership and retention | Step 8 | Definite state transition |
| Interprocedural ownership | Step 9 | After external precision evidence |
| Frame and memory risk | Step 10 | Performance severity, measured tuning |
| Workspace, manifest, and assets | Step 11 | Deterministic static validation |
| Safe fixes | Step 12 | Semantics-preserving and idempotent |
| Incremental and LSP delivery | Steps 13–14 | Batch-equivalent diagnostics |
| VS Code and GoLand | Step 15 | Same engine and rule identities |

## Milestones

- **CLI MVP after Step 3:** high-confidence device compatibility in local and
  CI runs.
- **SDK-aware alpha after Step 8:** lifecycle, capabilities, borrowed data,
  results, and local ownership.
- **Deep-analysis beta after Step 10:** bounded interprocedural and hot-path
  findings with explicit confidence.
- **Stable CLI candidate after Step 12:** complete documented static scope,
  workspace validation, and safe fixes.
- **Editor API after Step 14:** incremental batch-equivalent LSP diagnostics.
- **Live IDE diagnostics after Step 15:** thin VS Code and GoLand consumers.
- **Default-ready analyzer after Step 16:** external-game precision evidence.

## Global acceptance gates

Every default rule requires positive, negative, target, build-tag, test,
generated-source, suppression, and baseline fixtures; stable text and structured
output; documented source contracts; and a supported correction. Maintained
examples must have no unexplained default findings.

Native CI must cover Windows, macOS, and Linux with identical rule identities
and source positions for platform-independent fixtures. Platform path behavior
must be tested separately. Race, fuzz, adversarial input, cancellation, cache
invalidation, and external-consumer CLI coverage are required in proportion to
the stage being released.

Before live deep analysis becomes a default editor behavior, record cold
workspace time, incremental latency, peak memory, cache behavior, and
cancellation delay on representative games. These are analyzer performance
measurements, not game runtime evidence.

## Static-analysis limits

The analyzer cannot generally prove exact allocation counts, heap or stack
bounds, native handle state, callback delivery, dynamic reflection targets,
arbitrary interface targets, data-dependent loop bounds, SDK conversion
success, Simulator loading, USB behavior, or physical-device correctness.

When proof ends at an unknown call, native boundary, reflection, unresolved
build selection, or configured budget, the result must be silent or explicitly
lower confidence. The analyzer may recommend `gopdsdk build`, a probe, a
Simulator run, runtime diagnostics, or hardware measurement, but must never
present that recommendation as completed evidence. Source breakpoints and
runtime debugging remain outside this roadmap.
