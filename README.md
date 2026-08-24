# gopdsdk

<p align="center">
  <img src="docs/assets/gopdsdk-logo.svg" alt="gopdsdk" width="720">
</p>

An independent Go SDK and toolchain for building Playdate applications.

gopdsdk provides a stable public Go API for creating complete offline Playdate
games without a game-owned C bridge or imports from internal packages. It
covers graphics, sprites and collisions, input, system integration, files and
persistence, video, diagnostics, and the complete offline audio surface. Native
resources have explicit ownership and lifecycle contracts, while callbacks and
frame-spanning work use bounded, device-aware designs.

The accompanying `gopdsdk` CLI owns the development workflow: it can inspect a
host, create a project, build and package games for the official Simulator or a
physical Playdate, launch them, deploy through USB, and retrieve device logs.
Projects remain ordinary Go modules, so game logic can be tested with the Go
toolchain before target-specific compilation.

Device builds use a documented sequential Go profile designed around
Playdate's memory and callback constraints. Bounded packages such as
`playdate/schedule`, `playdate/json`, `playdate/store`, and
`playdate/diagnostics` provide practical replacements for desktop-runtime
facilities that do not fit the device. Focused examples demonstrate each public
capability, and [API.md](API.md) defines the stable contract. Supported hosts,
toolchains, target evidence, and known limitations are tracked separately in
[COMPATIBILITY.md](COMPATIBILITY.md).

## Support status

| Host    | Native CI | Official SDK/Simulator  | Device build/deploy           |
| ------- | --------- | ----------------------- | ----------------------------- |
| Windows | passing   | verified with SDK 3.1.1 | verified on physical hardware |
| macOS   | passing   | unverified              | unverified                    |
| Linux   | passing   | unverified              | unverified                    |

Native CI proves Go behavior, path policy, CLI composition, and external-module
consumption. It does not prove that an official Simulator starts or that USB
deployment works on that host.

## Device Go profile

Device applications use sequential Go compiled by TinyGo with conservative GC,
`scheduler none`, and fail-stop panic handling. The profile deliberately does
not promise compatibility with every Go language mechanism or standard-library
package.

| Mechanism                                                                                                         | Decision                               | Available in device builds                                                             |
| ----------------------------------------------------------------------------------------------------------------- | -------------------------------------- | -------------------------------------------------------------------------------------- |
| Ordinary sequential Go: functions, methods, interfaces, structs, arrays, slices, maps, pointers, and control flow | Supported                              | Yes                                                                                    |
| Allocation, conservative GC, and `runtime.GC`                                                                     | Supported                              | Yes; memory and pause bounds remain workload-specific                                  |
| `panic`                                                                                                           | Supported as a terminal fail-stop trap | Yes; no unwinding, deferred cleanup, or recovery                                       |
| `defer` on normal return                                                                                          | Supported bounded subset               | Yes with conservative GC; dynamic registration can allocate                            |
| Public `reflect` inspection, extraction, conversion, and mutation                                                 | Supported bounded subset               | Yes with conservative GC; only documented operations pass the exact symbol gate         |
| Goroutines                                                                                                        | Replaced                               | Use stackless `playdate/schedule` tasks                                                 |
| Channels and `select`                                                                                             | Replaced                               | Use bounded non-blocking queues and round-robin polling from `playdate/schedule`        |
| `time.Now`, `Sleep`, timers, and tickers                                                                          | Replaced                               | Use Playdate-clock delayed, deadline, and repeating tasks; pure duration/value operations are supported |
| `fmt`                                                                                                             | Replaced                               | Use typed `strconv`, bounded buffers, builders, and writers                            |
| `encoding/json`                                                                                                   | Replaced                               | Use the bounded reflection-free `playdate/json` package                                |
| `recover`                                                                                                         | Unsupported                            | Use explicit error returns                                                             |
| Finalizers                                                                                                        | Unsupported                            | Use explicit ownership and lifecycle cleanup                                           |
| Application cgo                                                                                                   | Unsupported                            | Native integration remains owned by gopdsdk bridges                                    |

The decision column records the audit outcome; the availability column controls
what applications may rely on in current device builds. See [API.md](API.md) for
the public contract and [docs/ROADMAP.md](docs/ROADMAP.md) for post-1.0 work.

## Requirements

- Go 1.26.x for development, CLI use, tests, and Simulator compilation.
- The official Playdate SDK for packaging, Simulator runs, and device tools.
- A native C compiler supported by `doctor` for Simulator builds.
- TinyGo 0.41.1 and GNU Arm Embedded 15.3.1 for the verified device build.

Set `PLAYDATE_SDK_PATH` when the SDK is outside its conventional host location.
TinyGo and the Arm toolchain are unnecessary for Simulator-only development.

## Install

Run the released CLI directly at that version:

```sh
go run github.com/ivan-gromov-dev/gopdsdk/cmd/gopdsdk@v1.0.0 doctor
go run github.com/ivan-gromov-dev/gopdsdk/cmd/gopdsdk@v1.0.0 init --module example.com/my-game ./my-game
cd my-game
go mod tidy
```

The tagged CLI creates a project requiring the same module version without a
local `replace`. A development CLI built from a checkout intentionally writes a
local replacement instead; use the checkout workflow below when developing
from source. See [API.md](API.md), [COMPATIBILITY.md](COMPATIBILITY.md),
[MIGRATING.md](MIGRATING.md), and [RELEASING.md](RELEASING.md). Post-1.0
real-game validation and networking research are recorded in
[docs/ROADMAP.md](docs/ROADMAP.md).

## Environment diagnostics

Run the read-only environment check:

```sh
go run ./cmd/gopdsdk doctor
```

If the Playdate SDK is not in a conventional location, set the official
`PLAYDATE_SDK_PATH` environment variable or provide it explicitly:

```sh
go run ./cmd/gopdsdk doctor --sdk /path/to/PlaydateSDK
```

The command assesses SDK discovery, development, Simulator compilation, device
compilation, and device deployment separately. A tool reported as `UNVERIFIED`
was found but has not yet passed an end-to-end probe.

Run the Simulator and device-build probes during diagnostics with:

```sh
go run ./cmd/gopdsdk doctor --probe
```

This verifies both packaging pipelines but intentionally does not install or run
anything on a connected Playdate; `device-deploy` remains a separate capability.

Run the native Simulator toolchain probe with:

```sh
go run ./cmd/gopdsdk probe simulator --sdk /path/to/PlaydateSDK
```

The probe builds a temporary Go shared library, verifies the required
`eventHandler` export, packages a `.pdx`, and removes all temporary artifacts.

Launch the packaged probe in Playdate Simulator and verify `kEventInit`, the
update callback, and a Hello World `drawText` call with:

```sh
go run ./cmd/gopdsdk probe simulator --run --sdk /path/to/PlaydateSDK
```

The automated probe intentionally terminates the Simulator process it launches
after verification succeeds or the timeout expires.

Verify the first device compilation stage with:

```sh
go run ./cmd/gopdsdk probe device
```

This currently proves a hard-float Cortex-M7 TinyGo object, the official
Playdate `setup.c` and `link_map.ld` link, an ELF32/ARM executable with
`eventHandlerShim`, a one-time TinyGo runtime bootstrap, no unresolved symbols,
and conversion of `pdex.elf` into a packaged `pdex.bin` with the official `pdc`.
Device builds default to TinyGo conservative GC with a checked 256 KiB heap,
bounded-memory validation, and deterministic fail-stop OOM behavior.
Deployment and physical-device execution are proven on the verified Windows setup.
The marker was rendered after the Go event handler returned.

Build an importable application package for a physical Playdate with:

```sh
go run ./cmd/gopdsdk build device --sdk /path/to/PlaydateSDK ./examples/hello
```

The default output is `build/<package>.pdx`; `--output` selects another path and
`--force` replaces an existing output.

After the read-only connection probe succeeds, explicitly install the verified
probe package with:

```sh
go run ./cmd/gopdsdk probe device --install --sdk /path/to/PlaydateSDK
```

`--install` changes the connected device by copying the probe game. It does not
run the game; start `gopdsdk Device Probe` manually on the Playdate.

Build, install, and launch the verified device probe in one command with:

```sh
go run ./cmd/gopdsdk run device --sdk /path/to/PlaydateSDK ./examples/hello
```

This changes the connected device, then asks the official `pdutil` to launch
the installed package. The existing `gopdsdk run <package>` form continues to
build and launch a Simulator application.

Safely check whether `pdutil` can open a connected Playdate without mounting,
installing, or running anything:

```sh
go run ./cmd/gopdsdk probe connection --sdk /path/to/PlaydateSDK
```

Connect and unlock the Playdate over USB before running the probe. A successful
probe verifies communication only; it does not modify the device or prove that
the packaged game runs.

Connect and unlock the Playdate, then mount its data disk and print
`crashlog.txt` directly to the console with:

```sh
go run ./cmd/gopdsdk crashlog --sdk /path/to/PlaydateSDK
```

Retrieve `errorlog.txt` through the same flow with:

```sh
go run ./cmd/gopdsdk errorlog --sdk /path/to/PlaydateSDK
```

Both commands read but do not modify the selected log. Log contents are written
to stdout, so they can be redirected to a file, and the resolved source path is
written to stderr. Mounting changes the connected Playdate into data-disk mode;
neither command proves that a game ran successfully.

## External-game measurements

`playdate/diagnostics` aggregates a bounded run without allocating while
samples are recorded. External games use the runtime-provided frame delta,
sample live heap from their Go runtime, and count the native resources they
explicitly own:

```go
collector, _ := diagnostics.New(1800) // one minute at 30 FPS
// update and render the frame
var memory runtime.MemStats
runtime.ReadMemStats(&memory)
_ = collector.Record(diagnostics.Sample{
	FrameMilliseconds: uint32(ctx.Input().DeltaSeconds*1000 + 0.5),
	HeapBytes:          memory.HeapAlloc,
	NativeResources:    ownedResources,
})
```

After the interval, `Report` returns frame-time mean/p50/p95/p99/max, heap
start/end/max/growth, and resource start/end/min/max. Render or persist that
report after measurement so diagnostic I/O does not distort the samples. Use
`gopdsdk build device` for static RAM, ELF, and packaged PDX sizes, and label
Simulator and physical-device measurements separately.

## Build a Simulator application

Create an independent starter project with the CLI using:

```sh
go run ./cmd/gopdsdk init --module example.com/my-game --author "Your Name" --bundle-id com.example.my-game ./my-game
```

When run from a development checkout, the generated `go.mod` uses a local
`replace` directive. A tagged CLI uses its own published module version without
`replace`. Run `go mod tidy` once to resolve that dependency and create
`go.sum` before the first build. The command never overwrites an existing path.

An application is an importable Go package that provides
`New() playdate.Game` and an official Playdate `pdxinfo` file in the same
directory. Source assets live only below `resources/`; its contents become the
root of the packaged PDX. Build the included Hello World example on Windows with:

```sh
go run ./cmd/gopdsdk build --sdk /path/to/PlaydateSDK ./examples/hello
```

The default output is `build/hello.pdx`. Use `--output` to select another path.
The build command does not overwrite an existing artifact. Simulator execution
on macOS and Linux remains unverified; device builds use the verified
TinyGo conservative runtime.

Inspect either build without running compilers or SDK tools:

```sh
go run ./cmd/gopdsdk build --dry-run --sdk /path/to/PlaydateSDK ./examples/hello
go run ./cmd/gopdsdk build device --dry-run --sdk /path/to/PlaydateSDK ./examples/hello
```

Dry-run output is a typed semantic plan with structured executable arguments,
portable `${WORK}` and `${PACKAGE}` tokens, and explicit artifact retention.
Temporary workspaces are marked `cleanup`; published `.pdx` outputs are marked
`preserve`. Cleanup rejects unresolved, relative, and filesystem-root paths.

The CLI carries its project-owned Simulator and device ABI bridge sources as
package-owned `go:embed` assets. It materializes those version-matched sources
only inside the temporary workspace. Official `pd_api.h`, `setup.c`, and
`link_map.ld` remain external inputs read from the selected Playdate SDK.

Build and launch the example, replacing its previous build artifact, with:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/hello
```

The command leaves the Simulator process running. `Process.Kill` is used only by
the automated probe command.

## Lifecycle and input parity

Lifecycle and input parity are complete on the verified Windows profile. The same game code received
matching button transitions, crank state, dock state, frame delta, and ordered
pause/resume and lock/unlock callbacks in Simulator and on a physical Playdate.
The device reported an approximately 33.40 ms frame delta and completed the
required 60-second regression soak. Low-power and terminate routing are covered
by deterministic pure-Go tests and the common ABI event path; those two events
were not separately induced during physical acceptance.

The `examples/lifecycleinput` game exercises the same public lifecycle and
per-frame input snapshots on Simulator and device. Build or run that single
package on both targets:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/lifecycleinput
go run ./cmd/gopdsdk run device --sdk /path/to/PlaydateSDK ./examples/lifecycleinput
```

The display reports an ordered lifecycle trace and counters; current, pressed,
released, held, and latched edge button masks; crank angle and change; dock
transitions; frame delta; and the soak marker. Pure-Go tests supply fixed input
and lifecycle sequences to this same game implementation.

## Launch and lifecycle control

The v0.10.0 `playdate.SystemControls` capability adds copied
launch arguments and game path, restart arguments, an owned 400×240 system-menu
image, auto-lock and crank-sound controls, mirror lifecycle events, and bounded
button callbacks. Button callbacks preserve multiple ordered up/down
transitions of one button between frame snapshots; they are delivered from a
fixed native queue immediately before the next Go update and expose bridge
overflow counts. Termination clears callbacks and the retained menu image and
restores system settings before the game releases its owned resources.

The `examples/systemcontrol` consumer displays launch and mirror state, button
event timestamps and overflow, installs a menu image, toggles auto-lock and
crank sounds with B, and restarts with P10 launch arguments when A is pressed:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/systemcontrol
go run ./cmd/gopdsdk run device --sdk /path/to/PlaydateSDK ./examples/systemcontrol
```

Its pure-Go consumer, runtime, public-API snapshot, and generated
Simulator/device ABI tests pass. On 2026-08-13 the consumer compiled and
packaged with the official Windows SDK 3.1.1, and its conservative hard-float
device build used 283,228 bytes of static RAM and produced a 1,196,892-byte ELF
and a 49,261-byte PDX. On 2026-08-17 Simulator interaction covered the menu
image, settings, buttons without overflow, and mirror state; restart closed the
Simulator application instead of demonstrating a restarted instance. USB
installation and launch on a physical Playdate then passed the complete
interaction matrix, including restart with `p10-restarted` launch arguments.
Final release soak, bounded-memory, and unchanged post-run device-log checks
passed on the verified Windows/Playdate profile.

## Clock, calendar, and device information

The v0.10.0 `playdate.SystemEnvironment` capability exposes
copied seconds and milliseconds since the Playdate epoch at January 1, 2000,
epoch/calendar conversion, the SDK high-resolution elapsed timer, and copied
OS version, language, and game PDX version. `DateTimeToEpoch` rejects invalid
dates and values beyond the representable `uint32` epoch. Its `Weekday` field
is derived by `EpochToDateTime` and ignored when converting back to epoch
seconds. Server time is not part of this offline capability.

`Input.DeltaSeconds` uses the separate wrapping monotonic millisecond clock, so
frame updates do not reset a game's elapsed timer. The
`examples/systemenvironment` consumer displays all P10.2 values and resets the
elapsed timer when A is pressed:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/systemenvironment
go run ./cmd/gopdsdk run device --sdk /path/to/PlaydateSDK ./examples/systemenvironment
```

Its pure-Go consumer, runtime, public-API snapshot, generated Simulator/device
ABI tests, official Windows SDK 3.1.1 compilation/package, and conservative
hard-float device build pass. The device build used 294,788 bytes of static RAM
and produced a 1,241,272-byte ELF and a 70,370-byte PDX. On 2026-08-17
user-confirmed Simulator and physical-Playdate interaction covered current
epoch/calendar display, exact seconds round-trip, elapsed growth and reset, and
copied system information. The final combined release artifacts passed
physical-device installation, execution, soak, bounded-memory, and unchanged
post-run device-log checks.

## Bounded cooperative scheduling

`playdate/schedule` spreads explicit task steps across update frames without
goroutines, blocking, or hidden callbacks. Construction fixes task capacity and
the maximum steps per frame; `Update` runs each ready task at most once in FIFO
order. Tasks return `Yield`, `Complete`, `Delay`, or `Repeat`, and can also be
scheduled at a wrapping `CurrentTimeMilliseconds` deadline. The optional elapsed
budget is checked only between steps, so task steps must remain small.

The package also provides fixed-capacity non-blocking generic queues and a
deterministic round-robin poller. Full, empty, and nothing-ready states are
explicit results rather than waits or panics. Queue operations are sequential;
native callback bridges must continue to copy records into their own bounded
update-boundary queues.

`examples/schedule` processes 120 items through four cooperative tasks and
terminates its scheduler with the application lifecycle. Pure-Go tests, the
official Windows SDK 3.1.1 Simulator build and launch, and the TinyGo 0.41.1
conservative device build, COM3 installation, and launch pass. The final device
artifact uses 283,068 bytes of static RAM and produces a 1,188,160-byte ELF and
a 48,729-byte PDX. On 2026-08-18 the user confirmed physical-device `PASS`
markers for a two-step peak, exactly 40 task steps, completion in 20 frames, and
equal 30-item progress across all four tasks. The physical frame-time,
bounded-memory, memory-growth, soak, and unchanged post-run device-log gates
also passed by user confirmation; exact measurement values were not recorded.

`examples/reflection` exercises the bounded public reflection subset: type kind,
name and struct-field metadata; struct tags; `Interface`; numeric `Convert`;
and mutable struct fields, slice elements, and map entries. The device linker
uses a fail-closed symbol allowlist, so dynamic calls and construction, reflected
methods and channels, unsupported function-type APIs, and new TinyGo reflection
paths are rejected. Prefer direct typed code or generated accessors when the
shape is known. The TinyGo 0.41.1 conservative device build uses 282,684 bytes
of static RAM and produces a 1,279,604-byte ELF and a 59,359-byte PDX. The
Simulator launch and COM3 device installation and launch commands pass. Visual
device acceptance on 2026-08-18 confirmed `Operations PASS`, a stable
12,544-byte aggregate allocation measurement, `Memory PASS`, and `Soak PASS`
after 60 seconds. The user also confirmed that post-run `crashlog.txt` and
`errorlog.txt` remained unchanged.

## Bounded JSON

`playdate/json` provides a reflection-free value tree for device-safe JSON
without native callbacks, userdata, `defer`, or `recover`. Decoding accepts an
`io.Reader` or byte slice and applies explicit document-byte, nesting-depth,
node-count, and decoded-string limits. Objects preserve source order and
duplicate names; numbers retain their original valid spelling. Encoding streams
validated compact or pretty output to an `io.Writer` without buffering the
complete result.

```go
config, err := json.DecodeBytes(data, json.Limits{
	MaxBytes: 4096,
	MaxDepth: 8,
})
if err != nil {
	return err
}

level, ok := config.Lookup("level")
if !ok || level.Type != json.Number {
	return errors.New("missing numeric level")
}

return json.Encode(output, config, json.EncodeOptions{})
```

`examples/jsoncodec` reads a packaged configuration, performs bounded decoding,
schema lookup and mutation, then writes into a fixed-capacity buffer. Unit tests,
the full Go suite, Windows SDK 3.1.1 Simulator build and launch, conservative
hard-float device build, USB installation, and user-confirmed physical behavior
passed. The accepted artifact uses 283,900 bytes of static RAM and produces a
1,279,000-byte ELF and a 62,029-byte PDX. The final P11 soak, bounded-memory,
memory-growth, and unchanged post-run device-log checks passed by user
confirmation on 2026-08-18; separate Simulator visual confirmation remains
unverified.

## Bitmap acceptance

Bitmap acceptance is complete on the verified Windows profile. The same public bitmap API
loaded, created, filled, measured, drew, scaled, and explicitly closed native
resources in Simulator and on a physical Playdate. Owned and borrowed handles,
double-close, use-after-close, invalid arguments, and platform-independent
errors have deterministic pure-Go coverage. Device acceptance used the
hard-float bridge and produced no new `errorlog.txt` or crashlog entry.

Application packages keep source assets below a dedicated `resources`
directory. Its contents become the PDX resource root:

```text
game/
  game.go
  game_test.go
  pdxinfo
  resources/
    images/
    audio/
    fonts/
    data/
```

For example, `resources/images/player.png` is compiled into the PDX as
`images/player.pdi` and loaded with `LoadBitmap("images/player")`. Files outside
`resources` are never copied into the package.

Launcher artwork uses the same staging rule. Set, for example,
`imagePath=images/launcher` in `pdxinfo`, then provide:

```text
resources/images/launcher/
  card.png        # 350x155
  icon.png        # 32x32
  launchImage.png # 400x240
```

The compiler places these files at the PDX paths named by `imagePath`. The
official format also permits highlighted/pressed artwork and launch animation
directories; these can be added under the same resource directory.

The `examples/bitmap` game loads a packaged 64x64 bitmap, creates and fills an
owned bitmap, draws both bitmaps, draws a scaled copy, reports the loaded
dimensions, and closes both resources on termination. Run the same package on
both targets:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/bitmap
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/bitmap
```

The accepted display contains `PDX: 64x64`, a PASS line, the packaged icon at
two scales, and a solid square created at runtime. Pure-Go tests verify the
operation order and one-time ownership cleanup.

For pixel editors, masks, collision maps, transformed assets, and persistent
screen captures, assert `playdate.BitmapDataGraphics`. Its bitmap-data view is
callback-scoped, while copies, rotations, tables, and display snapshots return
owned resources that the game closes explicitly.

`examples/bitmapdata` is the complete P7.2 acceptance scene. It edits owned
pixels with dirty tracking, copies and reloads bitmaps, creates and reloads a
table, attaches and inspects a mask, performs flipped mask collision, creates a
rotated bitmap, and retains a display-buffer snapshot:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/bitmapdata
go run ./cmd/gopdsdk run device --memory conservative --install --sdk /path/to/PlaydateSDK ./examples/bitmapdata
```

On 2026-08-08 this scene passed deterministic tests, official Windows SDK
3.1.1 Simulator build and visual execution, conservative hard-float device
build, USB installation through COM3, launch, and visual execution on a
physical Playdate. The accepted device artifact uses 275,580 bytes of static
RAM and produces an 872,964-byte ELF and a 38,768-byte PDX. Extended soak,
memory-growth measurement, and post-run device-log inspection passed on
2026-08-08.

For complete text layout and custom renderers, assert `playdate.TextGraphics`.
It adds bounded aligned text, character and word wrapping, tracking, leading,
wrapping-height measurement, and font-owned borrowed glyph bitmaps with advance
and kerning metrics. Packaged `.fnt` resources remain the portable offline font
source on both Simulator and device.

The public package remains a single `playdate` import but is organized by
domain (`application`, `lifecycle`, `input`, `graphics`, `bitmap`, `audio`, and
`errors`). `playdate.Context` composes narrower capabilities so application
helpers can depend on only the API surface they use.

## Sprites

`examples/sprites` exercises explicitly owned sprites, bitmap assignment,
position and relative movement, visibility, z-index, idempotent display-list
membership, the shared update/draw pass, display presentation controls, and
live logical dimensions, nominal refresh rate, and measured FPS introspection.
Sprites must be closed before the bitmaps they reference.

On 2026-08-08 the P7.4 scene passed matching visual acceptance in the official
Windows SDK 3.1.1 Simulator and on a physical Playdate after conservative-GC
hard-float build and USB installation through COM3. The accepted device
artifact uses 292,016 bytes of static RAM and produces a 69,505-byte PDX.
Bounded-memory, soak, and post-run device-log gates passed on 2026-08-08;
performance regression evidence remains unverified.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/sprites
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/sprites
```

`examples/spritepresentation` is the P8 sprite acceptance scene. Its P8.1
coverage includes sprite
center, bounds, position and state getters, all image flips, draw mode, opacity,
stencil patterns and images, clipping, draw-offset policy, and independent
update/collision enable state. It also attaches an explicitly owned native
tilemap to a sprite and alternates one tile while reporting its native getter.
At startup it additionally self-checks the P8.2 line and detailed-hit queries,
non-mutating collision checks, sprite count, bulk add/remove, remove-all state,
and collision-world reset. It also renders a procedural sprite through a draw
callback, counts ordered update callbacks, and verifies a pair-specific bounce
response before displaying `P8.3 PASS`.
Press A or B to toggle the enable states, Up to
clear or restore both stencils, Down to clear or restore clipping, and Left or
Right to demonstrate the difference between offset-following and
offset-ignoring sprites.

On 2026-08-08 this scene passed visual acceptance in the official Windows SDK
3.1.1 Simulator and on a physical Playdate. The conservative-GC hard-float
device artifact uses 281,880 bytes of static RAM and produces a 1,304,340-byte
ELF and 61,792-byte PDX. The final P8 device acceptance covers the required
conservative-GC soak, bounded memory growth, and unchanged post-run device
logs.

On 2026-08-09 the expanded P8.2 startup self-check and visible PASS status
succeeded in the official Windows SDK 3.1.1 Simulator and on a physical
Playdate. The conservative-GC hard-float device artifact uses 282,512 bytes of
static RAM and produces a 1,398,940-byte ELF and 69,893-byte PDX. The final P8
device acceptance covers the required conservative-GC soak, bounded memory
growth, and unchanged post-run device logs.

On 2026-08-09 the P8.3 build displayed `P8.3 PASS` in the official Windows SDK
3.1.1 Simulator. Its startup pair-specific collision-response check succeeded,
and the visible update-callback counter increased continuously. The conservative
hard-float device build uses 283,104 bytes of static RAM and produces a
1,479,780-byte ELF and a 73,656-byte PDX. USB installation through COM3 and
launch passed; user-confirmed physical execution showed matching `P8.3 PASS`, a
continuously increasing update counter, and the procedural draw. A
user-confirmed 60-second conservative-GC physical-device soak passed on
2026-08-09. The user also confirmed bounded memory growth and unchanged
post-run `crashlog.txt` and `errorlog.txt` for the accepted device run.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/spritepresentation
go run ./cmd/gopdsdk run device --memory conservative --install --sdk /path/to/PlaydateSDK ./examples/spritepresentation
```

## Collisions

`examples/collision` is a deterministic collision scene using collide
rectangles, slide/freeze/overlap/bounce responses, resolved movement, and
point/rectangle/overlap queries. The SDK additionally exposes non-mutating
collision checks, simple and detailed line queries, sprite count, bulk
display-list membership, remove-all, and collision-world reset. Portable
results contain ordered collision or line-intersection geometry without
exposing native pointers.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/collision
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/collision
```

## Bitmap-table animation

`examples/animation` loads a packaged bitmap table and selects borrowed frames
with the allocation-free `Animation` helper. It supports delta-time looping,
fixed frames, and pause/resume while retaining partial-frame time.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/animation
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/animation
```

## Base audio

The base audio implementation established two deliberately narrow vertical APIs: a memory-backed short
sound effect and one streaming file/music player. Both expose stereo volume,
stopped/playing/paused status, lifecycle pause/resume, and explicit close.
`examples/audio` now retains those paths while also serving as the advanced sample
advanced-audio acceptance game described below.

Run the same package on both targets:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/audio
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/audio
```

The original implementation and generated ABI remain regression-tested.
Advanced synthesis and microphone input remain optional capabilities outside
that base slice.

## Fonts and game UI

`examples/fontsui` is the complete P7.3 acceptance scene. It loads
`resources/fonts/gopdsdk-ui.fnt` as `fonts/gopdsdk-ui`, configures tracking and
leading, measures wrapped height, reads glyph advance and pair kerning, and
draws centered word-wrapped text inside a bounded rectangle. Its pure
`LayoutPlan` produces stable HUD, score, pause, and game-over commands; A
increments score or restarts after game over, B ends the run, and lifecycle
pause/resume selects the pause screen. Glyph bitmaps borrow the font lifetime,
and the owned font is closed on termination.

Run the same example on either target with:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/fontsui
go run ./cmd/gopdsdk run device --memory conservative --install --sdk /path/to/PlaydateSDK ./examples/fontsui
```

On 2026-08-08 deterministic tests, Windows SDK 3.1.1 Simulator visual
acceptance, and physical-device installation, launch, and visual acceptance
passed with bounded wrapping, matching custom-font HUD, score, pause,
game-over, and restart screens. The conservative hard-float artifact uses
278,688 bytes of static RAM and produces an 826,340-byte ELF and a 41,203-byte
PDX. Device soak, memory-growth measurement, and post-run log inspection passed
on 2026-08-08.

## Drawing primitives and state

`examples/primitives` is the consumer-driven acceptance scene for immediate
mode lines, outlined and filled rectangles, ellipses, outlined and filled
triangles and polygons, rounded rectangles, solid/XOR/8x8 pattern paint, line
caps, background color, local and screen clipping, draw offset, and bitmap draw
modes. Games opt into the narrow `playdate.PrimitiveGraphics` and
`playdate.GraphicsState` capabilities instead of expanding every `Graphics`
fake.

Run the same scene on either target with:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/primitives
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/primitives
```

Deterministic unit tests, Windows SDK 3.1.1 Simulator compilation and visual
execution, hard-float device build, USB deployment, and matching physical
Playdate execution passed on 2026-08-02. The accepted device artifact used
266,556 bytes of static RAM and produced a 29,160-byte PDX. Conservative-GC
soak and memory-growth measurement remain unverified.

## Framebuffer and offscreen drawing

Native contexts now expose narrow `playdate.FramebufferGraphics` and
`playdate.OffscreenGraphics` capabilities. Framebuffer access is callback
scoped and zero-copy, combines explicitly reported dirty rows, and rejects
checked access after the callback. Direct byte mutations require an explicit
`MarkDirtyRows` call. `DrawInto` accepts owned bitmaps only and always restores
the previous drawing context before returning.

The portable pixel layout, validation, dirty-range aggregation, generated
Simulator bridge, and both device memory profiles are unit-tested. SDK
integration, Simulator visual execution, device build, USB deployment, and
physical-device execution for this capability remain unverified.

## Tile map and camera

`playdate.TileMap` renders only cells intersecting an integer-pixel
`playdate.Camera`; `TileDrawStats` makes that per-frame bound observable.
Tile zero is empty and other values index caller-owned bitmaps. Static solid
tile overlap is available through `IntersectsSolid` and deliberately remains
separate from sprite collision and movement response.

The deterministic unit suite covers camera clamping, visible-range work,
screen-space placement, copied configuration, validation, draw failures, and
static collision edges. `examples/tilemap` is the consumer-driven vertical
slice. Windows SDK 3.1.1 Simulator visual acceptance and conservative-GC
physical-device build, USB deployment, controls, jump, collision, camera, and
matching scene execution passed on 2026-08-02. The accepted device artifact
used 270,908 bytes of static RAM and produced a 37,440-byte PDX. Soak and
memory-growth measurement remain unverified.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/tilemap
```

## Repeated resource ownership

Resources remain explicitly owned by the scene that loads or creates them.
Initialization closes already acquired resources when a later step fails, and
termination closes each owned resource once in dependency-safe order. Borrowed
handles, such as bitmap-table frames, are never independently closed.

The resource-ownership audit found no two real consumers with matching cache and transition
semantics, so the SDK does not yet expose a resource manager or reference
counting API. The integrated acceptance scene is the next consumer evidence; an
abstraction will be added only after another real scene repeats its complete
loading, caching, rollback, transition, and shutdown policy.

## Integrated acceptance game

The external [Crank Caverns](https://github.com/ivan-gromov-dev/gopdsdkgame) game
completes the integrated consumer slice. Its repository is currently private, but the
link is retained as the canonical acceptance-game reference. It uses only the
public `gopdsdk` API and integrates lifecycle and input, crank control, owned
bitmaps, sprites and collision queries, bitmap-table animation, sound effect
and streaming music, custom-font UI, primitives and graphics state, offscreen
drawing, direct framebuffer pixels, tile map and camera.

The game begins at an in-game menu with `Play` and `Exit`.
Gameplay can transition back to that menu without restarting the application.
`Exit` asserts `playdate.Launcher` and calls `ExitToLauncher`; Playdate sends
`LifecycleTerminate` before returning to the Launcher, preserving normal owned
resource cleanup. Both generated native adapters implement and forward this
capability.

`examples/navigation` is the deterministic acceptance scene for this slice:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/navigation
```

The example includes generated 1-bit launcher artwork at the three baseline
sizes under `resources/images/launcher` and selects it with
`imagePath=images/launcher`.

Windows SDK 3.1.1 Simulator and physical-device acceptance passed on
2026-08-02: `Play`, B-button return to the menu, `Exit` back to the Launcher,
and the packaged card, icon, and launch image all behaved as intended. The
device path used a conservative-GC hard-float build and USB deployment on COM3.
Crank Caverns deterministically tests its gameplay state and render plans and
owns every native resource explicitly. The complete game establishes the
integrated product boundary. Later diagnostics recorded frame-time
distributions, live-heap growth, and stable native-resource counts over bounded
1,800-frame Simulator
and physical-device runs. The `v0.6.0` regression run subsequently covered extended
soak, termination cleanup observation, and post-run device-log comparison; that
later evidence is not implied by the v0.3.0 release.

## Owned filesystem

Games can optionally assert `playdate.FileSystem` from their callback context.
It exposes owned files with Go `Read`, `Write`, `Seek`, `Flush`, and `Close`
behavior, plus `Stat`, non-recursive `List`, `Mkdir`, `Remove`, and `Rename`.
Read options preserve the official distinction between packaged PDX files,
Data files, and Data-first fallback:

```go
files, ok := any(context).(playdate.FileSystem)
if !ok {
	return playdate.ErrFileUnavailable
}

file, err := files.OpenFile(
	"save.bin",
	playdate.FileReadData|playdate.FileReadPackage,
)
if err != nil {
	return err
}
defer file.Close()
```

Paths are game-relative. Native diagnostics are copied into
`playdate.FileOperationError`; callers can use `errors.Is(err,
playdate.ErrFileIO)` without parsing diagnostic text. `rename` follows the
official overwrite behavior and is the atomic replacement primitive used by
the versioned store. The focused `examples/filesystem` flow passed Windows SDK 3.1.1 Simulator
and physical Playdate execution on 2026-08-02, including write, flush, close,
rename, Data read, stat, list, and recursive remove. Multi-session durability,
interrupted replacement, soak, and memory-growth evidence remain unverified.

## Versioned store

`playdate/store` adds bounded, versioned persistence above `FileSystem`. It
writes a binary envelope with a payload checksum to a sibling temporary file,
flushes and closes it, and first attempts the documented overwriting rename.
On hardware that rejects an existing destination, it falls back to a recoverable
`final → backup`, `temporary → final` swap. `Load` recovers the backup if power
was lost between those operations, and never selects a stale temporary file.

```go
save, err := store.New(files, store.Config{
	Path:        "save.bin",
	Version:     2,
	MaximumSize: 4096,
	Migrations: []store.VersionMigration{
		{From: 1, Migrate: migrateVersion1},
	},
})
if err != nil {
	return err
}
payload, err := save.Load()
```

Each migration advances exactly one version and a successfully migrated value
is atomically rewritten at the current version before it is returned. Callers
choose their own payload encoding; the SDK does not impose JSON. Pure-Go unit
tests cover first save, replacement, migrations, future versions, corruption,
size bounds, interrupted rename, short writes, and preservation of the last
valid value.

The `examples/persistence` flow passed Windows SDK 3.1.1 Simulator and physical
Playdate execution on 2026-08-02, displaying `STORE OK` after save,
migration, replacement, and reload. The first hardware run exposed that device
`rename` rejected an existing destination despite the documented overwrite
contract; it preserved both the valid final file and completed temporary file.
The backup-swap fallback was then added, passed the conservative-GC device gate
at 267,116 bytes of static RAM and a 30,269-byte PDX, and passed repeated USB
deployment and physical execution. Cross-launch durability, power-loss injection,
soak, and memory-growth evidence remain unverified.

## System menu and localization

Games can optionally assert `playdate.SystemMenu` to add owned action,
checkmark, and options items. Items expose their SDK title and value, support
idempotent removal, retain callbacks until removal, and are automatically
removed after `LifecycleTerminate`. `playdate.Localization` exposes only the
system language and `.strings` lookup; a missing key is reported explicitly so
fallback text stays in game code.

The `examples/systemmenu` consumer combines these capabilities with the
versioned store, persisting its checkmark and option values while localizing the
visible labels. On 2026-08-02 it passed Windows SDK 3.1.1 Simulator execution, the
conservative device gate at 268,932 bytes of static RAM and a 35,674-byte PDX,
USB deployment, and physical Playdate execution. The menu callbacks changed
both settings and their values survived a game restart. Extended
conservative-GC soak, memory-growth measurement, and post-run device-log
inspection remain unverified.

## Device and system status

Games can optionally assert `playdate.Accelerometer`, `playdate.PowerMonitor`,
and `playdate.SystemPreferences`. Accelerometer sampling requires explicit
enablement and is disabled automatically after lifecycle termination. Power and
preference access is read-only. The existing `playdate.Launcher` remains the
only exit-to-launcher API.

The `examples/systemstatus` consumer displays motion, battery, power, volume,
reduce-flashing, timezone, and clock-format values and exits through the
Launcher when B is pressed. On 2026-08-02 it passed deterministic tests, SDK
3.1.1 Simulator build and launch, and the conservative device gate at 283,884
bytes of static RAM and a 55,193-byte PDX, including USB deployment and physical
Playdate execution. Hardware observation confirmed accelerometer, battery,
volume, timezone, clock format, reduce-flashing, and `NONE`/`USB` power states.
The first device run exposed direct float-return ABI corruption for battery and
volume; passing their IEEE-754 bits across the TinyGo/C boundary corrected it.
`CHARGE` and `SCREWS` power states, extended conservative-GC soak, memory-growth
measurement, and post-run device-log inspection remain unverified.

## Optional online and debug facilities

`playdate.Scoreboards` provides bounded asynchronous board discovery, score
submission, personal-best retrieval, and score-list retrieval without implying
general networking or multiplayer support. `playdate.DebugMessages` separately
provides a bounded FIFO for Simulator `!msg` and device serial `msg` diagnostics. Scoreboard
completions copy SDK-owned data into a fixed four-slot queue and deliver game
callbacks at the next update boundary. Both facilities suppress delivery after
lifecycle termination.

The focused consumers pass deterministic adapter tests, Simulator builds, and
conservative device packaging. Serial `msg` delivery was confirmed on physical
hardware. On 2026-08-17 the scoreboard scene delivered authentication-related
board-discovery and score-submission failures and an unregistered-player
personal-best failure in the official Windows SDK 3.1.1 Simulator, remaining
responsive across sequential requests. The same artifact was installed and
launched on a physical Playdate, where user-confirmed interaction covered board
discovery, score submission, personal-best retrieval, and score-list retrieval
while remaining responsive. Successful configured-board responses and
pending-request termination remain unverified because they require external
board configuration. The final P11 soak, bounded-memory, memory-growth, and
unchanged post-run `crashlog.txt` and `errorlog.txt` checks passed by user
confirmation on 2026-08-18.

## Integrated persistence acceptance

The external Crank Caverns consumer now uses only public API to combine the released gameplay and persistence capabilities.
Its versioned checkpoint stores progress, score, best time, sound, and
difficulty; migrations cover four schema generations. The title and pause menus
exercise explicit new/save/load/continue flows, System Menu settings use
localized lookup with game-owned fallback, the HUD reads battery status, and
Exit uses `playdate.Launcher`.

Deterministic tests cover round trips, migration, corrupt payload rejection,
failed-write retry, reload, and new-run reset. Windows Simulator interaction,
the conservative-GC device gate, USB installation, and the device launch
command passed on 2026-08-02. The device artifact used 277,524 bytes of static
RAM and produced a 948,032-byte PDX. Physical multi-session restart/update,
injected power loss, corrupt-save recovery, soak, memory-growth measurement,
and post-run device-log inspection remain unverified.

## Advanced sample playback

Games can capability-assert `playdate.SamplePlayers` from their context and
load an explicitly owned `SamplePlayer`. It preserves the base sound-effect
controls and adds bounded repeats, forward or reverse rates, sample duration,
and playback-position control. Streaming `FilePlayer` values optionally expose
`VariableRatePlayer` for positive pitch/speed changes. Negative file-player
rates return `ErrAudioReverseUnsupported`, matching the official streaming API.
Sample reverse is available for PCM assets, but not ADPCM. `Close` releases
native ownership.

`examples/audio` maps A to three sample repeats, Left/Right to forward and
reverse sample rates, B to streaming-music start/stop, and Up/Down to music
rates from 0.25x through 2x. It redraws only on input or playback-state changes;
reverse sample playback seeks to the sample end before starting.

The runtime and generated Simulator/device ABI paths are regression-tested. On
2026-08-02 the example passed audible interaction in Windows Simulator and on a
physical Playdate after conservative hard-float build, USB installation on
COM3, and launch. The device artifact used 268,940 bytes of static RAM and
produced a 125,340-byte PDX. Extended soak, memory-growth measurement, lifecycle
stress, and post-run device-log inspection remain unverified.

## Owned samples and streaming control

`AudioSamples` creates empty buffers, loads packaged samples, or copies
caller-provided PCM/ADPCM bytes into native-owned storage. `AudioSample.Data`
exposes format, rate, length, and bounded copying through a borrowed view that
expires when its sample closes. `SamplePlayerFactory` attaches an owned sample
to a player without transferring ownership; `SamplePlayerControls` and
`LoopCallbackPlayer` expose frame ranges and callbacks without widening the
base player contract. `StreamingPlayerControls` adds file reload, buffer sizing,
loop ranges, underrun status, and stop-on-underrun behavior.

The separate `examples/samples` game generates mono PCM, verifies its borrowed
metadata view, attaches it to a player, and maps A to repeated range playback
and B to stop. Run it without changing the broader audio acceptance game:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/samples
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/samples
```

Deterministic runtime and generated-adapter tests cover ownership, stale views,
range forwarding, buffering, and underrun control. On 2026-08-09 the example
built with the official Windows SDK 3.1.1; its conservative-GC hard-float build
uses 282,280 bytes of static RAM and produces a 1,346,904-byte ELF and a
53,617-byte PDX. On 2026-08-09 the user confirmed visible initialization and
audible range playback with A, followed by stop with B, in the official Windows
Simulator. Loop-callback behavior, final-package device deployment/execution,
physical-device behavior, soak, memory-growth measurement, and post-run device
logs remain unverified.

## Timed fades and completion

Owned sample and file players optionally expose `CompletionPlayer`; replacing
or clearing its callback releases the previous registration, and `Close`
detaches it before freeing native ownership. Streaming players additionally
expose `FadingPlayer`, whose duration is measured in 44.1 kHz audio frames.
`AudioClock.CurrentAudioTime` returns the same wrapping frame clock through an
optional context capability, allowing fades and game scheduling to share an
exact timebase.

`examples/audio` keeps callbacks bounded to counters and redraw flags. A plays
the repeated sample, while B cycles streaming music through play, a half-second
fade, and stop; the scene displays completion counters and the clock sampled on
input. Unit tests cover callback replacement/lifetime, one-shot fade callbacks,
validation, clock forwarding, and generated Simulator/device bridge symbols.
Official Windows Simulator and hard-float device builds pass; the device
artifact uses 269,876 bytes of static RAM and produces a 130,127-byte PDX.
On 2026-08-02 the scene passed audible sample completion, the half-second music
fade, `Done S/F` callback counters, and an advancing audio-clock display in
Windows Simulator and on a physical Playdate. Installation through COM3 and
device launch succeeded. Extended soak, memory-growth measurement, lifecycle
stress, and post-run device-log inspection remain unverified.

## Routing, synthesizers, and signals

Games can capability-assert `AudioChannels`, `AudioOutputs`, and `Synthesizers`
without widening the base `Context`. `AudioOutputs` exposes the borrowed default
channel, current headphone/headset-microphone state, and headphone/speaker
activation. Explicitly owned channels route sample, file, and synth sources and
expose channel volume and pan. A channel's borrowed post-effects output can be
routed into another channel for nested submixes; closing the owner detaches and
expires that output, and routing cycles are rejected. Borrowed dry and wet level
signals expose channel volume before and after effects to the existing
modulation graph without transferring native ownership. Synths support native waveforms,
ADSR parameters, transpose, audio-clock note scheduling, and frequency or
amplitude modulation by owned LFOs, envelopes, and control-signal timelines.

`Synth` also exposes curvature, velocity sensitivity, and note-range rate
scaling for its internal note-triggered envelope. LFOs expose distinct current
and initial phase, reproducible sample-and-hold random seeding, and continuous
global update. The focused `examples/synthesis` scene routes its synth bus into
a master bus, uses dry and wet levels to modulate master pan and volume,
configures the completed LFO controls, and uses a deliberately long
attack and decay so the curvature difference is audible: use Left/Right to
select the curve, then press A to play a fresh note.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/synthesis
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/synthesis
```

On 2026-08-18 the updated scene passed deterministic tests, the official
Windows SDK 3.1.1 Simulator build and launch, and the TinyGo 0.41.1 conservative
hard-float device build. The final device artifact uses 285,856 bytes of static
RAM and produces a 1,539,220-byte ELF and a 61,460-byte PDX; COM3 installation
and launch pass. User-confirmed Simulator and physical-device interaction
covered audible nested routing, dry/wet level-driven pan and volume modulation,
note playback, curvature control, and responsive behavior. The physical
conservative-GC soak, bounded memory growth, and unchanged post-run
`crashlog.txt` and `errorlog.txt` also passed by user confirmation.

Two additional focused scenes cover custom audio without adding more controls
to `examples/audio`. `examples/callbackpcm` continuously renders a sine wave
through a bounded stereo `PCMCallbackSource`: Left plays 220 Hz in the left
channel, Right plays 660 Hz in the right channel, and A
deliberately starves the ring so silence and the native underrun counter are
observable. `examples/generatorsynth` uses a native custom `GeneratorSynth`: B
plays its root voice, A plays an overlapping C-major chord through copied
instrument voices, and Left/Right selects sine/square timbre through generator
parameter 0.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/callbackpcm
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/generatorsynth
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/callbackpcm
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/generatorsynth
```

Both callbacks execute on the frame-update goroutine. Native audio callbacks
only consume fixed 4,096-frame rings and emit silence when empty; no game
callback or slice crosses onto the audio thread. The 4,095 usable frames cover
two standard 30 FPS intervals at 44.1 kHz. Callback PCM sources have four native
slots; custom synth generators have eight native userdata/voice slots with
independent rings, including instrument copies. Exhausting either bounded pool
rejects new native construction instead of allocating from the audio callback.

Routing and modulation edges do not transfer endpoint ownership. Closing a
source or signal detaches every retained edge before freeing it; closing a
channel detaches its sources and borrowed output without closing upstream
owned sources. Unit tests cover duplicate
attachments, close ordering, invalid parameters, graph forwarding through
`NewApplication`, and generated Simulator/device bridge symbols.

The audio acceptance scene polls `AudioOutputState` while running and displays
`Output H/M` as connection state. This makes headphone insertion/removal visible
without requiring an audio-thread callback.

`examples/audio` exercises the complete audio graph. A starts an indefinite
scheduled synth note and release schedules `NoteOff`; B plays the routed sample
and A+B controls routed file music. Left/Right cycles all eight synth waveforms,
Up/Down cycles no modulation, amplitude/frequency LFO, envelope, and control
signal. A+Left/Right cycles all seven LFO shapes, B+Left/Right cycles transpose,
B+Up/Down changes channel volume, and the crank controls shared channel pan.

On 2026-08-02 this matrix passed audible interaction in the official Windows
Simulator and on a physical Playdate. The conservative hard-float device
artifact uses 273,456 bytes of static RAM and produces a 146,771-byte PDX; USB
installation through COM3 and launch succeeded. macOS/Linux SDK integration,
extended soak, memory-growth measurement, lifecycle stress, and post-run
device-log inspection remain unverified.

## Instruments, sequences, and effects

Games can capability-assert `Sequencers` and `AudioEffects`. Instruments retain
voice-range attachments without taking ownership of synths; tracks likewise do
not own instruments, and sequence slots do not own tracks. Tracks support note
and MIDI-controller events, while sequences support MIDI loading, tempo, loops,
time, dynamic track construction, and bounded one-shot completion callbacks.

Channels accept owned two-pole filters, bit crushers, ring modulators, delay
lines, and overdrive processors. Effect mix and parameter modulators are
explicit. Closing either endpoint detaches its graph edges before releasing the
native handle.

`examples/audio` creates a dynamic four-note sequence and routes its instrument,
sample player, streaming file player, and synth through one channel containing
all five effect types. B starts or fades music, B+Up plays the completion sample,
B+Left/Right selects the active effect, A+Up/Down starts or stops the sequence,
Up/Down selects modulation, A+Left/Right selects the LFO, and A+B+Left/Right
selects the synth waveform. Arpeggiator LFOs use the explicit `SetArpeggiation`
steps `0, 4, 7, 12` and select frequency modulation.

Unit tests and the official Windows Simulator and conservative hard-float device
builds pass. On 2026-08-03 the full sequence, completion-counter, routing,
effect, synth-waveform, and modulation matrix passed audible interaction in the
Windows Simulator and on a physical Playdate after USB installation through
COM3. The accepted device artifact uses 278,900 bytes of static RAM and produces
a 168,558-byte PDX. Device audio-thread completions enter a bounded native FIFO
and are delivered to Go on the next update frame. Extended soak, memory-growth
measurement, lifecycle stress, post-run device-log inspection, and macOS/Linux SDK
integration remain unverified.

## Microphone input

Games capability-assert `Microphones` without widening `Context`. Permission is
explicitly pending, denied, or granted; recording selects automatic, internal,
or headset input and returns an owned recorder. Native sample views expire when
their callback returns. `MicrophoneSamples.CopyTo` copies only into the game's
bounded destination and never retains that buffer.

`examples/microphone` requests permission, starts recording, and displays the
live peak and delivered-block count. A stops or restarts the recorder. B saves
up to one second as `microphone.wav` in Data and audibly plays the same capture
through native-owned copied PCM:

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/microphone
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/microphone
```

On 2026-08-03 permission, changing peak/block counters, stop/start, WAV save,
and audible playback passed in the official Windows Simulator and on a physical
Playdate installed through COM3. Device input reaches Go through a bounded
native FIFO on update frames. The accepted hard-float artifact uses 282,824
bytes of static RAM and produces a 50,601-byte PDX. Denial/revocation,
long-run overflow/memory measurement, lifecycle stress, post-run device-log
inspection, and macOS/Linux SDK integration remain unverified.

## Bitmap composition

Games capability-assert `BitmapCompositor` for rotated/scaled bitmap drawing
and callback-scoped stencils. Transforms reject non-finite values and
non-positive scales. `WithStencil` borrows a live bitmap only until its callback
returns, clears the native stencil on callback errors, rejects nesting, and
requires tiled stencil widths to be multiples of 32 pixels.

`examples/composition` builds an owned 64×64 source and a screen-aligned 400×240
stencil bitmap through `OffscreenGraphics`. Stencils use framebuffer
coordinates, so the mask is positioned around the right-hand draw target. SDK
3.1.1 rendered a direct stencil-plus-rotation path only at cardinal angles in
both the Go scene and an equivalent official Lua diagnostic. The accepted
portable path rotates into a transparent 400×240 offscreen canvas, then draws
that canvas through the screen stencil.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/composition
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/composition
```

The portable contract and generated Simulator/device bridge paths are
unit-tested. Official Windows SDK 3.1.1 Simulator compilation and conservative
hard-float device build pass. Visual interaction passed in the Simulator and on
a physical Playdate after USB deployment through COM3 on 2026-08-08. The
accepted device build used 275,312 bytes of static RAM and produced a
36,970-byte PDX. Performance, bounded-memory, soak, and post-run device-log
regression checks pass on the verified Windows profile.

## Video

`examples/video` owns a generated four-second, 48-frame PDV fixture and matching
audio track. It exercises metadata, validated frame playback, explicit video
and audio cleanup, and screen/offscreen render-target switching. A pauses, B
changes the target, and Left/Right step through frames.

On 2026-08-08 the complete interaction passed visual and audible acceptance in
the official Windows Simulator and on a physical Playdate. The accepted
hard-float device build used 279,880 bytes of static RAM and produced a
976,532-byte ELF and a 227,458-byte PDX. Performance, bounded-memory, soak, and
post-run device-log regression checks pass on the verified Windows profile.

```sh
go run ./cmd/gopdsdk run --sdk /path/to/PlaydateSDK ./examples/video
go run ./cmd/gopdsdk run device --memory conservative --sdk /path/to/PlaydateSDK ./examples/video
```

## Development and CI

Run the repository checks with:

```sh
gofmt -w cmd examples internal playdate
go test ./...
go vet ./...
git diff --check
go run ./cmd/gopdsdk doctor
```

`go test ./...` includes a CLI acceptance test that builds `gopdsdk`, creates a
standalone module in a path containing spaces, compiles it through its local
`replace`, and requests both Simulator and device dry-run plans from inside the
consumer module. GitHub Actions repeats this suite on Windows, macOS, and Linux;
Linux additionally runs the race detector.

Docker is not part of the supported-host matrix. It would provide another Linux environment,
not Windows or macOS semantics. A pinned Linux image becomes useful when it can
legally receive the official SDK and exercise the real Simulator/device build
toolchain without pretending to verify GUI or USB behavior.

## Current limitations

- The device Go-profile table above is authoritative. The current production
  linker still rejects goroutines, channels, `select`, reflection outside the
  documented bounded subset, finalizers, and application cgo. Normal-return
  `defer` is available only with conservative GC.
- OOM and panic are deterministic fail-stop traps, not recoverable errors.
- `recover` is unsupported; accepted future `defer` enablement is limited to
  normal returns and will not add panic unwinding.
- macOS and Linux official SDK integration remains unverified.
- Graphics cover clear/text, bitmaps, sprites, animation, custom fonts,
  callback-scoped framebuffer access, and drawing into owned bitmaps. Audio
  covers sound effects/file players, advanced sample controls,
  timed fades/completion callbacks, owned routing, waveform synths and
  modulation signals, instruments/sequencing/effects, and bounded
  microphone input with Simulator and physical-device acceptance.
