package simabi

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	header := filepath.Join(t.TempDir(), "SDK with spaces", "C_API", "pd_api.h")
	sources, err := Render(Config{
		APIHeader:         header,
		RuntimeImport:     "github.com/ivan-gromov-dev/gopdsdk/internal/features/runtime",
		PlaydateImport:    "github.com/ivan-gromov-dev/gopdsdk/playdate",
		ApplicationImport: "github.com/ivan-gromov-dev/gopdsdk/probe/app",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#include " + strconv.Quote(filepath.ToSlash(header)),
		`github.com/ivan-gromov-dev/gopdsdk/probe/app`,
		"sdkRuntime.NewApplication(app.New(), gameContext",
		"application.Handle",
		"application.Update",
		"sdkPlaydate.Buttons(C.bridgeButtons())",
		"C.bridgeCrankAngle",
		"C.bridgeCrankDelta",
		"C.bridgeCrankDocked",
		"C.bridgeFrameDelta",
		"C.bridgeClear",
		"C.bridgeDrawText",
		"C.bridgeLoadFont",
		"C.bridgeTextWidth",
		"C.bridgeFontHeight",
		"C.bridgeFreeFont",
		"C.bridgeCurrentTimeMilliseconds",
		"sdkPlaydate.SystemEnvironment",
		"C.bridgeCurrentEpochTime",
		"C.bridgeEpochToDateTime",
		"C.bridgeDateTimeToEpoch",
		"C.bridgeResetElapsedTime",
		"C.bridgeElapsedTime",
		"C.bridgeSystemInfo",
		"C.bridgeExitToLauncher",
		"sdkPlaydate.SystemControls",
		"C.bridgeGetLaunchArgs",
		"C.bridgeRestartGame",
		"C.bridgeSetMenuImage",
		"C.bridgeSetAutoLockDisabled",
		"C.bridgeSetCrankSoundsDisabled",
		"C.bridgeSetButtonCallback",
		"C.bridgePollButtonEvent",
		"C.bridgeButtonCallbackOverflow",
		"C.bridgeSetAccelerometerEnabled",
		"C.bridgeAccelerometer",
		"C.bridgePowerStatus",
		"C.bridgeBatteryPercentage",
		"C.bridgeSystemVolume",
		"C.bridgeTimezoneOffsetSeconds",
		"sdkPlaydate.FileSystem",
		"sdkPlaydate.SystemMenu",
		"sdkPlaydate.Localization",
		"sdkPlaydate.Scoreboards",
		"sdkPlaydate.DebugMessages",
		"goSerialMessage",
		"goScoreCallback",
		"sdkRuntime.NewScoreboardService",
		"sdkRuntime.ScoreboardCallbackQueue",
		"scoreboardCallbacks.Drain()",
		"scoreboardService.Terminate()",
		"C.bridgeGetScores",
		"goMenuCallback",
		"bridgeLocalizedText",
		"C.bridgeFree",
		"sdkRuntime.NewOwnedFile",
		"C.bridgeFileOpen",
		"C.bridgeFileList",
		"C.bridgeFileRename",
		"func fileErrorMessage() string { message := C.bridgeFileError(); if message == nil { return \"\" }; return C.GoString(message) }",
		"C.bridgeLoadBitmap",
		"C.bridgeLoadBitmapTable",
		"C.bridgeBitmapTableFrame",
		"C.bridgeFreeBitmapTable",
		"C.bridgeNewBitmap",
		"C.bridgeFreeBitmap",
		"C.bridgeBitmapSize",
		"C.bridgeFillBitmap",
		"C.bridgeBitmapData",
		"C.bridgeCopyBitmap",
		"C.bridgeLoadIntoBitmap",
		"C.bridgeNewBitmapTable",
		"C.bridgeLoadIntoBitmapTable",
		"C.bridgeSetBitmapMask",
		"C.bridgeGetBitmapMask",
		"C.bridgeCheckMaskCollision",
		"C.bridgeRotatedBitmap",
		"C.bridgeCopyDisplayBuffer",
		"C.bridgeDrawBitmap",
		"C.bridgeDrawScaledBitmap",
		"C.bridgeDrawRotatedBitmap",
		"C.bridgeSetStencil",
		"C.bridgeClearStencil",
		"C.bridgeLoadVideo",
		"C.bridgeVideoRenderFrame",
		"sdkRuntime.NewVideoPlayer",
		"C.bridgeDisplaySetRefreshRate",
		"C.bridgeDisplayWidth",
		"C.bridgeDisplayHeight",
		"C.bridgeDisplayRefreshRate",
		"C.bridgeDisplayFPS",
		"C.bridgeDisplaySetScale",
		"C.bridgeDisplaySetMosaic",
		"C.bridgeDrawLine",
		"C.bridgeDrawEllipse",
		"C.bridgeSetClipRect",
		"C.bridgeSetDrawMode",
		"C.bridgeFillPolygon",
		"C.bridgeDrawRoundedRect",
		"C.bridgeFillRoundedRect",
		"C.bridgeSetLineCapStyle",
		"C.bridgeSetBackgroundColor",
		"C.bridgeSetScreenClipRect",
		"C.bridgeGetFrame",
		"C.bridgeMarkUpdatedRows",
		"sdkRuntime.WithFramebuffer",
		"sdkRuntime.OwnedBitmapHandle",
		"C.bridgePushContext",
		"C.bridgePopContext",
		"C.bridgeNewSprite",
		"C.bridgeSpriteSetBitmap",
		"C.bridgeNewSpriteTileMap",
		"C.bridgeSpriteSetTileMap",
		"C.bridgeSpriteTileMapSetTiles",
		"C.bridgeSpriteMoveTo",
		"C.bridgeSpriteMoveBy",
		"C.bridgeSpriteSetVisible",
		"C.bridgeSpriteSetZIndex",
		"C.bridgeSpriteSetCollideRect",
		"C.bridgeSpriteMarkDirty",
		"C.bridgeSpriteMarkDirtyRect",
		"C.bridgeSpriteMoveWithCollisions",
		"C.bridgeSpriteCheckCollisions",
		"C.bridgeQuerySpritesAlongLine",
		"C.bridgeQuerySpriteInfoAlongLine",
		"C.bridgeSpriteRemoveMany",
		"C.bridgeRemoveAllSprites",
		"C.bridgeSpriteCount",
		"C.bridgeResetCollisionWorld",
		"C.bridgeQuerySpritesAtPoint",
		"C.bridgeQuerySpritesInRect",
		"C.bridgeSpriteAdd",
		"C.bridgeSpriteRemove",
		"C.bridgeSetAlwaysRedraw",
		"C.bridgeAddDirtyRect",
		"C.bridgeFreeSprite",
		"C.bridgeUpdateAndDrawSprites",
		"C.bridgeLoadSoundEffect",
		"C.bridgeSamplePlayerPlay",
		"C.bridgeLoadFilePlayer",
		"C.bridgeFilePlayerSetRate",
		"C.bridgeCurrentAudioTime",
		"C.bridgeSoundEffectSetFinishCallback",
		"C.bridgeFilePlayerFadeVolume",
		"sdkRuntime.InvokeAudioCallback",
		"sdkRuntime.NewSoundEffect",
		"sdkRuntime.NewSamplePlayer",
		"sdkPlaydate.PCMPlayers",
		"C.bridgeNewPCMPlayer",
		"sdkRuntime.NewFilePlayer",
		"sdkRuntime.NewAudioChannel",
		"C.bridgeAudioChannelOutput",
		"C.bridgeAudioChannelDryLevelSignal",
		"C.bridgeAudioChannelWetLevelSignal",
		"sdkRuntime.DefaultAudioChannel",
		"C.bridgeDefaultAudioChannel",
		"C.bridgeAudioOutputState",
		"C.bridgeSetAudioOutputsActive",
		"sdkRuntime.NewSynth",
		"sdkRuntime.NewLFO",
		"C.bridgeLFOSetStartPhase",
		"C.bridgeLFOSetRandomSeed",
		"C.bridgeLFOSetGlobal",
		"sdkRuntime.NewEnvelope",
		"sdkRuntime.NewControlSignal",
		"sdkPlaydate.Microphones",
		"sdkRuntime.NewMicrophoneService",
		"goMicrophonePermission",
		"goMicrophoneSamples",
	} {
		if !strings.Contains(sources.Go, want) {
			t.Errorf("Go source does not contain %q:\n%s", want, sources.Go)
		}
	}
	for _, want := range []string{
		"#include " + strconv.Quote(filepath.ToSlash(header)),
		"setUpdateCallback(bridgeUpdate, NULL)",
		"setSerialMessageCallback(bridgeSerialMessage)",
		"scoreboards->addScore",
		"scoreboards->getPersonalBest",
		"scoreboards->getScoreboards",
		"scoreboards->getScores",
		"scoreboards->freeScoresList",
		"graphics->clear(kColorWhite)",
		"drawText(text, length, kUTF8Encoding, x, y)",
		"graphics->loadFont",
		"graphics->getTextWidth",
		"graphics->getFontHeight",
		"graphics->setTextTracking",
		"graphics->getTextTracking",
		"graphics->setTextLeading",
		"graphics->drawTextInRect",
		"graphics->getTextHeightForMaxWidth",
		"graphics->getFontPage",
		"graphics->getPageGlyph",
		"channel->getOutputAsSource",
		"channel->getDryLevelSignal",
		"channel->getWetLevelSignal",
		"lfo->setStartPhase",
		"lfo->setRandomSeed",
		"lfo->setGlobal",
		"graphics->getGlyphKerning",
		"system->realloc((void*)font, 0)",
		"system->getCurrentTimeMilliseconds()",
		"system->getSecondsSinceEpoch",
		"system->convertEpochToDateTime",
		"system->convertDateTimeToEpoch",
		"system->getSystemInfo",
		"sound->getCurrentTime()",
		"sound->channel->newChannel",
		"sound->getDefaultChannel()",
		"sound->getHeadphoneState",
		"sound->setOutputsActive",
		"sound->synth->newSynth",
		"sound->lfo->newLFO",
		"sound->envelope->newEnvelope",
		"sound->controlsignal->newSignal",
		"sound->requestMicAccess",
		"sound->setMicCallback",
		"fileplayer->fadeVolume",
		"sampleplayer->setFinishCallback",
		"sample->newSampleFromData",
		"system->exitToLauncher()",
		"system->getLaunchArgs",
		"system->restartGame",
		"system->setMenuImage",
		"system->setAutoLockDisabled",
		"system->setCrankSoundsDisabled",
		"system->setButtonCallback",
		"BRIDGE_BUTTON_EVENT_CAPACITY 64",
		"file->open",
		"file->read",
		"file->listfiles",
		"file->rename",
		"system->getButtonState",
		"system->getCrankAngle",
		"system->getCrankChange",
		"system->isCrankDocked",
		"system->getElapsedTime",
		"system->resetElapsedTime",
		"graphics->loadBitmap",
		"graphics->loadBitmapTable",
		"graphics->getTableBitmap",
		"graphics->freeBitmapTable",
		"graphics->newBitmap",
		"graphics->freeBitmap",
		"graphics->getBitmapData",
		"graphics->clearBitmap",
		"graphics->copyBitmap",
		"graphics->loadIntoBitmap",
		"graphics->newBitmapTable",
		"graphics->loadIntoBitmapTable",
		"graphics->setBitmapMask",
		"graphics->getBitmapMask",
		"graphics->checkMaskCollision",
		"graphics->rotatedBitmap",
		"graphics->copyFrameBufferBitmap",
		"graphics->drawBitmap",
		"graphics->drawScaledBitmap",
		"graphics->drawRotatedBitmap",
		"graphics->setStencilImage",
		"graphics->setStencil(NULL)",
		"graphics->drawLine",
		"graphics->fillTriangle",
		"graphics->setClipRect",
		"graphics->setDrawMode",
		"graphics->fillPolygon",
		"graphics->drawRoundRect",
		"graphics->fillRoundRect",
		"graphics->setLineCapStyle",
		"graphics->setBackgroundColor",
		"graphics->setScreenClipRect",
		"graphics->getFrame",
		"graphics->markUpdatedRows",
		"graphics->pushContext",
		"graphics->popContext",
		"display->setRefreshRate",
		"display->setScale",
		"display->setMosaic",
		"sprite->newSprite",
		"sprite->setImage",
		"sprite->moveTo",
		"sprite->moveBy",
		"sprite->setVisible",
		"sprite->setZIndex",
		"sprite->setCollideRect",
		"sprite->markDirty",
		"sprite->markDirtyRect",
		"sprite->moveWithCollisions",
		"sprite->checkCollisions",
		"sprite->querySpritesAlongLine",
		"sprite->querySpriteInfoAlongLine",
		"sprite->removeSprites",
		"sprite->removeAllSprites",
		"sprite->getSpriteCount",
		"sprite->resetCollisionWorld",
		"sprite->querySpritesAtPoint",
		"sprite->querySpritesInRect",
		"sprite->addSprite",
		"sprite->removeSprite",
		"sprite->setAlwaysRedraw",
		"sprite->addDirtyRect",
		"LCDMakeRect",
		"sprite->freeSprite",
		"sprite->updateAndDrawSprites",
		"sound->sample->load",
		"sound->sampleplayer->newPlayer",
		"graphics->video->loadVideo",
		"graphics->video->renderFrame",
		"sound->fileplayer->loadIntoPlayer",
	} {
		if !strings.Contains(sources.C, want) {
			t.Errorf("C source does not contain %q:\n%s", want, sources.C)
		}
	}
}

func TestRenderRequiresEveryInput(t *testing.T) {
	_, err := Render(Config{})
	if got, want := err.Error(), "render Simulator ABI: APIHeader is required"; got != want {
		t.Fatalf("Render() error = %q, want %q", got, want)
	}
}

func TestCallbackAudioUsesBoundedNativeRings(t *testing.T) {
	sources, err := Render(Config{APIHeader: "pd_api.h", RuntimeImport: "example.com/sdk/runtime", PlaydateImport: "example.com/sdk/playdate", ApplicationImport: "example.com/game"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"BRIDGE_PCM_SOURCE_COUNT", "BRIDGE_PCM_RING_FRAMES", "bridgePCMRender", "bridgeNewPCMCallbackSource", "BRIDGE_GENERATOR_VOICE_COUNT", "BRIDGE_GENERATOR_RING_FRAMES", "bridgeGeneratorRender", "bridgeGeneratorCopy", "bridgeNewGeneratorSynth"} {
		if !strings.Contains(sources.C, want) {
			t.Errorf("C source does not contain %q", want)
		}
	}
	if !strings.Contains(sources.C, "bridgeGeneratorParameterCallback") || strings.Contains(sources.C, "static int bridgeGeneratorSetParameter(") {
		t.Error("generator callback and exported parameter setter must have distinct C symbols")
	}
	for _, want := range []string{"#define BRIDGE_PCM_RING_FRAMES 4096", "#define BRIDGE_GENERATOR_RING_FRAMES 4096"} {
		if !strings.Contains(sources.C, want) {
			t.Errorf("bounded ring is too short for a 30 FPS update interval: missing %q", want)
		}
	}
	for _, want := range []string{"sdkRuntime.NewPCMCallbackSource", "sdkRuntime.NewGeneratorSynth", "GeneratorVoiceState"} {
		if !strings.Contains(sources.Go, want) {
			t.Errorf("Go source does not contain %q", want)
		}
	}
}

func TestButtonCallbacksUseBoundedNativeQueueAndUpdateDelivery(t *testing.T) {
	sources, err := Render(Config{APIHeader: "pd_api.h", RuntimeImport: "example.com/sdk/runtime", PlaydateImport: "example.com/sdk/playdate", ApplicationImport: "example.com/game"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#define BRIDGE_BUTTON_EVENT_CAPACITY 64", "bridgeButtonDropped++", "bridgePollButtonEvent", "bridgeButtonCallbackOverflow"} {
		if !strings.Contains(sources.C, want) {
			t.Errorf("C source does not contain %q", want)
		}
	}
	if strings.Contains(sources.C, "extern void goButton") || strings.Contains(sources.C, "extern int goButton") {
		t.Fatal("native button callback must not enter Go directly")
	}
	for _, want := range []string{"for buttonCallback != nil", "callback(sdkPlaydate.ButtonEvent", "sdkRuntime.ValidateButtonCallbackConfig"} {
		if !strings.Contains(sources.Go, want) {
			t.Errorf("Go source does not contain %q", want)
		}
	}
}
