package deviceprobe

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/buildplan"
	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/gomodule"
)

func TestSpritePresentationBridgesExistInBothDeviceProfiles(t *testing.T) {
	for name, bootstrap := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, symbol := range []string{
			"bridgeSpriteSetCenterBits", "bridgeSpriteGetPointBits", "bridgeSpriteSetBoundsBits", "bridgeSpriteRectBits",
			"bridgeSpriteVisible", "bridgeSpriteZIndex", "bridgeSpriteSetImageFlip", "bridgeSpriteImageFlip",
			"bridgeSpriteSetDrawMode", "bridgeSpriteSetOpaque", "bridgeSpriteSetStencilImage", "bridgeSpriteSetStencilPattern",
			"bridgeSpriteClearStencil", "bridgeSpriteSetClipRect", "bridgeSpriteClearClipRect", "bridgeSpriteSetIgnoresDrawOffset",
			"bridgeSpriteSetUpdatesEnabled", "bridgeSpriteUpdatesEnabled", "bridgeSpriteSetCollisionsEnabled", "bridgeSpriteCollisionsEnabled", "bridgeSpriteTag",
			"bridgeNewSpriteTileMap", "bridgeFreeSpriteTileMap", "bridgeSpriteTileMapSetImageTable", "bridgeSpriteTileMapSetSize",
			"bridgeSpriteTileMapSize", "bridgeSpriteTileMapPixelSize", "bridgeSpriteTileMapSetTiles", "bridgeSpriteTileMapSetTile",
			"bridgeSpriteTileMapTile", "bridgeSpriteSetTileMap", "bridgeSpriteTileMap",
		} {
			if !strings.Contains(bootstrap, symbol) {
				t.Errorf("%s bootstrap does not contain %q", name, symbol)
			}
		}
	}
}

func TestRenderDeviceGoModAddsExternalApplicationModule(t *testing.T) {
	gameDir := filepath.Join(t.TempDir(), "game")
	app := applicationInfo{ImportPath: "example.com/game/pkg", Name: "pkg", Dir: filepath.Join(gameDir, "pkg")}
	app.Module = &struct{ Path, Dir, GoVersion string }{Path: "example.com/game", Dir: gameDir, GoVersion: "1.26"}
	got := renderDeviceGoMod(gomodule.Info{Path: "github.com/ivan-gromov-dev/gopdsdk", Root: filepath.Join(t.TempDir(), "sdk"), GoVersion: "1.26"}, app)
	for _, want := range []string{"require example.com/game v0.0.0", "replace example.com/game =>", strconv.Quote(filepath.ToSlash(gameDir))} {
		if !strings.Contains(got, want) {
			t.Errorf("renderDeviceGoMod() does not contain %q:\n%s", want, got)
		}
	}
}

func TestProbeSourceExportsGoEventHandler(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"bridgeLoadVideo", "bridgeVideoRenderFrame", "sdkRuntime.NewVideoPlayer"} {
		if !strings.Contains(source, want) {
			t.Errorf("probe source does not contain %q", want)
		}
	}
	for _, want := range []string{"package main", "//export goEventHandler", "func goEventHandler", "//export goUpdate", "sdkRuntime.NewApplication(app.New(), gameContext, nil)", "application.Handle", "application.Update", "bridgeClear", "bridgeLog", "gopdsdk event error:", "bridgeDrawText", "bridgeCurrentTimeMilliseconds", "bridgeCurrentAudioTime", "bridgeButtons", "bridgeCrankAngleBits", "bridgeCrankDeltaBits", "bridgeCrankDocked", "bridgeFrameDeltaBits", "float32FromBits", "bridgeLoadBitmap", "bridgeNewBitmap", "bridgeFreeBitmap", "bridgeBitmapSize", "bridgeFillBitmap", "bridgeBitmapData", "bridgeCopyBitmap", "bridgeLoadIntoBitmap", "bridgeNewBitmapTable", "bridgeLoadIntoBitmapTable", "bridgeSetBitmapMask", "bridgeGetBitmapMask", "bridgeCheckMaskCollision", "bridgeRotatedBitmapBits", "bridgeCopyDisplayBuffer", "bridgeDrawBitmap", "bridgeDrawScaledBitmapBits", "bridgeDrawRotatedBitmapBits", "bridgeSetStencil", "bridgeClearStencil", "bridgeDisplaySetRefreshRateBits", "bridgeDisplaySetScale", "bridgeDisplaySetMosaic", "bridgeDrawPrimitive", "bridgeSetClipRect", "bridgeSetDrawMode", "bridgeGetFrame", "bridgeMarkUpdatedRows", "sdkRuntime.WithFramebuffer", "sdkRuntime.OwnedBitmapHandle", "bridgePushContext", "bridgePopContext", "sdkRuntime.ValidatePrimitiveGeometry", "bridgeNewSprite", "bridgeSpriteSetBitmap", "bridgeSpriteMoveToBits", "bridgeSpriteMoveByBits", "bridgeSpriteSetVisible", "bridgeSpriteSetZIndex", "bridgeSpriteMarkDirty", "bridgeSpriteMarkDirtyRectBits", "bridgeSetAlwaysRedraw", "bridgeAddDirtyRect", "bridgeSpriteAdd", "bridgeSpriteRemove", "bridgeFreeSprite", "bridgeUpdateAndDrawSprites", "bridgeLoadSoundEffect", "bridgeSamplePlayerPlayBits", "bridgeLoadFilePlayer", "bridgeFilePlayerSetRateBits", "bridgeSoundEffectSetFinishCallback", "bridgeFilePlayerFadeVolumeBits", "sdkRuntime.InvokeAudioCallback", "sdkRuntime.NewSoundEffect", "sdkRuntime.NewSamplePlayer", "sdkRuntime.NewFilePlayer", "bridgeNewAudioChannel", "bridgeNewSynth", "bridgeNewLFO", "bridgeNewEnvelopeBits", "bridgeNewControlSignal", "sdkRuntime.NewAudioChannel", "sdkRuntime.NewSynth", "sdkRuntime.NewLFO", "sdkRuntime.NewEnvelope", "sdkRuntime.NewControlSignal", `"example.com/game"`, "func main()"} {
		if !strings.Contains(source, want) {
			t.Errorf("probe source does not contain %q", want)
		}
	}
}

func TestBothDeviceAdaptersContainDisplayIntrospection(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"bridgeDisplayWidth", "bridgeDisplayHeight", "bridgeDisplayRefreshRateBits", "bridgeDisplayFPSBits"} {
		if !strings.Contains(source, want) {
			t.Errorf("probe source does not contain %q", want)
		}
	}
	for name, bootstrap := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{"display->getWidth", "display->getHeight", "display->getRefreshRate", "display->getFPS"} {
			if !strings.Contains(bootstrap, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
	}
}

func TestProbeSourceContainsCollisionBridge(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"bridgeSpriteSetCollideRectBits", "bridgeSpriteMoveWithCollisionsBits", "bridgeSpriteCheckCollisionsBits", "bridgeQuerySpritesAtPointBits", "bridgeQuerySpritesInRectBits", "bridgeQuerySpritesAlongLineBits", "bridgeQuerySpriteInfoAlongLineBits", "bridgeSpriteRemoveMany", "bridgeRemoveAllSprites", "bridgeSpriteCount", "bridgeResetCollisionWorld", "sdkRuntime.NativeCollision", "bridgeFreeList(list)"} {
		if !strings.Contains(source, want) {
			t.Errorf("probe source does not contain %q", want)
		}
	}
	if strings.Contains(source, "defer bridgeFreeList") {
		t.Fatal("probe source retains unsupported TinyGo defer runtime for native sprite lists")
	}
}

func TestBothDeviceAdaptersContainFilesystemBridge(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"sdkPlaydate.FileSystem", "sdkRuntime.NewOwnedFile", "bridgeFileOpen", "bridgeFileList", "bridgeFileRename", "copiedCString(bridgeFileListItem", "func fileErrorMessage() string { pointer:=bridgeFileError();if pointer==0{return \"\"};return copiedCString(pointer) }"} {
		if !strings.Contains(source, want) {
			t.Errorf("probe source does not contain %q", want)
		}
	}
	for _, want := range []string{"sdkPlaydate.SystemMenu", "sdkPlaydate.Localization", "goMenuCallback", "sdkRuntime.NewOwnedMenuItem", "bridgeLocalizedText", "bridgeFree(pointer)"} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated source missing %q", want)
		}
	}
	if strings.Contains(source, "defer bridgeFileListFree") {
		t.Fatal("probe source retains unsupported TinyGo defer runtime for filesystem lists")
	}
	for name, bootstrap := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{"file->open", "file->read", "file->listfiles", "file->rename", "bridgeFileListFree"} {
			if !strings.Contains(bootstrap, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
	}
}

func TestBothDeviceAdaptersContainOnlineAndDebugBridges(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"sdkPlaydate.Scoreboards", "sdkPlaydate.DebugMessages", "goSerialMessage", "goScoreCallback", "sdkRuntime.NewScoreboardService", "sdkRuntime.ScoreboardCallbackQueue", "scoreboardCallbacks.Drain()", "scoreboardService.Terminate()", "bridgeGetScores"} {
		if !strings.Contains(source, want) {
			t.Errorf("probe source does not contain %q", want)
		}
	}
	for name, bootstrap := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{"setSerialMessageCallback(bridgeSerialMessage)", "scoreboards->addScore", "scoreboards->getPersonalBest", "scoreboards->getScoreboards", "scoreboards->getScores", "scoreboards->freeScoresList"} {
			if !strings.Contains(bootstrap, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
	}
}

func TestBothDeviceAdaptersContainSystemStatusBridge(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"bridgeSetAccelerometerEnabled", "bridgeAccelerometer", "bridgePowerStatus", "bridgeBatteryPercentageBits", "bridgeBatteryVoltageBits", "bridgeSystemVolumeBits", "float32FromBits(bridgeBatteryPercentageBits())", "bridgeReduceFlashing", "bridgeTimezoneOffsetSeconds", "bridgeUses24HourTime", "bridgeDefaultAudioChannel", "bridgeAudioOutputState", "bridgeSetAudioOutputsActive", "sdkRuntime.DefaultAudioChannel"} {
		if !strings.Contains(source, want) {
			t.Errorf("probe source does not contain %q", want)
		}
	}
	for name, bootstrap := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{"setPeripheralsEnabled", "getAccelerometer", "getPowerStatus", "getBatteryPercentage", "getBatteryVoltage", "getVolume", "getReduceFlashing", "getTimezoneOffset", "shouldDisplay24HourTime"} {
			if !strings.Contains(bootstrap, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
	}
}

func TestBothDeviceAdaptersContainP101SystemControls(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{
		"sdkPlaydate.SystemControls", "bridgeGetLaunchArgs", "bridgeRestartGame",
		"bridgeSetMenuImage", "bridgeSetAutoLockDisabled", "bridgeSetCrankSoundsDisabled",
		"bridgeSetButtonCallback", "bridgePollButtonEvent", "bridgeButtonCallbackOverflow",
		"for buttonCallback != nil", "sdkPlaydate.ButtonEvent",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("device Go source does not contain %q", want)
		}
	}
	for name, bootstrap := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{
			"system->getLaunchArgs", "system->restartGame", "system->setMenuImage",
			"system->setAutoLockDisabled", "system->setCrankSoundsDisabled",
			"system->setButtonCallback", "#define BRIDGE_BUTTON_EVENT_CAPACITY 64",
			"bridgeButtonDropped++", "bridgePollButtonEvent", "bridgeButtonCallbackOverflow",
		} {
			if !strings.Contains(bootstrap, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
		if strings.Contains(bootstrap, "goButtonCallback") {
			t.Errorf("%s bootstrap enters Go from the native button callback", name)
		}
	}
}

func TestBothDeviceAdaptersContainP102SystemEnvironment(t *testing.T) {
	source := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{
		"sdkPlaydate.SystemEnvironment", "bridgeCurrentEpochTime", "bridgeEpochToDateTime",
		"bridgeDateTimeToEpoch", "bridgeResetElapsedTime", "bridgeElapsedTimeBits",
		"bridgeSystemInfo", "sdkRuntime.ValidateDateTime",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("device Go source does not contain %q", want)
		}
	}
	for name, bootstrap := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{
			"system->getSecondsSinceEpoch", "system->convertEpochToDateTime",
			"system->convertDateTimeToEpoch", "system->resetElapsedTime",
			"system->getElapsedTime", "system->getSystemInfo",
		} {
			if !strings.Contains(bootstrap, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
		if strings.Contains(bootstrap, "bridgeFrameDeltaBits(void) { float value = activePlaydate->system->getElapsedTime()") {
			t.Errorf("%s frame delta still resets the public elapsed timer", name)
		}
	}
}

func TestBootstrapInitializesRuntimeOnce(t *testing.T) {
	for _, want := range []string{"runtime.run", "runtime.alloc", "activePlaydate->system->realloc(NULL, size)", "event == kEventInit && !booted", "runtimeRun();", "goEventHandler(playdate, event, arg)"} {
		if !strings.Contains(bootstrapSource, want) {
			t.Errorf("bootstrapSource does not contain %q", want)
		}
	}
	if strings.Contains(bootstrapSource, "runtime.preinit") || strings.Contains(bootstrapSource, "runtimePreinit") {
		t.Fatal("bootstrapSource must not call bare-metal preinit after the Playdate ELF loader")
	}
	activate := strings.Index(bootstrapSource, "activePlaydate = playdate;")
	run := strings.Index(bootstrapSource, "runtimeRun();")
	if activate < 0 || run < activate {
		t.Fatalf("bootstrap order activate/run = %d/%d", activate, run)
	}
}

func TestP93DeviceABIBridges(t *testing.T) {
	for name, source := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{"bridgeSamplePlayerSetRateModulator", "bridgeFilePlayerSetRateModulator"} {
			if !strings.Contains(source, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
	}
	for _, want := range []string{"bridgeAudioChannelSetModulator", "bridgeAudioChannelOutput", "bridgeAudioChannelDryLevelSignal", "bridgeAudioChannelWetLevelSignal", "bridgeLFOSetStartPhaseBits", "bridgeLFOSetRandomSeed", "bridgeLFOSetGlobal", "bridgeSynthSetWavetable", "bridgeSynthSetParameterBits", "bridgeTrackInspect", "bridgeTrackMetric", "bridgeTrackNoteAtBits", "bridgeSequenceGetTrack", "bridgeSequenceCurrentStep", "bridgeSequenceAllNotesOff", "bridgeNewOnePole", "bridgeNewPCMCallbackSource", "bridgeNewGeneratorSynth"} {
		if !strings.Contains(bootstrapSource, want) {
			t.Errorf("device bootstrap does not contain %q", want)
		}
		if !strings.Contains(applicationSourceTemplate, want) {
			t.Errorf("device application does not contain %q", want)
		}
	}
	for _, want := range []string{"bridgePCMRender", "BRIDGE_PCM_RING_FRAMES", "bridgeGeneratorRender", "bridgeGeneratorCopy", "BRIDGE_GENERATOR_VOICE_COUNT"} {
		if !strings.Contains(bootstrapSource, want) {
			t.Errorf("device bootstrap does not contain %q", want)
		}
	}
}

func TestConservativeBootstrapInitializesRuntimeBoundary(t *testing.T) {
	for _, want := range []string{"runtime.run", "runtime.stackTop", "playdateRuntimeSCB", "runtimeSCB = runtimeSCBShadow", "prepareRuntimeBoundary();", "event == kEventInit && !booted", "runtimeRun();", "goEventHandler(playdate, event, arg)"} {
		if !strings.Contains(conservativeBootstrapSource, want) {
			t.Errorf("conservativeBootstrapSource does not contain %q", want)
		}
	}
	if strings.Contains(conservativeBootstrapSource, "runtime.preinit") || strings.Contains(conservativeBootstrapSource, "runtimePreinit") {
		t.Fatal("conservativeBootstrapSource must not call bare-metal preinit after the Playdate ELF loader")
	}
	activate := strings.Index(conservativeBootstrapSource, "activePlaydate = playdate;")
	shadow := strings.Index(conservativeBootstrapSource, "runtimeSCB = runtimeSCBShadow;")
	boundary := strings.Index(conservativeBootstrapSource, "prepareRuntimeBoundary();")
	run := strings.Index(conservativeBootstrapSource, "runtimeRun();")
	if activate < 0 || shadow < activate || boundary < shadow || run < boundary {
		t.Fatalf("bootstrap order activate/shadow/boundary/run = %d/%d/%d/%d", activate, shadow, boundary, run)
	}
}

func TestBothDeviceBootstrapsContainFramebufferAndOffscreenBridges(t *testing.T) {
	for name, source := range map[string]string{"hard-float": bootstrapSource, "conservative": conservativeBootstrapSource} {
		for _, want := range []string{"graphics->getFrame", "graphics->markUpdatedRows", "graphics->pushContext", "graphics->popContext", "graphics->drawRotatedBitmap", "graphics->setStencilImage", "graphics->setStencil(NULL)", "graphics->fillPolygon", "graphics->drawRoundRect", "graphics->fillRoundRect", "graphics->setLineCapStyle", "graphics->setBackgroundColor", "graphics->setScreenClipRect"} {
			if !strings.Contains(source, want) {
				t.Errorf("%s bootstrap does not contain %q", name, want)
			}
		}
	}
}

func TestDeviceMicrophoneDefersAudioThreadDeliveryToUpdate(t *testing.T) {
	goSource := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"bridgePollMicrophonePermission", "bridgePollMicrophoneSamples", "microphonePollBuffer"} {
		if !strings.Contains(goSource, want) {
			t.Errorf("device Go source does not contain %q", want)
		}
	}
	for _, want := range []string{"bridgeMicrophoneBuffer[2048]", "bridgePollMicrophoneSamples", "bridgeMicrophoneWrite"} {
		if !strings.Contains(bootstrapSource, want) {
			t.Errorf("device bootstrap does not contain %q", want)
		}
	}
	if strings.Contains(bootstrapSource, "return goMicrophoneSamples") {
		t.Fatal("device audio thread must not enter Go directly")
	}
}

func TestDevicePCMSampleCopiesCallerDataIntoNativeOwnership(t *testing.T) {
	goSource := renderProbeSource("github.com/ivan-gromov-dev/gopdsdk", "example.com/game")
	for _, want := range []string{"sdkPlaydate.PCMPlayers", "bridgeNewPCMPlayer(&samples[0]"} {
		if !strings.Contains(goSource, want) {
			t.Errorf("device Go source does not contain %q", want)
		}
	}
	for _, want := range []string{"newSampleFromData", "memcpy(copy,samples", "kSound16bitMono"} {
		if !strings.Contains(bootstrapSource, want) {
			t.Errorf("device bootstrap does not contain %q", want)
		}
	}
}

func TestBootstrapReservesBoundedAlignedHeap(t *testing.T) {
	for _, want := range []string{"section(\".bss.playdate_runtime_heap\")", "aligned(16)", "playdateRuntimeHeap[256 * 1024]"} {
		if !strings.Contains(conservativeBootstrapSource, want) {
			t.Errorf("conservativeBootstrapSource does not contain %q", want)
		}
	}
}

func TestBootstrapDelegatesUpdateAndGraphicsToGo(t *testing.T) {
	for _, source := range []string{bootstrapSource, conservativeBootstrapSource} {
		for _, want := range []string{"result = goEventHandler(playdate, event, arg);", "setUpdateCallback(bridgeUpdate, playdate)", "return goUpdate();", "void bridgeClear(void)", "void bridgeDrawText", "graphics->drawText", "graphics->setTextTracking", "graphics->getTextTracking", "graphics->setTextLeading", "graphics->drawTextInRect", "graphics->getTextHeightForMaxWidth", "graphics->getFontPage", "graphics->getPageGlyph", "graphics->getGlyphKerning", "bridgeCurrentTimeMilliseconds", "system->getCurrentTimeMilliseconds()", "bridgeCurrentAudioTime", "sound->getCurrentTime()", "fileplayer->fadeVolume", "sampleplayer->setFinishCallback", "bridgeExitToLauncher", "system->exitToLauncher()", "bridgeButtons", "getButtonState", "bridgeFloatBits", "bridgeCrankAngleBits", "getCrankAngle", "bridgeCrankDeltaBits", "getCrankChange", "bridgeCrankDocked", "isCrankDocked", "bridgeFrameDeltaBits", "getElapsedTime", "resetElapsedTime"} {
			if !strings.Contains(source, want) {
				t.Errorf("bootstrap source does not contain %q", want)
			}
		}
	}
}

func TestTargetUsesHardFloatCortexM7(t *testing.T) {
	for _, want := range []string{"cortex-m7", "thumbv7em-unknown-unknown-eabihf", `"relocation-model": "pic"`, `"qemu"`, "-mfloat-abi=hard", "-mfpu=fpv5-sp-d16"} {
		if !strings.Contains(targetSource, want) {
			t.Errorf("targetSource does not contain %q", want)
		}
	}
	for _, forbidden := range []string{"nucleof722ze", "stm32f722", "stm32f7", `"stm32"`} {
		if strings.Contains(targetSource, forbidden) {
			t.Errorf("targetSource contains board-specific tag %q", forbidden)
		}
	}
}

func TestAdapterDefinesInterruptHooks(t *testing.T) {
	for _, want := range []string{"tinygo_scanCurrentStack:", "push {r4-r11, lr}", "bl tinygo_scanstack", "add sp, #32", "pop {pc}", "DisableInterrupts:", "mrs r0, primask", "EnableInterrupts:", "msr primask, r0", "SemihostingCall:", "_exit:", "_kill:", "_getpid:", ".thumb_func"} {
		if !strings.Contains(adapterSource, want) {
			t.Errorf("adapterSource does not contain %q", want)
		}
	}
}

func TestFirstLine(t *testing.T) {
	if got, want := firstLine("first\r\nsecond\n"), "first"; got != want {
		t.Fatalf("firstLine() = %q, want %q", got, want)
	}
}

func TestRequireNonEmptyFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "pdex.bin")
	if err := os.WriteFile(path, []byte("device binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireNonEmptyFile(path); err != nil {
		t.Fatalf("requireNonEmptyFile() error = %v", err)
	}
}

func TestRequireNonEmptyFileRejectsEmptyFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "pdex.bin")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireNonEmptyFile(path); err == nil {
		t.Fatal("requireNonEmptyFile() error = nil, want empty-file error")
	}
}

func TestSummarizeOutput(t *testing.T) {
	if got, want := summarizeOutput("Playdate detected\r\nInstalled DeviceProbe.pdx\r\n"), "Playdate detected Installed DeviceProbe.pdx"; got != want {
		t.Fatalf("summarizeOutput() = %q, want %q", got, want)
	}
	if got, want := summarizeOutput("\r\n"), "installed by pdutil"; got != want {
		t.Fatalf("summarizeOutput(empty) = %q, want %q", got, want)
	}
}

func TestSummarizeRunOutput(t *testing.T) {
	if got, want := summarizeRunOutput("Playdate detected\r\nCommand sent.\r\n"), "Playdate detected Command sent."; got != want {
		t.Fatalf("summarizeRunOutput() = %q, want %q", got, want)
	}
	if got, want := summarizeRunOutput("\r\n"), "launch command sent by pdutil"; got != want {
		t.Fatalf("summarizeRunOutput(empty) = %q, want %q", got, want)
	}
}

func TestStrongUndefinedSymbolsIgnoresWeakReferences(t *testing.T) {
	output := "         w __gnu_Unwind_Find_exidx\n         U requiredSymbol\n"
	got := strongUndefinedSymbols(output)
	if len(got) != 1 || got[0] != "requiredSymbol" {
		t.Fatalf("strongUndefinedSymbols() = %v, want [requiredSymbol]", got)
	}
}

func TestDirectoryFileSizeSumsNestedRegularFiles(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "one.bin"), []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "two.bin"), []byte("567"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := directoryFileSize(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Fatalf("directoryFileSize() = %d, want 7", got)
	}
}

func TestUnsupportedRuntimeSymbolsRejectsUnsupportedSubset(t *testing.T) {
	output := "00003ae0 t runtime.setupDeferFrame\n000056b8 t runtime._recover\n00006000 t runtime.chanSend\n00006100 t runtime.SetFinalizer\n00006200 t reflect.Value.Call\n00003298 t runtime/interrupt.In\n"
	got := unsupportedRuntimeSymbols(output, buildplan.DeviceMemoryNone)
	want := []string{"runtime.setupDeferFrame", "runtime._recover", "runtime.SetFinalizer", "reflect.Value.Call", "runtime.chan", "runtime/interrupt.In"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unsupportedRuntimeSymbols() = %v, want %v", got, want)
	}
	if got := unsupportedRuntimeSymbols("00000100 t runtime.run\n", buildplan.DeviceMemoryNone); len(got) != 0 {
		t.Fatalf("unsupportedRuntimeSymbols(safe) = %v, want none", got)
	}
	if got := unsupportedRuntimeSymbols("00000100 t runtime/interrupt.In\n", buildplan.DeviceMemoryConservative); len(got) != 0 {
		t.Fatalf("unsupportedRuntimeSymbols(adapted interrupt query) = %v, want none", got)
	}
	if got := unsupportedRuntimeSymbols("00000100 t runtime.setupDeferFrame\n", buildplan.DeviceMemoryConservative); len(got) != 0 {
		t.Fatalf("unsupportedRuntimeSymbols(conservative defer) = %v, want none", got)
	}
	if got := unsupportedRuntimeSymbols("00000100 t runtime.setupDeferFrame\n00000110 t runtime._recover\n", buildplan.DeviceMemoryConservative); strings.Join(got, ",") != "runtime._recover" {
		t.Fatalf("unsupportedRuntimeSymbols(conservative recover) = %v, want runtime._recover", got)
	}
	if got := unsupportedRuntimeSymbols("00000100 t internal/reflectlite.Value.Kind\n", buildplan.DeviceMemoryConservative); len(got) != 0 {
		t.Fatalf("unsupportedRuntimeSymbols(internal reflectlite) = %v, want none", got)
	}
	if got := unsupportedRuntimeSymbols("00000100 t runtime.chanLen\n00000110 t runtime.chanCap\n", buildplan.DeviceMemoryConservative); len(got) != 0 {
		t.Fatalf("unsupportedRuntimeSymbols(channel queries) = %v, want none", got)
	}
}

func TestUnsupportedReflectionSymbolsAllowsOnlyAuditedOperations(t *testing.T) {
	output := "00000100 T reflect.TypeOf\n00000110 T reflect.Value.Field\n00000120 T reflect.Value.SetInt\n"
	if got := unsupportedReflectionSymbols(output); len(got) != 0 {
		t.Fatalf("unsupportedReflectionSymbols(audited) = %v, want none", got)
	}
	output += "00000130 T reflect.Value.Call\n00000140 T reflect.Value.CallSlice\n00000150 T reflect.MakeFunc\n00000160 T reflect.Value.Method\n00000170 T reflect.ChanOf\n00000180 T reflect.FuncOf\n00000190 T reflect.futureTinyGoStub\n"
	want := []string{"reflect.Value.Call", "reflect.Value.CallSlice", "reflect.MakeFunc", "reflect.Value.Method", "reflect.ChanOf", "reflect.FuncOf", "reflect.futureTinyGoStub"}
	if got := unsupportedReflectionSymbols(output); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unsupportedReflectionSymbols(forbidden) = %v, want %v", got, want)
	}
}

func TestValidateConservativeHeapSymbols(t *testing.T) {
	valid := `00000100 B _globals_end
00000100 B _heap_start
00000100 B playdateRuntimeHeap
00040100 A _heap_end
00040200 B __bss_end__
00000010 D playdateRuntimeSCB
00000014 D runtime.stackTop
00000018 T runtime.runtimePanicAt
00000020 t runtime/interrupt.In
00000030 T tinygo_scanCurrentStack
`
	if err := validateConservativeHeapSymbols(valid); err != nil {
		t.Fatalf("validateConservativeHeapSymbols(valid) error = %v", err)
	}
	tests := []struct {
		name   string
		output string
	}{
		{"missing adapter", strings.ReplaceAll(valid, "00000010 D playdateRuntimeSCB\n", "")},
		{"missing panic trap", strings.ReplaceAll(valid, "00000018 T runtime.runtimePanicAt\n", "")},
		{"globals overlap", strings.ReplaceAll(valid, "00000100 B _globals_end", "000000f0 B _globals_end")},
		{"misaligned", strings.ReplaceAll(valid, "00000100 B _heap_start\n00000100 B playdateRuntimeHeap\n00040100 A _heap_end", "00000101 B _heap_start\n00000101 B playdateRuntimeHeap\n00040101 A _heap_end")},
		{"wrong size", strings.ReplaceAll(valid, "00040100 A _heap_end", "00030100 A _heap_end")},
		{"past BSS", strings.ReplaceAll(valid, "00040200 B __bss_end__", "00040000 B __bss_end__")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateConservativeHeapSymbols(test.output); err == nil {
				t.Fatal("validateConservativeHeapSymbols() error = nil")
			}
		})
	}
}
