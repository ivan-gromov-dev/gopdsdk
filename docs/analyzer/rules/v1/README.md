# Analyzer rules v1

This reference is generated from the versioned analyzer contract inventory.
Stable rules are enabled by the default profile; experimental rules require
`--profile experimental` or `--profile deep`.

- [`application-entry`](application-entry.md) — application entry does not match the generated runtime wiring.
- [`application-lifecycle-shape`](application-lifecycle-shape.md) — a game lifecycle method cannot receive runtime events.
- [`application-nested-scheduler`](application-nested-scheduler.md) — a scheduler cannot re-enter its update.
- [`application-nested-stencil`](application-nested-stencil.md) — stencil callbacks cannot be nested.
- [`application-scheduler-update-boundary`](application-scheduler-update-boundary.md) — scheduler update runs outside Game.Update.
- [`application-sprite-callback-close`](application-sprite-callback-close.md) — an active sprite callback cannot close its participating sprites.
- [`application-termination-resource-leak`](application-termination-resource-leak.md) — a successfully acquired resource is lost on a visible termination path.
- [`capability-impossible-assertion`](capability-impossible-assertion.md) — an optional capability assertion is known to fail on this path.
- [`capability-redundant-check`](capability-redundant-check.md) — a capability check has a statically known result.
- [`capability-unchecked-assertion`](capability-unchecked-assertion.md) — an optional Context capability is asserted without a check.
- [`capability-unproven-assertion`](capability-unproven-assertion.md) — a related capability check cannot be connected to this assertion.
- [`capability-video-availability`](capability-video-availability.md) — video capability use is not valid for the selected target contract.
- [`device-build-constraint`](device-build-constraint.md) — host-selected source is excluded by fixed device build constraints.
- [`device-cgo`](device-cgo.md) — application cgo is unavailable in the device profile.
- [`device-channel`](device-channel.md) — channels are unavailable in the device profile.
- [`device-compiler-directive`](device-compiler-directive.md) — a compiler directive aliases an unavailable device symbol.
- [`device-encoding-json`](device-encoding-json.md) — encoding/json is unavailable in the device profile.
- [`device-finalizer`](device-finalizer.md) — finalizers are unavailable in the device profile.
- [`device-fmt`](device-fmt.md) — fmt is unavailable in the device profile.
- [`device-go-assembly`](device-go-assembly.md) — Go assembly cannot supply a TinyGo function body.
- [`device-goroutine`](device-goroutine.md) — goroutines are unavailable in the device profile.
- [`device-panic-cleanup`](device-panic-cleanup.md) — explicit device panic bypasses potentially pending deferred calls.
- [`device-recover`](device-recover.md) — recover cannot recover a device panic.
- [`device-reflect-symbol`](device-reflect-symbol.md) — a reflect symbol is outside the audited device subset.
- [`device-runtime-control`](device-runtime-control.md) — an application runtime-control hook is unavailable on device.
- [`device-select`](device-select.md) — select is unavailable in the device profile.
- [`device-time-runtime`](device-time-runtime.md) — standard-library clocks, sleep, timers, and tickers are unavailable on device.
- [`lifetime-audio-render-buffer-escape`](lifetime-audio-render-buffer-escape.md) — callback-scoped audio render buffer escapes its callback.
- [`lifetime-bitmap-data-escape`](lifetime-bitmap-data-escape.md) — callback-scoped bitmap data escapes its callback.
- [`lifetime-framebuffer-escape`](lifetime-framebuffer-escape.md) — callback-scoped framebuffer data escapes its callback.
- [`lifetime-microphone-samples-escape`](lifetime-microphone-samples-escape.md) — callback-scoped microphone samples escape their callback.
- [`ownership-bitmap-use-after-close`](ownership-bitmap-use-after-close.md) — an owned bitmap is used after successful close.
- [`ownership-borrowed-close`](ownership-borrowed-close.md) — a borrowed handle is closed by a non-owner.
- [`ownership-close-order`](ownership-close-order.md) — owned handles are closed outside their documented dependency order.
- [`ownership-double-close`](ownership-double-close.md) — a local handle is closed more than once.
- [`ownership-resource-leak`](ownership-resource-leak.md) — a local owned handle reaches a return without cleanup or transfer.
- [`ownership-retained-close`](ownership-retained-close.md) — a handle is closed while retained by another live object.
- [`ownership-use-after-close`](ownership-use-after-close.md) — a local SDK handle is used after successful close.
- [`result-invalid-argument`](result-invalid-argument.md) — an SDK argument is definitely outside its accepted contract.
- [`result-menu-image-offset`](result-menu-image-offset.md) — menu image offset is outside the accepted range.
- [`result-sdk-error-comparison`](result-sdk-error-comparison.md) — a wrapped SDK sentinel is compared directly.
- [`result-sdk-error-discarded`](result-sdk-error-discarded.md) — an SDK error result is discarded.
- [`result-sdk-typed-diagnostic`](result-sdk-typed-diagnostic.md) — an SDK typed diagnostic is asserted directly.
- [`result-sdk-value-discarded`](result-sdk-value-discarded.md) — a contract-significant SDK result is discarded.
- [`workspace-manifest`](workspace-manifest.md) — pdxinfo metadata is malformed, conflicting, incomplete, or unsafe.
- [`workspace-missing-resource`](workspace-missing-resource.md) — a statically named SDK resource is absent or has mismatched case.
- [`workspace-module-version`](workspace-module-version.md) — application module does not declare a versioned gopdsdk dependency.
- [`workspace-resource-path`](workspace-resource-path.md) — packaged resource path is non-portable, escaped, excluded, or case-colliding.
