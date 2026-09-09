# Public API

This document describes the stable `v1.0.0` public contract. The
contract includes advanced drawing, bitmap data and masks, text and font
metrics, display introspection, complete offline sprite and collision
facilities, offline sound, optional video, and the bounded diagnostics package.
The `v1.x` line follows semantic versioning: minor releases may add
backward-compatible API and behavior, and patch releases may make compatible
fixes. Intentional source- or behavior-breaking changes require a new major
version. Security, correctness, official-SDK, or toolchain changes that cannot
be made compatibly are documented explicitly with their migration path and
evidence rather than silently weakening an existing contract.

Public API is the exported surface of `playdate` and its documented
subpackages, plus the documented CLI commands and flags. Deprecated API remains
available throughout `v1.x` unless retaining it would violate a security or
correctness guarantee; deprecations name a replacement in Go documentation and
the changelog. Compatibility applies to documented behavior and ownership,
error identity, callback ordering and lifetime contracts, not to unspecified
implementation details or exact error strings.

Applications import the native contract from
`github.com/ivan-gromov-dev/gopdsdk/playdate`. Applications that need the optional
bounded persistence layer additionally import
`github.com/ivan-gromov-dev/gopdsdk/playdate/store`. External games collecting bounded
performance evidence import
`github.com/ivan-gromov-dev/gopdsdk/playdate/diagnostics`. Packages below `internal/`,
generated runtime bridges, CLI build plans, and example internals are not
public API.

Applications that need device-safe JSON import
`github.com/ivan-gromov-dev/gopdsdk/playdate/json`. The package replaces the official
callback JSON surface without C callbacks, userdata, reflection, `defer`, or
`recover`.

Applications that need bounded cooperative work import
`github.com/ivan-gromov-dev/gopdsdk/playdate/schedule`.

## Bounded cooperative scheduling

`schedule.New` allocates fixed task and FIFO-order storage from `Capacity` and
requires a positive `StepsPerFrame`. `Scheduler.Update` is called only from the
application update boundary. It runs each ready task at most once per call,
preserves FIFO order, and performs no scheduler-owned per-frame allocation.
Each `Step` returns `Yield`, `Complete`, `Delay`, or `Repeat`; there is no
parallelism, preemption, transparent async function, or hidden task execution
from native callbacks.

`Schedule`, `ScheduleAfter`, and `ScheduleAt` return generation-bearing task
identifiers. `Cancel` and `Pending` reject completed, cancelled, unknown, and
stale identifiers. Capacity exhaustion, nil steps, invalid delays, nested
updates, and use after `Terminate` return sentinel errors. Steps may enqueue or
cancel work; newly enqueued tasks do not run until a later update. Termination
clears retained step closures and permanently rejects new work.

Delayed, absolute-deadline, and repeating tasks use the wrapping
`CurrentTimeMilliseconds` clock. Intervals are limited to
`MaxDelayMilliseconds`, the unambiguous signed half of the `uint32` range.
Repeats remain anchored to the previous deadline. An optional elapsed-time
budget is checked between steps after at least one step runs; it is secondary
to `StepsPerFrame` and cannot prevent one oversized step from overrunning a
frame.

`Queue[T]` is a fixed-capacity sequential FIFO with explicit non-blocking
`TrySend` and `TryReceive` results. `Poller[T]` checks multiple queues with
deterministic round-robin priority beginning after the last successful queue;
when no queue is ready it returns `ready == false` and index `-1`. These types
do not claim Go channel or `select` semantics and are not safe for concurrent
use.

## Bounded JSON

`json.Decode` and `DecodeBytes` produce a reflection-free `Value` tree under
explicit document-byte, nesting-depth, node-count, and decoded-string limits.
Zero limits select conservative defaults of 64 KiB, depth 32, 4,096 nodes, and
16 KiB per decoded string. Objects preserve source order and duplicate member
names. Numbers preserve their original valid JSON spelling, avoiding implicit
precision loss. Strings validate UTF-8 and decode all JSON escapes, including
UTF-16 surrogate pairs.

`SyntaxError` reports a byte offset, JSON Pointer path, and stable diagnostic.
`Encode` writes directly to an `io.Writer`, validates programmatically-created
values, optionally pretty-prints, and bounds recursive depth without buffering
the complete output. Callers choose their schema and can use a fixed writer to
cap output allocation. Reflection-based struct tags and automatic Go-value
marshaling are intentionally outside this device profile.

## Bounded diagnostics

`playdate/diagnostics.Collector` records a fixed number of samples without
allocating in the frame loop. A game supplies elapsed frame milliseconds, live
heap bytes, and its count of currently owned native resources. The report
contains mean, p50, p95, p99, and maximum frame time; heap start, end, maximum,
and signed growth; and native-resource start, end, minimum, and maximum. Frame
times of 255 ms or more share a bounded tail bucket; the exact observed maximum
is retained. The 36,000-frame ceiling is twenty minutes at 30 FPS.

The collector deliberately does not discover runtime memory or resource
ownership and does not perform frame-loop I/O. Consumers obtain heap data from
their target runtime, count the resources they explicitly own, and write or
render the final report outside the measured interval. Device artifact static
RAM, ELF size, and packaged PDX size continue to come from `gopdsdk build
device`; those build metrics are not evidence of Simulator or hardware timing.

## Application contract

An importable application package provides:

```go
func New() playdate.Game
```

`Game.Init` runs before updates. `Game.Update` receives the common `Context` and
returns whether the display should refresh. An error from a game callback
permanently stops that application instance.

Implement `playdate.LifecycleGame` when the game needs pause, resume, lock,
unlock, terminate, low-power, mirror-started, or mirror-ended events. Events
retain platform order. Release owned native resources on
`LifecycleTerminate`; initialization must roll back resources already acquired
when a later acquisition fails.

## Application diagnostics

`gopdsdk check` enables application and callback-lifetime diagnostics for
`shared`, `simulator`, and `device`. All currently implemented rules below are
default errors; unresolved dispatch and ownership do not produce an error.

| Rule | Proven condition checked |
| --- | --- |
| `application-entry` | A package containing `pdxinfo` is `main` or lacks `func New() playdate.Game`. Libraries without `pdxinfo` do not require a factory. |
| `application-lifecycle-shape` | A concrete Game returned by `New` or a local factory helper has `HandleLifecycle` but its actual pointer/value method set does not implement `LifecycleGame`. |
| `application-scheduler-update-boundary` | A local static call path from Game `Init`, `HandleLifecycle`, or a registered native sprite/audio/microphone callback reaches `Scheduler.Update`. |
| `application-nested-stencil` | A registered stencil callback reaches another `WithStencil` through local static calls. |
| `application-nested-scheduler` | A scheduled closure calls `Update` on the same captured scheduler, with a locally stable capture. |
| `application-sprite-callback-close` | A sprite callback closes a participating sprite parameter, directly or through a local helper. |
| `application-termination-resource-leak` | A local owned acquisition or a proven non-nil private owned field is lost on a visible termination path without cleanup or transfer. |
| `lifetime-framebuffer-escape` | A registered framebuffer callback stores its view or byte slice outside the callback. |
| `lifetime-bitmap-data-escape` | A registered bitmap-data callback stores its view, image bytes, or mask bytes outside the callback. |
| `lifetime-microphone-samples-escape` | A recording callback retains its samples outside the callback. |
| `lifetime-audio-render-buffer-escape` | A PCM or generator render callback retains its output slices outside the callback. |
| `lifetime-framebuffer-possible-escape` | Experimental: framebuffer data reaches a call whose retention behavior cannot be proved. |
| `lifetime-bitmap-data-possible-escape` | Experimental: bitmap data reaches a call whose retention behavior cannot be proved. |
| `lifetime-microphone-samples-possible-escape` | Experimental: microphone samples reach a call whose retention behavior cannot be proved. |
| `lifetime-audio-render-buffer-possible-escape` | Experimental: a render buffer reaches a call whose retention behavior cannot be proved. |
| `ownership-resource-leak` | A successfully acquired local owned handle reaches a return without close or a visible transfer. |
| `ownership-double-close` | The same local owned handle is closed twice on one path. |
| `ownership-use-after-close` | A non-bitmap SDK handle is used after a locally visible close. |
| `ownership-bitmap-use-after-close` | A bitmap is used after a locally visible close. |
| `ownership-borrowed-close` | A borrowed handle, such as a bitmap-table frame, is closed by the caller. |
| `ownership-retained-close` | A menu bitmap is closed before it is cleared or replaced. |
| `ownership-close-order` | A handle is closed before a locally visible retaining owner detaches or closes it. |

Game method mismatches that already prevent Go type checking remain package-load
errors. The analyzer follows at most eight local static call edges and 4096
block visits per root, carrying scalar arguments and the loaded lifecycle-event
constants through helpers. Unknown interface dispatch and arbitrary aliases are
outside these proofs. Conditional helper blocks are omitted when argument facts
are unavailable; their unavoidable continuation remains eligible. Copies of
transient bytes into owned storage are allowed. The current public contract does
not prohibit retaining `Context` itself, so there is no Context-escape rule.

Termination analysis explores at most 256 local path states, using the loaded
SDK's `LifecycleTerminate` constant and explicit successful acquisition checks.
Paths with other unresolved conditions are omitted.
Unknown calls and deferred helpers may perform aggregate cleanup and therefore
discard ownership knowledge. A private field can establish pre-existing ownership
only when every visible non-nil write comes directly from an owned constructor,
no handle/address escapes or opaque uses invalidate it, and a termination guard
proves the active receiver's field non-nil. This includes owned PCM callback
sources, whose `Close` releases their registration. A known `Close`, transfer, or
cleanup helper suppresses this limited field proof. Microphone recording is
excluded from owned-acquisition leak checks because application termination
performs aggregate cleanup. General retained-callback graphs, public fields,
loops, and cross-package ownership remain outside these local proofs.

Local ownership analysis follows SSA control-flow paths within one function. It
recognizes successful SDK constructors, aliases, deferred cleanup, borrowed and
wrapper results, aggregate owners, and documented retain, replace, clear, and
remove operations. Unknown calls are treated as possible transfers and do not
become leak findings. Invalid close order includes related source information
pointing to the retaining operation. The analysis is deliberately silent when
constructor success, an escape, or cross-function cleanup cannot be proved.

## Error contracts

`gopdsdk check` applies SDK-specific result rules only to calls whose declaring
package is `playdate` or one of its subpackages. It does not report similarly
named application or standard-library calls.

| Rule | Default | Proven condition checked |
| --- | --- | --- |
| `result-sdk-error-discarded` | error | An error result from an SDK call is unused, assigned to `_`, or discarded by `defer`. Returning it, storing it in a named result, wrapping it, or otherwise consuming it is accepted. |
| `result-sdk-value-discarded` | warning | A documented queue, lookup, cancellation, readiness, or copy result is unused. Other SDK values are not treated as significant by type alone. |
| `result-sdk-error-comparison` | error | An SDK `Err…` sentinel is used with `==` or `!=`; wrapped SDK errors require `errors.Is`. |
| `result-sdk-typed-diagnostic` | error | An error is directly asserted to an exported SDK error type; wrapped diagnostics require `errors.As`. |
| `result-menu-image-offset` | error | `SetMenuImage` receives a constant or finite constant join outside `0..200`. |
| `result-invalid-argument` | error | A constant or finite constant join violates a modeled queue-capacity, diagnostics-frame-limit, animation-index, or animation-count range. |

Range propagation is deliberately limited to integer constants, conversions,
and phi joins whose every input is known. A partially invalid join is reported
with its proven minimum and maximum; unknown arithmetic or input ends the proof.
The rule pack emits no edits because error propagation and fallback behavior
are application-specific. Runtime handle state, path validity, floating-point
finiteness, and ownership-dependent value compatibility remain outside these
Step 7 proofs and belong to later ownership or workspace analysis.

## Capability diagnostics

`gopdsdk check` enables these rules for shared, Simulator, and device analysis:

| Rule | Default | Meaning |
| --- | --- | --- |
| `capability-unchecked-assertion` | error | A single-value assertion to an optional capability has no proven successful guard. |
| `capability-unproven-assertion` | warning, likely confidence | Another function checks this capability, but the analyzer cannot prove that check protects this assertion. |
| `capability-impossible-assertion` | error | A single-value assertion is known to fail if reached, including a failed guard, nil interface, or incompatible boxed concrete value. |
| `capability-redundant-check` | information | A comma-ok check has an already established result on this path. |
| `capability-video-availability` | error | `Videos` is used for a target or configured compatibility floor that predates its versioned contract. |

Checks recognize dominating comma-ok guards, early returns, type switches,
boolean negation/equality, compatible interface wrappers, and simple local
boolean helpers up to eight inference edges. Read-only closure captures can
inherit an enclosing guard when their captured cell has a single initialization
and no address escape. SSA value identity invalidates facts after reassignment;
unknown predicates and mutable captures do not establish a successful guard.
An exported cross-package boolean helper establishes a guard only when it returns
one unchanged parameter's direct capability check (or its negation); multiple
returns, memory loads, opaque calls, and function values publish no fact.
Helpers returning a checked capability plus an error remain
valid without another type assertion. Known checked failures should use the
application's unsupported-capability fallback. No automatic fixes are generated.
Checks in other functions, including a game's `Init`, may require lifecycle or
caller facts that are not yet available. Such related checks produce the
explicitly lower-confidence warning rather than a proven error. That warning
still meets the CLI's default warning failure threshold.

Capability types come from the loaded SDK package, including re-exporting
wrappers, rather than the analyzer binary. Missing Go API symbols remain
package-load errors. `--gopdsdk-floor vMAJOR.MINOR.PATCH` declares the oldest
gopdsdk release the game supports; `--playdate-sdk MAJOR.MINOR.PATCH` supplies
the configured official SDK version. The equivalent `.gopdsdk-check.json`
fields are `gopdsdkFloor` and `playdateSDK`. When set, the analyzer compares
uses against the contract inventory's gopdsdk introduction, minimum verified
official-SDK version, and target metadata. Omitted floors make no unsupported
inference from the analyzer binary or the host toolchain.

## Context capabilities

`playdate.Context` composes five smaller interfaces:

- `System` provides the wrapping monotonic millisecond clock.
- `InputReader` provides one immutable frame snapshot.
- `Graphics` provides clear/text and the accepted bitmap operations.
- `Sprites` creates explicitly owned sprites and runs the shared display-list
  update/draw pass.
- `Audio` loads explicitly owned short sound effects and one-file streaming
  players.

Helpers should accept the narrowest interface they need. This keeps pure game
logic independent from runtime callbacks and makes deterministic testing
straightforward.

Games that offer an explicit quit action assert the optional `Launcher`
capability and call `ExitToLauncher`. The official runtime sends
`LifecycleTerminate` before starting the Launcher, so cleanup remains in the
normal lifecycle path. An in-game title or pause menu is application state:
switching from gameplay back to that menu does not terminate or restart the
process.

Games that need launch and lifecycle settings assert the optional
`SystemControls` capability. `LaunchArguments` copies both the argument string
and loaded game path out of transient SDK storage. `RestartGame` rejects an
embedded NUL with `ErrLaunchArguments`. Auto-lock changes are restored and
crank-sound changes return and restore the prior state when the application
terminates.

`SetMenuImage` accepts only an owned 400×240 bitmap and an x offset from 0
through 200. The runtime retains that bitmap until replacement, explicit
`ClearMenuImage`, or termination; `Close` returns
`ErrBitmapMenuImageInUse` while retained. The reference is cleared before the
game receives `LifecycleTerminate`, so normal lifecycle cleanup can close the
bitmap. Size and offset failures return `ErrMenuImageSize` and
`ErrMenuImageOffset` before a native call.

Games that sample motion assert `Accelerometer`, explicitly enable it, and read
`AccelerometerXYZ`; reads before enablement return zero and the runtime disables
the peripheral after `LifecycleTerminate`. `PowerMonitor` exposes the power
flags, battery percentage, and voltage. `SystemPreferences` exposes only the
read-only system volume, reduce-flashing preference, timezone offset in seconds,
and 12/24-hour preference. These capabilities do not add setters for global
user preferences.

Games that need offline wall-clock, calendar, or runtime compatibility metadata
assert the optional `SystemEnvironment` capability. `CurrentEpochTime` returns
an owned `EpochTime` containing seconds and milliseconds since midnight,
January 1, 2000. `EpochToDateTime` returns an owned `DateTime`; weekday uses 1
for Monday through 7 for Sunday. `DateTimeToEpoch` ignores the supplied weekday,
rejects invalid dates or instants outside the representable `uint32` epoch with
`ErrDateTime`, and returns seconds in the same epoch.

`ResetElapsedTime` starts the SDK's high-resolution timer and `ElapsedTime`
reports seconds since that reset. `Input.DeltaSeconds` uses the separate
wrapping monotonic millisecond clock and does not reset this timer. `SystemInfo`
returns a copied snapshot containing the numerically encoded OS version,
localization `Language`, and PDX version declared by the game. Server time is an
online capability and is not included.

Low-level system delay, allocator, instruction-cache, printf-style
formatting/parsing, fatal-error, and raw userdata entry points are intentionally
not public: they are runtime plumbing or have direct Go equivalents. Native
console error logging remains an adapter development facility. The public FPS
surface is the existing read-only `Display.FPS` introspection rather than a
system-owned FPS drawing operation.

Games that draw immediate-mode geometry assert `PrimitiveGraphics`; games that
change clipping, draw offset, or bitmap compositing assert `GraphicsState`.
These optional capabilities keep existing graphics fakes small. Primitive
dimensions and stroke widths must be positive, and ellipse angles must be
finite. Invalid values return the corresponding graphics sentinel before a
native call. The runtime's per-frame context forwards both optional slices to
the platform adapter; an adapter without the requested slice returns
`ErrGraphicsUnavailable`.

`PrimitiveGraphics` includes filled polygons expressed as `[]GraphicsPoint`
with non-zero or even-odd fill rules, plus stroked and filled rounded
rectangles. A polygon requires at least three vertices and rounded-rectangle
radii cannot be negative. `GraphicsState` also controls butt, square, or round
line caps, the solid display background color, and clipping in absolute screen
coordinates. `SetClipRect` remains drawing-coordinate-relative;
`SetScreenClipRect` is unaffected by the current drawing offset.

Games that need direct 1-bit pixels assert `FramebufferGraphics` and use
`WithFramebuffer`. The callback receives a zero-copy view of the 400×240
working framebuffer with a 52-byte row stride. `Pixel` and `SetPixel` use
MSB-first bits; a set bit is white. `SetPixel` marks its row dirty, while code
that mutates `Bytes` must call `MarkDirtyRows`. Playdate receives the combined
inclusive dirty range when the callback returns, including when it returns an
error. The view and its byte slice must not be retained; checked operations on
the view return `ErrFramebufferExpired` after the callback.

Games that render through normal graphics operations into an owned bitmap
assert `OffscreenGraphics` and use `DrawInto`. The previous native drawing
context is restored before `DrawInto` returns, including callback errors.
Borrowed table frames are rejected with `ErrBitmapBorrowed`; callbacks and
bitmap cleanup remain explicitly owned by the game.

Games that rotate or stencil bitmaps assert `BitmapCompositor`.
`DrawRotatedBitmap` accepts a finite angle and center proportions plus positive,
finite scales. `WithStencil` borrows a live bitmap only for its synchronous
callback and clears the native stencil before returning, including callback
errors. Stencil callbacks cannot be nested. A tiled stencil must have a width
that is a multiple of 32 pixels, matching the official SDK constraint.
Playdate SDK 3.1.1 does not reliably combine an active screen stencil with a
direct rotated-bitmap draw at non-cardinal angles. Render the transform into a
transparent offscreen bitmap first, then draw that bitmap through the stencil.

`SolidPaint`, `XORPaint`, and `PatternPaint` create primitive paint values.
Patterns contain eight image rows followed by eight mask rows and are copied by
value; neither runtime adapter retains a pointer after the drawing call.
`DrawMode` mirrors the official copy, transparent, fill, XOR/NXOR, and inverted
bitmap compositing modes. Restore offsets, clipping, and draw mode after a
localized drawing pass because these values are native graphics state.

Games that need PDV playback assert the optional `Videos` capability.
`LoadVideo` returns an explicitly owned `VideoPlayer`; close it exactly once.
`Info` reports dimensions, rate, frame count, and decoder position.
`RenderFrame` validates the frame index and reports native decoder errors as
`VideoOperationError`. `SetContext` borrows a live owned bitmap without taking
ownership; keep it open until selecting the screen or another bitmap.
SDK 3.1.1 declares `playdate_videostream`, but does not document its input
container, call contract, or HTTP/TCP protocol and ships no fixture or producer.
Its `setFile` entry point does not accept an ordinary seekable PDV as an offline
replacement for `loadVideo`: direct official C API testing in the Windows
Simulator read the complete valid PDV, returned a native video-player pointer
and no decoder error, but reported zero frames and drew no output. The same
result occurred through the generated Go bridge. A conservative physical-device
probe then failed before the native update call: TinyGo 0.41.1 rejected the
returned stream's dynamic type at its first interface dispatch and entered
`runtime.nilPanic`. The experimental wrapper was removed rather than exposing a
Simulator-only contract with no usable source. Treat the complete capability as
post-v1.0 networking research until Panic documents a source format and protocol
and both native targets can execute it safely. Games should use `LoadVideo` for
packaged PDV playback. `playdate_video.getContext` remains equivalent to the Go
wrapper's retained target state.
The four-second `examples/video` consumer passed visual and audible acceptance
in the official Windows Simulator and on a physical Playdate on 2026-08-08.
This evidence covers synchronized companion audio, pause/resume, looping,
stepping, screen/offscreen targets, and the later performance,
bounded-memory, soak, and post-run device-log regression checks on the verified
Windows profile.

## Input

`Input.Buttons` is the current button set. `Pressed` and `Released` are edges
for the current frame; `Held` contains buttons that remained down across the
frame boundary. `Buttons.Has` tests whether every requested bit is set.

`CrankAngle`, `CrankDelta`, dock state/transitions, and `DeltaSeconds` use the
same model in Simulator and device builds. Game state should advance from the
snapshot rather than branching on the build target.

The optional `SystemControls.SetButtonCallback` preserves ordered native
up/down transitions, including multiple transitions of one button between two
frame snapshots. Queue sizes are limited to 1 through 64; a nil callback with
size zero disables delivery, and other invalid combinations return
`ErrButtonCallbackConfig`. Native callbacks write only to a fixed bridge queue;
Go callbacks run immediately before the next game update. The bridge drops the
newest event if its queue is full and reports the count through
`ButtonCallbackOverflow`. Termination disables the callback before delivering
`LifecycleTerminate`.

## Bitmap ownership and errors

`LoadBitmap` and `NewBitmap` return owned bitmap handles. Close each successful
handle exactly once. A failed partial initialization must close earlier handles.
Do not rely on finalizers.

An owned bitmap installed through `SystemControls.SetMenuImage` cannot be
closed while retained. Clear or replace it first, or close it from
`LifecycleTerminate` after the runtime has cleared the native menu reference.

After a successful close, bitmap operations return `ErrBitmapClosed`.
Invalid sizes, colors, and scales use the exported sentinel errors. A failed
resource load returns `BitmapLoadError`, which retains the Playdate diagnostic.
Callers should use `errors.Is` for sentinels and `errors.As` for the typed load
error rather than matching error text.

Games that need bitmap storage and mask operations assert
`BitmapDataGraphics`. `WithBitmapData` accepts only an owned bitmap and exposes
its MSB-first image and optional mask bytes for one synchronous callback;
checked access after return fails with `ErrBitmapDataExpired`. `SetPixel`
accepts black or white, updates the native image bytes immediately, and marks
the view dirty. Call `MarkDirty` after direct slice writes; `Dirty` lets an
editor propagate the change to sprites through their redraw API.

`CopyBitmap`, `RotatedBitmap`, and `CopyDisplayBuffer` return new owned handles.
The integer returned with a rotated bitmap is the native allocation size for
budget accounting. `LoadIntoBitmap` and `LoadIntoBitmapTable` require owned
destinations and preserve the handle identity. A bitmap table created with
`NewBitmapTable` owns its frames; returned frames remain borrowed from it.

`SetBitmapMask` requires equal bitmap dimensions and keeps both Go handles live
without transferring ownership. `BitmapMask` returns an owned native view tied
to the masked bitmap's storage: close the view before its parent. A mask cannot
be closed while another bitmap retains it. `ClearBitmapMask` removes the
association without closing either bitmap. Mask collision accepts only the four
`BitmapFlip` values and an integer test rectangle.

## Sprites and display list

Games that need physical-display presentation controls assert the optional
`Display` capability. Refresh rates must be finite and between 0 and 50 FPS;
scale is limited to 1, 2, 4, or 8; mosaic components are limited to 0 through
3. `Width` and `Height` report the logical dimensions after scale is applied;
`RefreshRate` reports the nominal target and `FPS` the measured actual rate.
Inversion, flipping, and display offset remain explicit presentation state.
The callback-scoped framebuffer remains available through `FramebufferGraphics`;
there is no public debug-framebuffer or explicit flush operation because the
`Game.Update` refresh result owns that presentation decision.

Games that control global redraw policy assert `SpriteRedraw` and use
`SetAlwaysRedraw` or `AddDirtyRect`. Individual sprites use `MarkDirty` or
`MarkDirtyRect`; invalid rectangles are rejected before native calls. The
display and redraw acceptance scene passed dirty/full redraw switching,
partial invalidation, display effects, logical-size changes, nominal and
measured frame-rate reporting, comparative measurements, and reset behavior
in the official Windows Simulator and on physical Playdate hardware.

`NewSprite` returns an owned sprite. Configure its bitmap, center, bounds,
position, visibility, z-index, image flip, draw mode, opacity, stencil, clip
rectangle, and draw-offset policy, then call `Add`. Update and collision passes
can be enabled independently and queried together with the useful geometry and
presentation state. `Add` and `Remove` are idempotent. Each frame,
move game objects and call `UpdateAndDrawSprites` once to update and render the
global Playdate display list.

Procedural sprites install `SetDrawCallback`, `SetUpdateCallback`, and
`SetCollisionResponseCallback` directly on an owned `Sprite`; passing `nil`
clears a callback. Draw and update callbacks run synchronously in native
display-list order, and collision callbacks synchronously select the response
for the ordered sprite pair. A context retains at most 64 installed callback
slots across the three kinds and returns `ErrSpriteCallbackLimit` before
changing native state when full. `Close` releases every slot, and lifecycle
termination suppresses subsequent delivery. Callback code must not panic and
must not close either sprite during the active native callback; mutations to
other sprite state take effect according to the official SDK's current frame
pass.

`ClearStencil` uses a fully open stencil pattern on SDK 3.1.1. This preserves
the documented drawing result while avoiding the SDK Simulator's rejection of
the null bitmap used by its native sprite-clear path. Sprite stencil images use
framebuffer coordinates rather than moving with the sprite.

Games that need official sprite tilemap attachment assert `SpriteTileMaps` and
create an owned `SpriteTileMap` from an owned `BitmapTable`, positive map
dimensions, and a complete row-major `[]uint16` of zero-based image-table
indices. The native adapter retains a C-owned copy of the index data because
the official tilemap retains that array after `setTiles` returns. `Size`,
`PixelSize`, `Tile`, and `SetTile` expose bounded native state. A sprite can
attach, clear, and query its tilemap. Close sprites or clear their tilemaps
before closing the tilemap, then close the tilemap before its bitmap table;
in-use closes return explicit errors. The portable `TileMap` remains a separate
immediate-mode layer and is never treated as an `LCDTileMap` handle.

`Close` removes an added sprite before freeing it and is always explicit; close
sprites before closing bitmaps referenced by them. If initialization fails,
close every sprite already created, followed by its bitmap. Sprite movement and
crank input use the same float32 contract in Simulator and device adapters.

Source resources live below `resources/`; that directory becomes the PDX root.
For example, load `resources/images/player.png` as `images/player`.

## Collision queries

Sprite collision rectangles opt an owned sprite into collision queries.
`MoveWithCollisions` resolves a goal position and returns the actual position
plus ordered contacts using slide, freeze, overlap, or bounce responses;
`CheckCollisions` returns the same result without moving the sprite. Point,
rectangle, line, detailed line-hit, and overlap queries return borrowed sprites.
Detailed line hits include ordered entry/exit times and points. Games assert
`SpriteQueries` for line operations and `SpriteDisplayList` for sprite count,
bulk add/remove, remove-all, and collision-world reset. Bulk operations validate
the complete input before mutation and preserve its order. A foreign or closed
sprite passed to an operation returns the corresponding sprite error.

## Bitmap tables and animation

`LoadBitmapTable` returns an owned table whose `Frame` method returns borrowed
bitmap handles. Close the table only after its animations and sprites no longer
use those frames; borrowed frames cannot be closed independently.

`NewAnimation` creates a looping, allocation-free frame selector over a bounded
table range. Advance it with `Input.DeltaSeconds`, or select a fixed frame.
Pause and resume retain partial-frame time. Invalid construction returns
`ErrAnimationConfig`; an invalid table frame returns `ErrBitmapFrameRange`.

## Audio ownership and lifecycle

`LoadSoundEffect` loads a short, memory-backed sample/player pair;
`LoadFilePlayer` loads one streaming file player. Both expose play, stop,
stereo volume, stopped/playing/paused state, pause/resume, and explicit close.
Repeated `SoundEffect.Play` restarts the accepted effect without allocating a
new native player.

Pause players on pause/lock lifecycle events, resume them on resume/unlock,
and close file players before sound effects on termination. `Close` stops
playback before releasing native handles. A failed sound-effect construction
frees its partially acquired sample/player, and game initialization must close
earlier successful audio loads if a later load or configuration step fails.
After close, operations return `ErrAudioClosed`; invalid volume returns
`ErrAudioVolume`, playback rejection returns `ErrAudioPlay`, and load failures
return `AudioLoadError`.

Advanced games may separately capability-assert `AudioChannels`,
`AudioOutputs`, `Synthesizers`, `Sequencers`, and `AudioEffects`; these optional
slices do not widen the base `Context`. `AudioOutputs.DefaultAudioChannel`
returns a borrowed default-channel wrapper: closing the wrapper detaches graph
edges tracked through it but never removes or frees the native default channel.
`AudioChannel.Output` returns a borrowed post-effects, pre-volume-and-pan
`AudioSource` for nested submix and bus routing. The output expires when its
owning channel closes; closing the owner first detaches the output from its
downstream channel. A source remains attached to at most one channel, duplicate
attachment refreshes the native route, and direct or transitive channel cycles
return `ErrAudioRoute` before native mutation.
`DryLevelSignal` and `WetLevelSignal` return borrowed SDK-owned signals that
follow the channel level before and after effects, respectively. They implement
the existing `Signal` interface and can drive any current modulation setter.
Closing a wrapper detaches its tracked graph edges without freeing native state;
a later request creates a fresh wrapper. Closing the owning channel detaches and
expires both wrappers, after which their operations return
`ErrAudioGraphClosed`.
`AudioOutputState` synchronously reports headphone and headset-microphone
presence at the time of the call; poll it during updates when the UI or routing
must react to connection changes. `SetAudioOutputsActive` selects headphone and
speaker output; a Simulator may accept the selection without an observable
host-routing change.

`Synth.SetEnvelopeCurvature`, `SetEnvelopeVelocitySensitivity`, and
`SetEnvelopeRateScaling` configure the synth-owned, note-triggered ADSR rather
than a separately allocated `Envelope` modulation signal. Rate-scaling note
ranges must be ordered from the lower note to the higher note; non-finite
parameters and reversed ranges return `ErrAudioParameter`. A separately created
`Envelope` remains an explicitly owned signal and exposes the corresponding
setters for modulation graphs whose trigger is managed by the native graph.
LFOs separately expose current phase and initial phase, reproducible
sample-and-hold random seeding, and continuous global updating. Initial phase
must be finite and within zero through one; invalid values return
`ErrAudioParameter`. The similarly named envelope global-update method exists
only in the official Lua API, not the C SDK contract.

Microphone games capability-assert `Microphones`. Request permission before
recording, handle pending/denied/granted explicitly, and close the owned
`MicrophoneRecorder`. A `MicrophoneSamples` value is callback-scoped; copy only
the needed samples into a bounded caller-owned `[]int16`. The view expires when
the callback returns and the runtime never retains the destination. Microphone
failures use the separate `ErrMicrophone*` domain rather than audio errors.
On device, microphone input first enters a bounded native FIFO and is delivered
to Go during update frames rather than re-entering Go from the audio thread.
`PCMPlayers.NewPCMPlayer` synchronously copies mono signed 16-bit PCM into
native-owned storage and never retains the caller slice.

Sample-oriented games capability-assert `AudioSamples` and
`SamplePlayerFactory`. `NewSample` and `LoadSample` return explicitly owned
native buffers; `NewSampleFromData` synchronously copies the caller's bytes into
native-owned storage. `AudioSample.Data` returns a borrowed view whose metadata
and `CopyTo` remain valid only until the sample closes. A player borrows its
attached sample: close or replace the player before closing that sample, or
`AudioSample.Close` returns `ErrAudioSampleInUse`.

Samples can be reloaded in place, inspected, decompressed, and attached to a
new or existing player through `SamplePlayerControls`. Sample playback ranges
use start-inclusive, end-exclusive PCM frame indexes. Sample and file players
may expose `LoopCallbackPlayer`; callbacks are delivered through the bounded
update-frame queue. File players may expose `StreamingPlayerControls` for
reload, buffer length, loop range, underrun query, and stop-on-underrun control.
Invalid buffer lengths return
`ErrAudioBufferLength`; invalid ranges return `ErrAudioRange`.

`LFO.SetArpeggiation` requires at least one finite half-step offset and configures
the native arpeggiator sequence; an empty or non-finite step list returns
`ErrAudioParameter`. Audio completion callbacks are retained by ID and delivered
on an update frame. On device, native audio callbacks first enter a bounded FIFO
instead of re-entering Go from the audio thread.

Games that generate continuous PCM capability-assert `CallbackAudio` and pass
an existing `AudioChannel` to `NewPCMCallbackSource`. Construction attaches the
owned source to that channel. Its `PCMRenderCallback` receives at most 512
frames per call on the update goroutine, with a nil right slice for mono. The
returned frame count is clamped to the offered range; returning zero stops that
frame's refill. Each native adapter has four fixed source slots and a 4,096-frame
ring per slot (4,095 usable frames, enough for two 30 FPS intervals at 44.1 kHz).
The native audio callback only consumes the
ring. Empty rings produce silence and increment `UnderrunCount`; they never
re-enter Go. `Close` detaches the native callback source, releases its fixed
slot, and suppresses future render calls. A nil callback returns
`ErrAudioCallback`; an exhausted native pool returns `ErrAudioCreate`.

Custom synth games separately capability-assert `GeneratorSynthesizers`.
`NewGeneratorSynth` returns a native `Synth`, so its envelope, note scheduling,
parameter, modulation, routing, and `Instrument.AddVoice` contracts remain
available. Its `GeneratorRenderCallback` also runs only during update and emits
signed 16-bit PCM; the native bridge converts it to the official generator's
Q8.24 format. `GeneratorState` reports the native voice slot, latest note,
velocity, requested length, release state and sample end offset, Q0.32 phase
rate and signed rate delta, plus eight custom parameter slots. `Synth.SetParameter` uses the
official 1-based generator parameter indices 1 through 8; callback state exposes
them in `Parameters[0]` through `Parameters[7]`. Each native adapter has eight fixed generator userdata slots,
each with an independent 4,096-frame ring, and offers at most 256 frames per Go
render call. Native `PDSynth.copy` operations used by instruments copy the
parameter snapshot but allocate independent note state, phase-rate state, and
PCM storage; pool exhaustion rejects further native voice copies. A generator
synth cannot close while retained by an instrument, matching ordinary synth
ownership. `SetWaveform` and `SetWavetable` return `ErrAudioUnavailable` for a
generator synth because replacing its native generator would invalidate the
bounded userdata and ring lifetime.

## Fonts and deterministic UI

Native contexts also implement the narrow `FontGraphics` capability. Assert it
only in games that need custom fonts, load packaged `resources/fonts/name.fnt`
as `fonts/name`, and close every successful `Font` exactly once. `TextWidth`
and `Height` use the same native font metrics as drawing; closed fonts return
`ErrFontClosed`, and foreign handles return `ErrFontInvalid`.

Assert `TextGraphics` for bounded rectangle drawing, character or word
wrapping, alignment, tracking, leading, wrapping-height measurement, and glyph
metrics. `Glyph` returns advance and pair kerning plus a borrowed bitmap; the
bitmap cannot be closed and expires when its owning font closes. Missing glyphs
return `ErrFontGlyph`. Rectangle dimensions and maximum widths must be positive.
Fonts remain package-backed: load `.fnt` resources with `LoadFont`; native font
creation from opaque game-owned data is intentionally omitted because the
official API does not define a portable lifetime for Go-owned bytes.

Keep game UI as state plus a deterministic draw plan: measure strings, derive
coordinates, then execute `DrawTextFont` commands. HUD, pause, and game-over
screens do not require a generic widget tree, focus model, event router, or
layout engine. Add shared UI abstractions only after a second real game exposes
the same repeated contract.

## Filesystem and versioned persistence

Games assert the optional `FileSystem` capability for owned files, metadata,
non-recursive listing, and directory mutations. Paths are game-relative;
read-source flags distinguish packaged resources from the Data directory.
Every successful `OpenFile` result must be closed. Native failures are copied
into `FileOperationError` and classify as `ErrFileIO`.

`File.Seek` follows `io.Seeker`: after the official seek succeeds, it uses the
official current-position operation and returns that position. Consequently,
`Seek(0, io.SeekCurrent)` reports the current position without a separate
public `Tell` method. A failure to obtain the position is reported as a
`FileOperationError` for the `tell` operation. The official transient `geterr`
pointer is never exposed or retained; adapters copy it only while constructing
the error for the failing filesystem operation.

The separate `playdate/store` package provides bounded checksummed envelopes,
atomic temporary-file replacement, backup recovery, and explicit one-version
migrations. Serialization and corrupt-save policy remain application-owned.
Migration failures classify as `store.ErrMigration` and preserve their direct
cause for `errors.Is`; callers must not parse error strings.

Native capability sentinels remain members of the common `playdate` package and
are grouped by API area in the source tree. `playdate/store` owns its separate
error domain because it is a higher-level package rather than a native context
capability. Source-file placement does not change error identity or import
paths.

## System integration

`SystemMenu` owns at most three action, checkmark, or options items. Items retain
their callback until removal, may be removed idempotently, and are cleaned up
after termination. `Localization` exposes system language and copied `.strings`
lookups; missing-key fallback remains game policy.

`Accelerometer` requires explicit enablement and is disabled at termination.
`PowerMonitor` and `SystemPreferences` expose read-only device/global state.
`SystemEnvironment` exposes copied offline clock, calendar, elapsed-time, and
runtime/game version values. `Launcher` remains the single exit-to-launcher
capability.

`Scoreboards` is an optional bounded asynchronous service, not general
networking. `DebugMessages` is a separate bounded diagnostic FIFO for Simulator
and serial input. Scoreboards allow one pending request of each of their four
operation kinds; native completions copy SDK-owned data into a fixed four-slot
queue, and game callbacks run at the next update boundary. Immediate request
failure releases the operation slot. Neither capability may re-enter game code
from a native callback, and queued or late callbacks after termination are
suppressed.

## Device source compatibility

Device compilation uses TinyGo and the Cortex-M7 target, with its `linux/arm`,
`tinygo`, `baremetal`, and `cortexm` build selection plus gopdsdk's `qemu` tag.
The analyzer loads host Go packages; `device-build-constraint` is informational
when a selected file cannot match those fixed device constraints. It does not
prove the absence of an alternative device implementation. Unknown application
and Go-version tags are not assumed false.

Go assembler `TEXT` implementations in `.s` files do not provide TinyGo
function bodies. `device-go-assembly` identifies matching bodyless Go
declarations in the selected package; it excludes assembly files with known
incompatible device constraints. This is not a ban on native ARM assembly.
The declaration and symbol matching follows the
[Go assembler syntax](https://go.dev/doc/asm).

`device-compiler-directive` diagnoses `//go:linkname` aliases to public package
functions already forbidden by the device symbol rules. It does not ban all
directives, private linker names, or `go:linkname` itself. Generic code is also
not generally forbidden: `device-channel` diagnoses channel type arguments,
including inferred and imported named channel types.

`device-panic-cleanup` warns when local control flow permits an explicit builtin
`panic` after a deferred call has been registered. It is a possible-path warning,
not proof that cleanup is required or that branch conditions are jointly
feasible. Normal-return defer remains supported. Implicit runtime panics,
panics in callees, and deferred panics are outside this rule's current scope.

The device checker follows statically resolved dependency calls and package
initialization for goroutines, channels, `select`, `recover`, forbidden imports,
and forbidden runtime symbols. It reports the shortest known path, following
generic calls and immutable local function/closure aliases. Uncalled closure
bodies do not contribute to those paths. Reassigned function values, globals,
parameters, and unresolved interface calls are outside that proof. Audited
`time`, `reflect`, and `runtime` APIs are terminal contracts rather than paths
through the host standard library's implementation.

## Device Go subset

The accepted device profile is sequential Go with TinyGo conservative GC,
`scheduler none`, and fail-stop panic handling. Allocation and `runtime.GC` are
supported, but applications must keep their live object graph bounded and treat
pause and memory bounds as workload-specific. Panic is a terminal trap without
stack unwinding, deferred cleanup, or recovery.

Goroutines, channels, `select`, standard-library clocks, sleep, timers, and
tickers are unsupported. Their bounded sequential use cases are replaced by
`playdate/schedule`. `fmt` is replaced by typed `strconv` and bounded writers,
while `encoding/json` is replaced by `playdate/json`.

Normal-return `defer` is supported by conservative device builds. This includes
normal and early returns, LIFO execution, argument evaluation at registration,
named-result mutation, and repeated registration. Dynamic registration,
including `defer` in a loop, can allocate and should not be used in update hot
paths without measuring heap growth. Device builds use `-panic trap`: panic is
terminal and does not unwind the stack or run deferred cleanup.

The production device linker permits the audited public reflection subset:
`TypeOf`; type kind, name, and struct-field metadata; struct tags; `ValueOf`,
`Elem`, `Field`, `Index`, `Kind`, validity and set/interface
checks; `Interface`; the verified `int` to `int64` numeric `Convert`; and `SetInt`,
`SetString`, and `SetMapIndex` mutation. It rejects every other newly linked
public `reflect` symbol. In particular, dynamic calls and construction,
reflected methods and channels, and unsupported function Type APIs remain
unsupported, as do `recover`, finalizers, application cgo, and application
runtime-control hooks. Prefer direct typed code or generated accessors where a
static implementation is practical. Reflection allocation and memory bounds are
workload-specific and require measurement before use in an update hot path.

The linked-symbol audit only sees symbols retained in the ELF. TinyGo can inline
an unsupported operation such as `reflect.Value.Call` into a panic trap, leaving
no symbol for that audit to reject. The source analyzer still reports the
unsupported API; successful packaging does not establish its runtime support.

Pure `time.Duration` value, parsing, and formatting operations are supported.
This does not enable `time.Now`, sleep, timers, or tickers; use Playdate-clock
tasks from `playdate/schedule` for those cases. Duration formatting can retain
normal-return defer and allocate, so measure it before use in an update hot
path.

Simulator compilation alone does not prove device compatibility for a standard
library package. See [COMPATIBILITY.md](COMPATIBILITY.md) for the evidence
matrix and exact verified toolchain.
