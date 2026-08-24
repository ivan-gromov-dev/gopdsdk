package playdate_test

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestExportedAPIBaseline(t *testing.T) {
	t.Parallel()

	actual := exportedAPI(t)
	if actual != apiBaseline {
		oldLines, newLines := strings.Split(apiBaseline, "\n"), strings.Split(actual, "\n")
		for index := range oldLines {
			if index >= len(newLines) || oldLines[index] != newLines[index] {
				t.Fatalf("exported playdate API order differs at %d: baseline %q, current %q", index, oldLines[index], newLines[index])
			}
		}
		t.Fatalf("exported playdate API changed (-baseline +current):\n%s", lineDiff(apiBaseline, actual))
	}
}

func exportedAPI(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate API snapshot test")
	}
	directory := filepath.Dir(filename)
	files, err := filepath.Glob(filepath.Join(directory, "*.go"))
	if err != nil {
		t.Fatal(err)
	}

	set := token.NewFileSet()
	parsed := make([]*ast.File, 0, len(files))
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, parseErr := parser.ParseFile(set, file, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", file, parseErr)
		}
		parsed = append(parsed, source)
	}

	config := types.Config{Importer: importer.Default()}
	pkg, err := config.Check("github.com/ivan-gromov-dev/gopdsdk/playdate", set, parsed, nil)
	if err != nil {
		t.Fatalf("type-check playdate: %v", err)
	}
	qualifier := func(other *types.Package) string {
		if other == pkg {
			return ""
		}
		return other.Path()
	}

	var lines []string
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		object := scope.Lookup(name)
		if !object.Exported() {
			continue
		}
		lines = append(lines, types.ObjectString(object, qualifier))
		typeName, ok := object.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		for methodIndex := 0; methodIndex < named.NumMethods(); methodIndex++ {
			method := named.Method(methodIndex)
			if method.Exported() {
				lines = append(lines, types.ObjectString(method, qualifier))
			}
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

func lineDiff(baseline, current string) string {
	oldLines := strings.Split(strings.TrimSuffix(baseline, "\n"), "\n")
	newLines := strings.Split(strings.TrimSuffix(current, "\n"), "\n")
	oldSet := make(map[string]bool, len(oldLines))
	newSet := make(map[string]bool, len(newLines))
	for _, line := range oldLines {
		oldSet[line] = true
	}
	for _, line := range newLines {
		newSet[line] = true
	}

	var result bytes.Buffer
	for _, line := range oldLines {
		if !newSet[line] {
			result.WriteString("- ")
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}
	for _, line := range newLines {
		if !oldSet[line] {
			result.WriteString("+ ")
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}
	return result.String()
}

const apiBaseline = `const BitmapFlippedX BitmapFlip
const BitmapFlippedXY BitmapFlip
const BitmapFlippedY BitmapFlip
const BitmapUnflipped BitmapFlip
const ButtonA Buttons
const ButtonB Buttons
const ButtonCallbackQueueLimit untyped int
const ButtonDown Buttons
const ButtonLeft Buttons
const ButtonRight Buttons
const ButtonUp Buttons
const CollisionBounce CollisionResponse
const CollisionFreeze CollisionResponse
const CollisionOverlap CollisionResponse
const CollisionSlide CollisionResponse
const ColorBlack Color
const ColorClear Color
const ColorWhite Color
const DrawModeBlackTransparent DrawMode
const DrawModeCopy DrawMode
const DrawModeFillBlack DrawMode
const DrawModeFillWhite DrawMode
const DrawModeInverted DrawMode
const DrawModeNXOR DrawMode
const DrawModeWhiteTransparent DrawMode
const DrawModeXOR DrawMode
const FileAppend FileOptions
const FileReadData FileOptions
const FileReadPackage FileOptions
const FileWrite FileOptions
const FilterBandPass FilterType
const FilterHighPass FilterType
const FilterHighShelf FilterType
const FilterLowPass FilterType
const FilterLowShelf FilterType
const FilterNotch FilterType
const FilterPEQ FilterType
const LFOTypeArpeggiator LFOType
const LFOTypeSampleAndHold LFOType
const LFOTypeSawtoothDown LFOType
const LFOTypeSawtoothUp LFOType
const LFOTypeSine LFOType
const LFOTypeSquare LFOType
const LFOTypeTriangle LFOType
const LanguageEnglish Language
const LanguageJapanese Language
const LanguageSystem Language
const LifecycleLock LifecycleEvent
const LifecycleLowPower LifecycleEvent
const LifecycleMirrorEnded LifecycleEvent
const LifecycleMirrorStarted LifecycleEvent
const LifecyclePause LifecycleEvent
const LifecycleResume LifecycleEvent
const LifecycleTerminate LifecycleEvent
const LifecycleUnlock LifecycleEvent
const LineCapButt LineCapStyle
const LineCapRound LineCapStyle
const LineCapSquare LineCapStyle
const MicrophonePermissionDenied MicrophonePermission
const MicrophonePermissionGranted MicrophonePermission
const MicrophonePermissionPending MicrophonePermission
const MicrophoneSourceAutomatic MicrophoneSource
const MicrophoneSourceHeadset MicrophoneSource
const MicrophoneSourceInternal MicrophoneSource
const PlaybackPaused PlaybackState
const PlaybackPlaying PlaybackState
const PlaybackStopped PlaybackState
const PolygonFillEvenOdd PolygonFillRule
const PolygonFillNonZero PolygonFillRule
const PowerCharging PowerStatus
const PowerScrews PowerStatus
const PowerUSB PowerStatus
const Sound16BitMono SoundFormat
const Sound16BitStereo SoundFormat
const Sound8BitMono SoundFormat
const Sound8BitStereo SoundFormat
const SoundADPCMMono SoundFormat
const SoundADPCMStereo SoundFormat
const TextAlignCenter TextAlignment
const TextAlignLeft TextAlignment
const TextAlignRight TextAlignment
const TextWrapCharacter TextWrappingMode
const TextWrapClip TextWrappingMode
const TextWrapWord TextWrappingMode
const WaveformNoise Waveform
const WaveformPODigital Waveform
const WaveformPOPhase Waveform
const WaveformPOVosim Waveform
const WaveformSawtooth Waveform
const WaveformSine Waveform
const WaveformSquare Waveform
const WaveformTriangle Waveform
func (*Animation).Bitmap() (Bitmap, error)
func (*Animation).Frame() int
func (*Animation).Pause()
func (*Animation).Paused() bool
func (*Animation).Resume()
func (*Animation).SetFixedFrame(frame int) error
func (*Animation).Update(deltaSeconds float32)
func (*Animation).UseDeltaTime()
func (*Camera).Clamp(worldWidth int, worldHeight int)
func (*TileMap).Draw(graphics Graphics, bitmaps []Bitmap, camera Camera) (TileDrawStats, error)
func (*TileMap).IntersectsSolid(rect Rect) bool
func (*TileMap).TileAt(column int, row int) (uint8, bool)
func (*TileMap).WorldSize() (width int, height int)
func (AudioLoadError).Error() string
func (BitmapLoadError).Error() string
func (Buttons).Has(requested Buttons) bool
func (FileOperationError).Error() string
func (FileOperationError).Unwrap() error
func (FontLoadError).Error() string
func (FontLoadError).Is(target error) bool
func (Paint).Components() (solid uint8, pattern [16]byte, patterned bool)
func (PowerStatus).Has(requested PowerStatus) bool
func (ScoreboardOperationError).Error() string
func (ScoreboardOperationError).Unwrap() error
func (VideoOperationError).Error() string
func NewAnimation(table BitmapTable, first int, count int, frameSeconds float32) (*Animation, error)
func NewTileMap(config TileMapConfig) (*TileMap, error)
func PatternPaint(image [8]byte, mask [8]byte) Paint
func SolidPaint(color Color) (Paint, error)
func XORPaint() Paint
type Accelerometer interface{AccelerometerXYZ() (x float32, y float32, z float32); SetAccelerometerEnabled(bool)}
type Animation struct{table BitmapTable; first int; count int; frame int; frameSeconds float32; elapsed float32; paused bool; fixed bool}
type Audio interface{LoadFilePlayer(path string) (FilePlayer, error); LoadSoundEffect(path string) (SoundEffect, error)}
type AudioChannel interface{AddEffect(effect AudioEffect) error; AddSource(source AudioSource) error; Close() error; DryLevelSignal() (Signal, error); Output() (AudioSource, error); RemoveEffect(effect AudioEffect) error; RemoveSource(source AudioSource) error; SetPan(pan float32) error; SetPanModulator(Signal) error; SetVolume(volume float32) error; SetVolumeModulator(Signal) error; Volume() (float32, error); WetLevelSignal() (Signal, error)}
type AudioChannels interface{NewAudioChannel() (AudioChannel, error)}
type AudioClock interface{CurrentAudioTime() (uint32, error)}
type AudioEffect interface{Close() error; SetMix(level float32) error; SetMixModulator(signal Signal) error}
type AudioEffects interface{NewBitCrusher() (BitCrusher, error); NewDelayLine(lengthFrames int, stereo bool) (DelayLine, error); NewOnePoleFilter() (OnePoleFilter, error); NewOverdrive() (Overdrive, error); NewRingModulator() (RingModulator, error); NewTwoPoleFilter(FilterType) (TwoPoleFilter, error)}
type AudioLoadError string
type AudioOutputState struct{Headphones bool; HeadsetMicrophone bool}
type AudioOutputs interface{AudioOutputState() (AudioOutputState, error); DefaultAudioChannel() (AudioChannel, error); SetAudioOutputsActive(headphones bool, speaker bool) error}
type AudioSample interface{Close() error; Data() (SampleData, error); Decompress() error; Length() (float32, error); Load(path string) error}
type AudioSamples interface{LoadSample(path string) (AudioSample, error); NewSample(byteCount int) (AudioSample, error); NewSampleFromData(data []byte, format SoundFormat, sampleRate uint32) (AudioSample, error)}
type AudioSource interface{SetVolume(left float32, right float32) error; State() (PlaybackState, error); Volume() (left float32, right float32, err error)}
type BitCrusher interface{SetDepth(float32) error; SetDepthModulator(Signal) error; SetDownsampling(float32) error; SetDownsamplingModulator(Signal) error; SetExponential(bool) error; AudioEffect}
type Bitmap interface{Clear() error; Close() error; Fill(Color) error; Height() (int, error); Width() (int, error)}
type BitmapCompositor interface{DrawRotatedBitmap(bitmap Bitmap, x int, y int, degrees float32, centerX float32, centerY float32, scaleX float32, scaleY float32) error; WithStencil(stencil Bitmap, tiled bool, callback func() error) error}
type BitmapData interface{Bytes() ([]byte, error); Dirty() (bool, error); Height() int; MarkDirty() error; MaskBytes() ([]byte, error); Pixel(x int, y int) (Color, error); RowBytes() int; SetPixel(x int, y int, color Color) error; Width() int}
type BitmapDataGraphics interface{BitmapMask(bitmap Bitmap) (Bitmap, bool, error); CheckBitmapMaskCollision(first Bitmap, firstX int, firstY int, firstFlip BitmapFlip, second Bitmap, secondX int, secondY int, secondFlip BitmapFlip, rectX int, rectY int, rectWidth int, rectHeight int) (bool, error); ClearBitmapMask(bitmap Bitmap) error; CopyBitmap(bitmap Bitmap) (Bitmap, error); CopyDisplayBuffer() (Bitmap, error); LoadIntoBitmap(path string, bitmap Bitmap) error; LoadIntoBitmapTable(path string, table BitmapTable) error; NewBitmapTable(count int, width int, height int) (BitmapTable, error); RotatedBitmap(bitmap Bitmap, degrees float32, scaleX float32, scaleY float32) (Bitmap, int, error); SetBitmapMask(bitmap Bitmap, mask Bitmap) error; WithBitmapData(bitmap Bitmap, callback func(BitmapData) error) error}
type BitmapFlip uint8
type BitmapLoadError string
type BitmapTable interface{Close() error; Frame(index int) (Bitmap, error)}
type Board struct{ID string; Name string}
type BoardsList struct{LastUpdated uint32; Boards []Board}
type ButtonCallback func(ButtonEvent)
type ButtonEvent struct{Button Buttons; Down bool; When uint32}
type Buttons uint8
type CallbackAudio interface{NewPCMCallbackSource(channel AudioChannel, stereo bool, callback PCMRenderCallback) (PCMCallbackSource, error)}
type Camera struct{X int; Y int; Width int; Height int}
type CheckmarkMenuItem interface{SetValue(bool); Value() bool; MenuItem}
type Collision struct{Other Sprite; ResponseType CollisionResponse; Overlaps bool; Time float32; Move Point; Normal Point; Touch Point; SpriteRect Rect; OtherRect Rect}
type CollisionResponse uint8
type Color uint8
type CompletionPlayer interface{SetFinishCallback(callback func()) error}
type Context interface{System; Graphics; InputReader; Sprites; Audio}
type ControlSignal interface{AddEvent(step int, value float32, interpolate bool) error; ClearEvents() error; RemoveEvent(step int) error; Signal}
type DateTime struct{Year uint16; Month uint8; Day uint8; Weekday uint8; Hour uint8; Minute uint8; Second uint8}
type DebugMessages interface{PollDebugMessage() (message string, ok bool)}
type DelayLine interface{AddTap(delayFrames int) (DelayTap, error); SetFeedback(float32) error; SetLength(frames int) error; AudioEffect}
type DelayTap interface{Close() error; SetChannelsFlipped(bool) error; SetDelay(frames int) error; SetDelayModulator(Signal) error; AudioSource}
type Display interface{FPS() float32; Height() int; RefreshRate() float32; SetFlipped(x bool, y bool); SetInverted(bool); SetMosaic(x uint, y uint) error; SetOffset(x int, y int); SetRefreshRate(framesPerSecond float32) error; SetScale(uint) error; Width() int}
type DrawMode uint8
type Envelope interface{SetAttack(seconds float32) error; SetCurvature(amount float32) error; SetDecay(seconds float32) error; SetLegato(legato bool) error; SetRateScaling(scaling float32, startNote uint8, endNote uint8) error; SetRelease(seconds float32) error; SetRetrigger(retrigger bool) error; SetSustain(level float32) error; SetVelocitySensitivity(amount float32) error; Signal}
type EpochTime struct{Seconds uint32; Milliseconds uint32}
type FadingPlayer interface{FadeVolume(left float32, right float32, audioFrames uint32, callback func()) error}
type File interface{Close() error; Flush() error; io.Reader; io.Writer; io.Seeker}
type FileInfo struct{IsDir bool; Size uint32; Year int; Month int; Day int; Hour int; Minute int; Second int}
type FileOperationError struct{Operation string; Path string; Message string}
type FileOptions uint8
type FilePlayer interface{Close() error; Pause() error; Play() error; Resume() error; Stop() error; AudioSource}
type FileSystem interface{List(path string, showHidden bool) ([]string, error); Mkdir(path string) error; OpenFile(path string, options FileOptions) (File, error); Remove(path string, recursive bool) error; Rename(from string, to string) error; Stat(path string) (FileInfo, error)}
type FilterType uint8
type Font interface{Close() error; Height() (int, error); TextWidth(text string) (int, error)}
type FontGlyph struct{Bitmap Bitmap; Advance int; Kerning int}
type FontGraphics interface{DrawTextFont(font Font, text string, x int, y int) error; LoadFont(path string) (Font, error)}
type FontLoadError string
type Framebuffer interface{Bytes() ([]byte, error); Height() int; MarkDirtyRows(start int, end int) error; Pixel(x int, y int) (Color, error); RowBytes() int; SetPixel(x int, y int, color Color) error; Width() int}
type FramebufferGraphics interface{WithFramebuffer(callback func(Framebuffer) error) error}
type Game interface{Init(Context) error; Update(Context) (refresh bool, err error)}
type GeneratorRenderCallback func(state GeneratorState, left []int16, right []int16) int
type GeneratorState struct{Voice uint8; Note float32; Velocity float32; Length float32; Released bool; ReleaseOffset int32; Rate uint32; DeltaRate int32; Parameters [8]float32}
type GeneratorSynth interface{Synth}
type GeneratorSynthesizers interface{NewGeneratorSynth(stereo bool, callback GeneratorRenderCallback) (GeneratorSynth, error)}
type Graphics interface{Clear(); DrawBitmap(bitmap Bitmap, x int, y int) error; DrawScaledBitmap(bitmap Bitmap, x int, y int, scaleX float32, scaleY float32) error; DrawText(text string, x int, y int); LoadBitmap(path string) (Bitmap, error); LoadBitmapTable(path string) (BitmapTable, error); NewBitmap(width int, height int) (Bitmap, error)}
type GraphicsPoint struct{X int; Y int}
type GraphicsState interface{ClearClipRect(); SetBackgroundColor(color Color) error; SetClipRect(x int, y int, width int, height int) error; SetDrawMode(mode DrawMode) error; SetDrawOffset(dx int, dy int); SetLineCapStyle(style LineCapStyle) error; SetScreenClipRect(x int, y int, width int, height int) error}
type Input struct{Buttons Buttons; Pressed Buttons; Released Buttons; Held Buttons; CrankAngle float32; CrankDelta float32; CrankDocked bool; CrankDockedThisFrame bool; CrankUndocked bool; DeltaSeconds float32}
type InputReader interface{Input() Input}
type Instrument interface{ActiveVoiceCount() (int, error); AddVoice(synth Synth, rangeStart uint8, rangeEnd uint8, transpose float32) error; AllNotesOff(when uint32) error; Close() error; NoteOff(note uint8, when uint32) error; SetPitchBend(float32) error; SetPitchBendRange(float32) error; SetTranspose(float32) error; SetVolume(left float32, right float32) error; Volume() (left float32, right float32, err error); AudioSource}
type LFO interface{SetArpeggiation(steps []float32) error; SetCenter(center float32) error; SetDepth(depth float32) error; SetGlobal(global bool) error; SetPhase(phase float32) error; SetRandomSeed(seed uint16) error; SetRate(rate float32) error; SetRetrigger(retrigger bool) error; SetStartPhase(phase float32) error; Signal}
type LFOType uint8
type Language int
type Launcher interface{ExitToLauncher()}
type LifecycleEvent uint8
type LifecycleGame interface{HandleLifecycle(Context, LifecycleEvent) error}
type LineCapStyle uint8
type ListScore struct{Rank uint32; Value uint32; Player string}
type Localization interface{Language() Language; LocalizedText(key string, language Language) (string, bool)}
type LoopCallbackPlayer interface{SetLoopCallback(callback func()) error}
type MenuItem interface{Remove(); SetTitle(string) error; Title() string}
type MicrophonePermission uint8
type MicrophoneRecorder interface{Close() error; Source() MicrophoneSource; Stop() error}
type MicrophoneSamples interface{CopyTo(destination []int16) (int, error); Len() int}
type MicrophoneSource uint8
type Microphones interface{RequestMicrophoneAccess(purpose string, callback func(MicrophonePermission)) (MicrophonePermission, error); StartMicrophoneRecording(source MicrophoneSource, callback func(MicrophoneSamples) bool) (MicrophoneRecorder, error)}
type MoveResult struct{ActualX float32; ActualY float32; Collisions []Collision}
type OffscreenGraphics interface{DrawInto(bitmap Bitmap, callback func() error) error}
type OnePoleFilter interface{SetParameter(float32) error; SetParameterModulator(Signal) error; AudioEffect}
type OptionsMenuItem interface{SetValue(int) error; Value() int; MenuItem}
type Overdrive interface{SetGain(float32) error; SetLimit(float32) error; SetLimitModulator(Signal) error; SetOffset(float32) error; SetOffsetModulator(Signal) error; AudioEffect}
type PCMCallbackSource interface{Close() error; UnderrunCount() (uint32, error); AudioSource}
type PCMPlayers interface{NewPCMPlayer(samples []int16, sampleRate uint32) (SamplePlayer, error)}
type PCMRenderCallback func(left []int16, right []int16) int
type Paint struct{pattern [16]byte; solid Color; kind uint8}
type PlaybackState uint8
type Point struct{X float32; Y float32}
type PolygonFillRule uint8
type PowerMonitor interface{BatteryPercentage() float32; BatteryVoltage() float32; PowerStatus() PowerStatus}
type PowerStatus uint8
type PrimitiveGraphics interface{DrawEllipse(x int, y int, width int, height int, lineWidth int, startAngle float32, endAngle float32, paint Paint) error; DrawLine(x1 int, y1 int, x2 int, y2 int, width int, paint Paint) error; DrawRect(x int, y int, width int, height int, paint Paint) error; DrawRoundedRect(x int, y int, width int, height int, radius int, lineWidth int, paint Paint) error; DrawTriangle(x1 int, y1 int, x2 int, y2 int, x3 int, y3 int, width int, paint Paint) error; FillEllipse(x int, y int, width int, height int, startAngle float32, endAngle float32, paint Paint) error; FillPolygon(points []GraphicsPoint, rule PolygonFillRule, paint Paint) error; FillRect(x int, y int, width int, height int, paint Paint) error; FillRoundedRect(x int, y int, width int, height int, radius int, paint Paint) error; FillTriangle(x1 int, y1 int, x2 int, y2 int, x3 int, y3 int, paint Paint) error}
type RateModulatedPlayer interface{SetRateModulator(Signal) error}
type Rect struct{X float32; Y float32; Width float32; Height float32}
type RingModulator interface{SetFrequency(float32) error; SetFrequencyModulator(Signal) error; AudioEffect}
type SampleData interface{CopyTo(dst []byte) (int, error); Format() SoundFormat; Len() int; SampleRate() uint32}
type SamplePlayer interface{Length() (float32, error); Offset() (float32, error); PlayRepeated(repeat int, rate float32) error; Rate() (float32, error); SetOffset(seconds float32) error; SetRate(rate float32) error; SoundEffect}
type SamplePlayerControls interface{SetPlayRange(startFrame int, endFrame int) error; SetSample(AudioSample) error}
type SamplePlayerFactory interface{NewSamplePlayer(sample AudioSample) (SamplePlayer, error)}
type SamplePlayers interface{LoadSamplePlayer(path string) (SamplePlayer, error)}
type Score struct{Rank uint32; Value uint32; Player string; BoardID string}
type ScoreboardOperationError struct{Operation string; BoardID string; Message string}
type Scoreboards interface{AddScore(boardID string, value uint32, callback func(Score, error)) error; GetPersonalBest(boardID string, callback func(Score, error)) error; GetScoreboards(callback func(BoardsList, error)) error; GetScores(boardID string, callback func(ScoresList, error)) error}
type ScoresList struct{BoardID string; LastUpdated uint32; PlayerIncluded bool; Limit uint32; Scores []ListScore}
type Sequence interface{AllNotesOff() error; Close() error; CurrentStep() (step int, timeOffset int, err error); IsPlaying() (bool, error); Length() (uint32, error); LoadMIDI(path string) error; Play(callback func()) error; SetLoops(start int, end int, count int) error; SetTempo(stepsPerSecond float32) error; SetTime(uint32) error; SetTrack(index uint, track SequenceTrack) error; Stop() error; Tempo() (float32, error); Time() (uint32, error); Track(index uint) (SequenceTrack, error); TrackCount() (uint, error)}
type SequenceNote struct{Step uint32; Length uint32; Note uint8; Velocity float32}
type SequenceTrack interface{ActiveVoiceCount() (int, error); AddControlEvent(controller int, step int, value float32, interpolate bool) error; AddNote(step uint32, length uint32, note uint8, velocity float32) error; ClearControlEvents() error; ClearNotes() error; Close() error; ControlSignal(index int) (ControlSignal, error); ControlSignalCount() (int, error); Instrument() (Instrument, error); Length() (uint32, error); NoteAt(index int) (SequenceNote, bool, error); NoteIndexAtStep(step uint32) (int, error); Polyphony() (int, error); RemoveControlEvent(controller int, step int) error; RemoveNote(step uint32, note uint8) error; SetInstrument(Instrument) error; SetMuted(bool) error; SignalForController(controller int, create bool) (ControlSignal, error)}
type Sequencers interface{NewInstrument() (Instrument, error); NewSequence() (Sequence, error); NewSequenceTrack() (SequenceTrack, error)}
type Signal interface{Close() error; SetOffset(offset float32) error; SetScale(scale float32) error; Value() (float32, error)}
type SoundEffect interface{Close() error; Pause() error; Play() error; Resume() error; Stop() error; AudioSource}
type SoundFormat uint8
type Sprite interface{Add() error; Bounds() (Rect, error); Center() (x float32, y float32, err error); CheckCollisions(goalX float32, goalY float32) (MoveResult, error); ClearClipRect() error; ClearCollideRect() error; ClearStencil() error; ClearTileMap() error; Close() error; CollideRect() (Rect, error); CollisionsEnabled() (bool, error); ImageFlip() (BitmapFlip, error); MarkDirty() error; MarkDirtyRect(Rect) error; MoveBy(dx float32, dy float32) error; MoveWithCollisions(goalX float32, goalY float32) (MoveResult, error); Position() (x float32, y float32, err error); Remove() error; SetBitmap(Bitmap) error; SetBounds(Rect) error; SetCenter(x float32, y float32) error; SetClipRect(x int, y int, width int, height int) error; SetCollideRect(Rect) error; SetCollisionResponseCallback(SpriteCollisionResponseCallback) error; SetCollisionsEnabled(bool) error; SetDrawCallback(SpriteDrawCallback) error; SetDrawMode(DrawMode) error; SetIgnoresDrawOffset(bool) error; SetImageFlip(BitmapFlip) error; SetOpaque(bool) error; SetPosition(x float32, y float32) error; SetStencilImage(Bitmap, bool) error; SetStencilPattern([8]byte) error; SetTag(uint8) error; SetTileMap(SpriteTileMap) error; SetUpdateCallback(SpriteUpdateCallback) error; SetUpdatesEnabled(bool) error; SetVisible(bool) error; SetZIndex(int) error; Tag() (uint8, error); TileMap() (SpriteTileMap, bool, error); UpdatesEnabled() (bool, error); Visible() (bool, error); ZIndex() (int, error)}
type SpriteCollisionResponseCallback func(sprite Sprite, other Sprite) CollisionResponse
type SpriteDisplayList interface{AddSprites([]Sprite) error; RemoveAllSprites(); RemoveSprites([]Sprite) error; ResetCollisionWorld(); SpriteCount() int}
type SpriteDrawCallback func(sprite Sprite, bounds Rect, drawRect Rect)
type SpriteQueries interface{QuerySpriteInfoAlongLine(x1 float32, y1 float32, x2 float32, y2 float32) []SpriteQueryInfo; QuerySpritesAlongLine(x1 float32, y1 float32, x2 float32, y2 float32) []Sprite}
type SpriteQueryInfo struct{Sprite Sprite; EntryTime float32; ExitTime float32; EntryPoint Point; ExitPoint Point}
type SpriteRedraw interface{AddDirtyRect(x int, y int, width int, height int) error; SetAlwaysRedraw(bool)}
type SpriteTileMap interface{Close() error; PixelSize() (width int, height int, err error); SetTile(column int, row int, index uint16) error; Size() (columns int, rows int, err error); Tile(column int, row int) (uint16, error)}
type SpriteTileMaps interface{NewSpriteTileMap(BitmapTable, int, int, []uint16) (SpriteTileMap, error)}
type SpriteUpdateCallback func(sprite Sprite)
type Sprites interface{NewSprite() (Sprite, error); QueryOverlappingSprites(Sprite) ([]Sprite, error); QuerySpritesAtPoint(x float32, y float32) []Sprite; QuerySpritesInRect(Rect) []Sprite; UpdateAndDrawSprites()}
type StreamingPlayerControls interface{DidUnderrun() (bool, error); Load(path string) error; SetBufferLength(seconds float32) error; SetLoopRange(start float32, end float32) error; SetStopOnUnderrun(bool) error}
type Synth interface{Close() error; NoteOff(when uint32) error; PlayMIDINote(note float32, velocity float32, length float32, when uint32) error; SetAmplitudeModulator(signal Signal) error; SetEnvelope(attack float32, decay float32, sustain float32, release float32) error; SetEnvelopeCurvature(amount float32) error; SetEnvelopeRateScaling(scaling float32, startNote uint8, endNote uint8) error; SetEnvelopeVelocitySensitivity(amount float32) error; SetFrequencyModulator(signal Signal) error; SetParameter(parameter int, value float32) error; SetParameterModulator(parameter int, signal Signal) error; SetTranspose(semitones float32) error; SetWaveform(waveform Waveform) error; SetWavetable(sample AudioSample, log2Size int, columns int, rows int) error; Stop() error; AudioSource}
type Synthesizers interface{NewControlSignal() (ControlSignal, error); NewEnvelope(attack float32, decay float32, sustain float32, release float32) (Envelope, error); NewLFO(lfoType LFOType) (LFO, error); NewSynth(waveform Waveform) (Synth, error)}
type System interface{CurrentTimeMilliseconds() uint32}
type SystemControls interface{ButtonCallbackOverflow() uint32; ClearMenuImage(); LaunchArguments() (arguments string, path string); RestartGame(arguments string) error; SetAutoLockDisabled(disabled bool); SetButtonCallback(callback ButtonCallback, queueSize int) error; SetCrankSoundsDisabled(disabled bool) (previous bool); SetMenuImage(bitmap Bitmap, xOffset int) error}
type SystemEnvironment interface{CurrentEpochTime() EpochTime; DateTimeToEpoch(dateTime DateTime) (uint32, error); ElapsedTime() float32; EpochToDateTime(epoch uint32) DateTime; ResetElapsedTime(); SystemInfo() SystemInfo}
type SystemInfo struct{OSVersion uint32; Language Language; PDXVersion uint32}
type SystemMenu interface{AddActionMenuItem(title string, callback func()) (MenuItem, error); AddCheckmarkMenuItem(title string, value bool, callback func()) (CheckmarkMenuItem, error); AddOptionsMenuItem(title string, options []string, callback func()) (OptionsMenuItem, error)}
type SystemPreferences interface{ReduceFlashing() bool; SystemVolume() float32; TimezoneOffsetSeconds() int32; Uses24HourTime() bool}
type TextAlignment uint8
type TextGraphics interface{DrawTextInRect(text string, x int, y int, width int, height int, wrapping TextWrappingMode, alignment TextAlignment) error; Glyph(font Font, codepoint rune, next rune) (FontGlyph, error); SetTextLeading(leading int); SetTextTracking(tracking int); TextHeight(font Font, text string, maxWidth int, wrapping TextWrappingMode, tracking int, leading int) (int, error); TextTracking() int}
type TextWrappingMode uint8
type TileDrawStats struct{Visited int; Drawn int}
type TileMap struct{columns int; rows int; tileWidth int; tileHeight int; tiles []uint8; solid []bool}
type TileMapConfig struct{Columns int; Rows int; TileWidth int; TileHeight int; Tiles []uint8; Solid []bool}
type TwoPoleFilter interface{SetFrequency(float32) error; SetFrequencyModulator(Signal) error; SetGain(float32) error; SetResonance(float32) error; SetResonanceModulator(Signal) error; AudioEffect}
type VariableRatePlayer interface{Rate() (float32, error); SetRate(rate float32) error}
type VideoInfo struct{Width int; Height int; FrameRate float32; FrameCount int; CurrentFrame int}
type VideoOperationError struct{Operation string; Message string}
type VideoPlayer interface{Close() error; Info() (VideoInfo, error); LastError() (string, error); RenderFrame(frame int) error; SetContext(Bitmap) error; UseScreenContext() error}
type Videos interface{LoadVideo(path string) (VideoPlayer, error)}
type Waveform uint8
var ErrAnimationConfig error
var ErrAudioBufferLength error
var ErrAudioCallback error
var ErrAudioChannelClosed error
var ErrAudioClosed error
var ErrAudioCreate error
var ErrAudioEventStep error
var ErrAudioFade error
var ErrAudioFormat error
var ErrAudioGraphClosed error
var ErrAudioOffset error
var ErrAudioPan error
var ErrAudioParameter error
var ErrAudioPlay error
var ErrAudioRange error
var ErrAudioRate error
var ErrAudioRepeat error
var ErrAudioReverseUnsupported error
var ErrAudioRoute error
var ErrAudioSampleClosed error
var ErrAudioSampleInUse error
var ErrAudioSampleSize error
var ErrAudioSourceInvalid error
var ErrAudioUnavailable error
var ErrAudioVolume error
var ErrAudioWaveform error
var ErrBitmapBorrowed error
var ErrBitmapBounds error
var ErrBitmapClosed error
var ErrBitmapColor error
var ErrBitmapCreate error
var ErrBitmapDataCallback error
var ErrBitmapDataExpired error
var ErrBitmapFlip error
var ErrBitmapFrameRange error
var ErrBitmapMask error
var ErrBitmapMaskInUse error
var ErrBitmapMaskSize error
var ErrBitmapMenuImageInUse error
var ErrBitmapScale error
var ErrBitmapSize error
var ErrBitmapTableBorrowed error
var ErrBitmapTableClosed error
var ErrBitmapTableInUse error
var ErrBitmapTableSize error
var ErrButtonCallbackConfig error
var ErrDateTime error
var ErrDisplayMosaic error
var ErrDisplayRefreshRate error
var ErrDisplayScale error
var ErrDisplayUnavailable error
var ErrFileClosed error
var ErrFileIO error
var ErrFileMode error
var ErrFileOffset error
var ErrFilePath error
var ErrFileUnavailable error
var ErrFontClosed error
var ErrFontGlyph error
var ErrFontInvalid error
var ErrFontLoad error
var ErrFramebufferBounds error
var ErrFramebufferCallback error
var ErrFramebufferColor error
var ErrFramebufferExpired error
var ErrGraphicsColor error
var ErrGraphicsDrawMode error
var ErrGraphicsGeometry error
var ErrGraphicsLineCap error
var ErrGraphicsPolygon error
var ErrGraphicsStencilActive error
var ErrGraphicsStencilCallback error
var ErrGraphicsStencilWidth error
var ErrGraphicsUnavailable error
var ErrLaunchArguments error
var ErrMenuImageOffset error
var ErrMenuImageSize error
var ErrMenuItemCreate error
var ErrMenuOptions error
var ErrMenuTitle error
var ErrMenuValue error
var ErrMicrophoneCallback error
var ErrMicrophoneClosed error
var ErrMicrophoneDenied error
var ErrMicrophoneSamplesExpired error
var ErrMicrophoneSource error
var ErrMicrophoneStart error
var ErrMicrophoneUnavailable error
var ErrOffscreenCallback error
var ErrScoreboardBoardID error
var ErrScoreboardBusy error
var ErrScoreboardCallback error
var ErrScoreboardRequest error
var ErrScoreboardUnavailable error
var ErrSpriteBorrowed error
var ErrSpriteCallbackLimit error
var ErrSpriteClosed error
var ErrSpriteCreate error
var ErrSpriteDirtyRect error
var ErrSpriteDisplayListUnavailable error
var ErrSpriteRedrawUnavailable error
var ErrSpriteTileMapBorrowed error
var ErrSpriteTileMapBounds error
var ErrSpriteTileMapClosed error
var ErrSpriteTileMapConfig error
var ErrSpriteTileMapCreate error
var ErrSpriteTileMapInUse error
var ErrSpriteTileMapUnavailable error
var ErrSystemControlsUnavailable error
var ErrSystemEnvironmentUnavailable error
var ErrTileMapBitmap error
var ErrTileMapConfig error
var ErrTileMapDraw error
var ErrVideoClosed error
var ErrVideoFrame error
var ErrVideoLoad error
var ErrVideoPath error
var ErrVideoUnavailable error
`
