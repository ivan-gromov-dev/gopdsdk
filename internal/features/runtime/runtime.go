// Package runtime coordinates platform-independent Playdate application lifecycle.
package runtime

import (
	"errors"
	"io"
	"math"
	"path"
	"strings"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

// FileDriver contains platform operations for one owned native file.
type FileDriver struct {
	Read  func(uintptr, []byte) (int, string)
	Write func(uintptr, []byte) (int, string)
	Flush func(uintptr) (int, string)
	Tell  func(uintptr) (int, string)
	Seek  func(uintptr, int32, int) (int, string)
	Close func(uintptr) (int, string)
}

type ownedFile struct {
	handle uintptr
	path   string
	driver FileDriver
	closed bool
}

// NewOwnedFile wraps a Playdate file handle that must be explicitly closed.
func NewOwnedFile(handle uintptr, filePath string, driver FileDriver) playdate.File {
	return &ownedFile{handle: handle, path: filePath, driver: driver}
}

func (file *ownedFile) nativeHandle() (uintptr, error) {
	if file == nil || file.closed || file.handle == 0 {
		return 0, playdate.ErrFileClosed
	}
	return file.handle, nil
}

func fileFailure(operation, filePath, message string) error {
	return playdate.FileOperationError{Operation: operation, Path: filePath, Message: message}
}

func (file *ownedFile) Read(buffer []byte) (int, error) {
	handle, err := file.nativeHandle()
	if err != nil {
		return 0, err
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	count, message := file.driver.Read(handle, buffer)
	if count < 0 {
		return 0, fileFailure("read", file.path, message)
	}
	if count == 0 {
		return 0, io.EOF
	}
	return count, nil
}

func (file *ownedFile) Write(buffer []byte) (int, error) {
	handle, err := file.nativeHandle()
	if err != nil {
		return 0, err
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	count, message := file.driver.Write(handle, buffer)
	if count < 0 {
		return 0, fileFailure("write", file.path, message)
	}
	if count != len(buffer) {
		return count, io.ErrShortWrite
	}
	return count, nil
}

func (file *ownedFile) Flush() error {
	handle, err := file.nativeHandle()
	if err != nil {
		return err
	}
	if result, message := file.driver.Flush(handle); result < 0 {
		return fileFailure("flush", file.path, message)
	}
	return nil
}

func (file *ownedFile) Seek(offset int64, whence int) (int64, error) {
	handle, err := file.nativeHandle()
	if err != nil {
		return 0, err
	}
	if offset < math.MinInt32 || offset > math.MaxInt32 || whence < io.SeekStart || whence > io.SeekEnd {
		return 0, playdate.ErrFileOffset
	}
	if result, message := file.driver.Seek(handle, int32(offset), whence); result < 0 {
		return 0, fileFailure("seek", file.path, message)
	}
	position, message := file.driver.Tell(handle)
	if position < 0 {
		return 0, fileFailure("tell", file.path, message)
	}
	return int64(position), nil
}

func (file *ownedFile) Close() error {
	handle, err := file.nativeHandle()
	if err != nil {
		return err
	}
	file.closed = true
	file.handle = 0
	if result, message := file.driver.Close(handle); result < 0 {
		return fileFailure("close", file.path, message)
	}
	return nil
}

// ValidateFilePath applies the shared Playdate relative-path contract.
func ValidateFilePath(filePath string, allowRoot bool) error {
	if strings.IndexByte(filePath, 0) >= 0 || strings.Contains(filePath, "\\") || strings.HasPrefix(filePath, "/") {
		return playdate.ErrFilePath
	}
	cleaned := path.Clean(filePath)
	if cleaned == "." {
		if allowRoot {
			return nil
		}
		return playdate.ErrFilePath
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != filePath {
		return playdate.ErrFilePath
	}
	return nil
}

// ValidateFileOptions applies the official FileOptions combinations.
func ValidateFileOptions(options playdate.FileOptions) error {
	switch options {
	case playdate.FileReadPackage, playdate.FileReadData,
		playdate.FileReadPackage | playdate.FileReadData,
		playdate.FileWrite, playdate.FileAppend:
		return nil
	default:
		return playdate.ErrFileMode
	}
}

// Framebuffer provides a direct, callback-scoped view of native display memory.
type Framebuffer struct {
	data                 []byte
	width, height        int
	rowBytes             int
	active               bool
	dirtyStart, dirtyEnd int
}

// WithFramebuffer scopes a native framebuffer view and reports its combined
// dirty row range after successful or failed callback execution.
func WithFramebuffer(data []byte, width, height, rowBytes int, mark func(int, int), callback func(playdate.Framebuffer) error) error {
	if callback == nil {
		return playdate.ErrFramebufferCallback
	}
	frame := &Framebuffer{data: data, width: width, height: height, rowBytes: rowBytes, active: true, dirtyStart: height, dirtyEnd: -1}
	err := callback(frame)
	frame.active = false
	frame.data = nil
	if frame.dirtyEnd >= frame.dirtyStart && mark != nil {
		mark(frame.dirtyStart, frame.dirtyEnd)
	}
	return err
}

func (f *Framebuffer) valid() error {
	if f == nil || !f.active {
		return playdate.ErrFramebufferExpired
	}
	return nil
}

func (f *Framebuffer) Width() int    { return f.width }
func (f *Framebuffer) Height() int   { return f.height }
func (f *Framebuffer) RowBytes() int { return f.rowBytes }
func (f *Framebuffer) Bytes() ([]byte, error) {
	if err := f.valid(); err != nil {
		return nil, err
	}
	return f.data, nil
}
func (f *Framebuffer) Pixel(x, y int) (playdate.Color, error) {
	if err := f.valid(); err != nil {
		return 0, err
	}
	if x < 0 || x >= f.width || y < 0 || y >= f.height {
		return 0, playdate.ErrFramebufferBounds
	}
	if f.data[y*f.rowBytes+x/8]&(0x80>>uint(x&7)) != 0 {
		return playdate.ColorWhite, nil
	}
	return playdate.ColorBlack, nil
}
func (f *Framebuffer) SetPixel(x, y int, color playdate.Color) error {
	if err := f.valid(); err != nil {
		return err
	}
	if x < 0 || x >= f.width || y < 0 || y >= f.height {
		return playdate.ErrFramebufferBounds
	}
	if color != playdate.ColorBlack && color != playdate.ColorWhite {
		return playdate.ErrFramebufferColor
	}
	index, mask := y*f.rowBytes+x/8, byte(0x80>>uint(x&7))
	if color == playdate.ColorWhite {
		f.data[index] |= mask
	} else {
		f.data[index] &^= mask
	}
	return f.MarkDirtyRows(y, y)
}
func (f *Framebuffer) MarkDirtyRows(start, end int) error {
	if err := f.valid(); err != nil {
		return err
	}
	if start < 0 || end < start || end >= f.height {
		return playdate.ErrFramebufferBounds
	}
	if start < f.dirtyStart {
		f.dirtyStart = start
	}
	if end > f.dirtyEnd {
		f.dirtyEnd = end
	}
	return nil
}

// ValidatePrimitiveGeometry applies the shared primitive dimension contract.
func ValidatePrimitiveGeometry(width, height, lineWidth int, startAngle, endAngle float32) error {
	if width <= 0 || height <= 0 || lineWidth <= 0 || math.IsNaN(float64(startAngle)) || math.IsNaN(float64(endAngle)) || math.IsInf(float64(startAngle), 0) || math.IsInf(float64(endAngle), 0) {
		return playdate.ErrGraphicsGeometry
	}
	return nil
}

// ValidateDrawMode applies the shared draw-mode contract.
func ValidateDrawMode(mode playdate.DrawMode) error {
	if mode > playdate.DrawModeInverted {
		return playdate.ErrGraphicsDrawMode
	}
	return nil
}

// ValidateLineCapStyle applies the shared line-cap contract.
func ValidateLineCapStyle(style playdate.LineCapStyle) error {
	if style > playdate.LineCapRound {
		return playdate.ErrGraphicsLineCap
	}
	return nil
}

// ValidatePolygon applies the shared polygon contract.
func ValidatePolygon(points []playdate.GraphicsPoint, rule playdate.PolygonFillRule) error {
	if len(points) < 3 || rule > playdate.PolygonFillEvenOdd {
		return playdate.ErrGraphicsPolygon
	}
	return nil
}

// Event identifies an event delivered by the Playdate runtime.
type Event int32

// Event values mirror PDSystemEvent in the official Playdate C API.
const (
	EventInit Event = iota
	EventInitLua
	EventLock
	EventUnlock
	EventPause
	EventResume
	EventTerminate
	EventKeyPressed
	EventKeyReleased
	EventLowPower
	EventMirrorStarted
	EventMirrorEnded
)

var (
	// ErrInitRequired indicates a missing application initialization callback.
	ErrInitRequired = errors.New("runtime init callback is required")
	// ErrUpdateRequired indicates a missing application update callback.
	ErrUpdateRequired = errors.New("runtime update callback is required")
	// ErrAlreadyInitialized indicates a duplicate initialization event.
	ErrAlreadyInitialized = errors.New("runtime is already initialized")
	// ErrNotInitialized indicates an update before successful initialization.
	ErrNotInitialized = errors.New("runtime is not initialized")
	// ErrFailed indicates that an earlier callback permanently stopped runtime.
	ErrFailed = errors.New("runtime callback previously failed")
	// ErrTerminated indicates a callback after successful termination.
	ErrTerminated    = errors.New("runtime is terminated")
	ErrInvalidBitmap = errors.New("bitmap handle was not created by this runtime")
)

// FontDriver contains platform operations for an owned native font.
type FontDriver struct {
	TextWidth func(uintptr, string) int
	Height    func(uintptr) int
	Free      func(uintptr)
}

type font struct {
	handle uintptr
	driver FontDriver
	closed bool
}

// NewOwnedFont wraps a font returned by the Playdate loader.
func NewOwnedFont(handle uintptr, driver FontDriver) playdate.Font {
	return &font{handle: handle, driver: driver}
}

func (f *font) nativeHandle() (uintptr, error) {
	if f == nil || f.closed || f.handle == 0 {
		return 0, playdate.ErrFontClosed
	}
	return f.handle, nil
}

func (f *font) TextWidth(text string) (int, error) {
	handle, err := f.nativeHandle()
	if err != nil {
		return 0, err
	}
	return f.driver.TextWidth(handle, text), nil
}

func (f *font) Height() (int, error) {
	handle, err := f.nativeHandle()
	if err != nil {
		return 0, err
	}
	return f.driver.Height(handle), nil
}

func (f *font) Close() error {
	handle, err := f.nativeHandle()
	if err != nil {
		return err
	}
	f.driver.Free(handle)
	f.handle = 0
	f.closed = true
	return nil
}

// FontHandle validates and extracts a font created by this runtime.
func FontHandle(value playdate.Font) (uintptr, error) {
	f, ok := value.(*font)
	if !ok {
		return 0, playdate.ErrFontInvalid
	}
	return f.nativeHandle()
}

// AudioDriver contains the common native operations for either accepted player.
type AudioDriver struct {
	Source            func(uintptr) uintptr
	Play              func(uintptr) bool
	PlayRepeated      func(uintptr, int, float32) bool
	Stop              func(uintptr)
	SetVolume         func(uintptr, float32, float32)
	Volume            func(uintptr) (float32, float32)
	IsPlaying         func(uintptr) bool
	Pause             func(uintptr, bool)
	Length            func(uintptr) float32
	SetOffset         func(uintptr, float32)
	Offset            func(uintptr) float32
	SetRate           func(uintptr, float32)
	SetRateModulator  func(uintptr, uintptr)
	Rate              func(uintptr) float32
	SetFinishCallback func(uintptr, uint32)
	SetLoopCallback   func(uintptr, uint32)
	SetSample         func(uintptr, uintptr)
	SetPlayRange      func(uintptr, int, int)
	Load              func(uintptr, string) bool
	SetBufferLength   func(uintptr, float32)
	SetLoopRange      func(uintptr, float32, float32)
	DidUnderrun       func(uintptr) bool
	SetStopOnUnderrun func(uintptr, bool)
	FadeVolume        func(uintptr, float32, float32, uint32, uint32)
	Free              func(uintptr)
}

type audioPlayer struct {
	handle         uintptr
	driver         AudioDriver
	allowReverse   bool
	paused         bool
	closed         bool
	finishCallback uint32
	loopCallback   uint32
	fadeCallback   uint32
	sample         *audioSample
	channels       map[*audioChannel]struct{}
	rateSignal     *signalNode
	ownerChannel   *audioChannel
}

func (p *audioPlayer) SetRateModulator(v playdate.Signal) error {
	h, e := p.nativeHandle()
	if e != nil {
		return e
	}
	if p.driver.SetRateModulator == nil {
		return playdate.ErrAudioUnavailable
	}
	s, e := signalFrom(v)
	if e != nil {
		return e
	}
	var sh uintptr
	if s != nil {
		sh = s.handle
	}
	if p.rateSignal != nil {
		delete(p.rateSignal.players, p)
	}
	p.driver.SetRateModulator(h, sh)
	p.rateSignal = s
	if s != nil {
		s.players[p] = struct{}{}
	}
	return nil
}

// AudioChannelDriver contains native operations for an explicitly owned
// routing node.
type AudioChannelDriver struct {
	Output             func(uintptr) uintptr
	OutputAudio        AudioDriver
	DryLevelSignal     func(uintptr) uintptr
	WetLevelSignal     func(uintptr) uintptr
	LevelSignal        SignalDriver
	AddSource          func(uintptr, uintptr) bool
	RemoveSource       func(uintptr, uintptr) bool
	AddEffect          func(uintptr, uintptr) bool
	RemoveEffect       func(uintptr, uintptr) bool
	SetVolume          func(uintptr, float32)
	Volume             func(uintptr) float32
	SetPan             func(uintptr, float32)
	SetVolumeModulator func(uintptr, uintptr)
	SetPanModulator    func(uintptr, uintptr)
	Remove             func(uintptr) bool
	Free               func(uintptr)
}

// AudioOutputDriver contains native operations for the process-wide output
// graph. The default channel is borrowed and must never be removed or freed.
type AudioOutputDriver struct {
	DefaultChannel   func() uintptr
	OutputState      func() (headphones, headsetMicrophone bool)
	SetOutputsActive func(headphones, speaker bool)
	Channel          AudioChannelDriver
}

type audioChannel struct {
	handle                  uintptr
	driver                  AudioChannelDriver
	sources                 map[*audioPlayer]struct{}
	effects                 map[*effectNode]struct{}
	closed                  bool
	volumeSignal, panSignal *signalNode
	output                  *audioPlayer
	drySignal, wetSignal    *signalNode
}

func (c *audioChannel) setModulator(value playdate.Signal, pan bool) error {
	h, e := c.nativeHandle()
	if e != nil {
		return e
	}
	s, e := signalFrom(value)
	if e != nil {
		return e
	}
	var sh uintptr
	if s != nil {
		sh = s.handle
	}
	old := c.volumeSignal
	slot := uint8(1)
	set := c.driver.SetVolumeModulator
	if pan {
		old = c.panSignal
		slot = 2
		set = c.driver.SetPanModulator
	}
	if old != nil {
		old.channels[c] &^= slot
		if old.channels[c] == 0 {
			delete(old.channels, c)
		}
	}
	set(h, sh)
	if pan {
		c.panSignal = s
	} else {
		c.volumeSignal = s
	}
	if s != nil {
		s.channels[c] |= slot
	}
	return nil
}
func (c *audioChannel) SetVolumeModulator(v playdate.Signal) error { return c.setModulator(v, false) }
func (c *audioChannel) SetPanModulator(v playdate.Signal) error    { return c.setModulator(v, true) }

// NewAudioChannel wraps a native channel already added to the sound graph.
func NewAudioChannel(handle uintptr, driver AudioChannelDriver) playdate.AudioChannel {
	return &audioChannel{handle: handle, driver: driver, sources: make(map[*audioPlayer]struct{}), effects: make(map[*effectNode]struct{})}
}

// DefaultAudioChannel wraps the borrowed native default channel. Closing the
// wrapper detaches its tracked graph edges but does not remove or free it.
func DefaultAudioChannel(handle uintptr, driver AudioChannelDriver) playdate.AudioChannel {
	driver.Remove = nil
	driver.Free = nil
	return NewAudioChannel(handle, driver)
}

func (c *audioChannel) AddEffect(value playdate.AudioEffect) error {
	h, err := c.nativeHandle()
	if err != nil {
		return err
	}
	e, err := effectFrom(value)
	if err != nil {
		return err
	}
	eh, err := e.nativeHandle()
	if err != nil {
		return err
	}
	if _, ok := c.effects[e]; ok {
		return nil
	}
	if !c.driver.AddEffect(h, eh) {
		return playdate.ErrAudioRoute
	}
	c.effects[e] = struct{}{}
	e.channels[c] = struct{}{}
	return nil
}

func (c *audioChannel) RemoveEffect(value playdate.AudioEffect) error {
	h, err := c.nativeHandle()
	if err != nil {
		return err
	}
	e, err := effectFrom(value)
	if err != nil {
		return err
	}
	if _, ok := c.effects[e]; !ok {
		return nil
	}
	if !c.driver.RemoveEffect(h, e.handle) {
		return playdate.ErrAudioRoute
	}
	delete(c.effects, e)
	delete(e.channels, c)
	return nil
}

func (c *audioChannel) nativeHandle() (uintptr, error) {
	if c == nil || c.closed || c.handle == 0 {
		return 0, playdate.ErrAudioChannelClosed
	}
	return c.handle, nil
}

func (c *audioChannel) Output() (playdate.AudioSource, error) {
	handle, err := c.nativeHandle()
	if err != nil {
		return nil, err
	}
	if c.output != nil {
		return c.output, nil
	}
	if c.driver.Output == nil {
		return nil, playdate.ErrAudioUnavailable
	}
	output := c.driver.Output(handle)
	if output == 0 {
		return nil, playdate.ErrAudioUnavailable
	}
	c.output = &audioPlayer{handle: output, driver: c.driver.OutputAudio, channels: make(map[*audioChannel]struct{}), ownerChannel: c}
	return c.output, nil
}

func (c *audioChannel) levelSignal(wet bool) (playdate.Signal, error) {
	handle, err := c.nativeHandle()
	if err != nil {
		return nil, err
	}
	slot := &c.drySignal
	get := c.driver.DryLevelSignal
	kind := uint8(1)
	if wet {
		slot = &c.wetSignal
		get = c.driver.WetLevelSignal
		kind = 2
	}
	if *slot != nil {
		return *slot, nil
	}
	if get == nil {
		return nil, playdate.ErrAudioUnavailable
	}
	signal := get(handle)
	if signal == 0 {
		return nil, playdate.ErrAudioUnavailable
	}
	*slot = newBorrowedSignalNode(signal, c.driver.LevelSignal, c, kind)
	return *slot, nil
}

func (c *audioChannel) DryLevelSignal() (playdate.Signal, error) { return c.levelSignal(false) }
func (c *audioChannel) WetLevelSignal() (playdate.Signal, error) { return c.levelSignal(true) }

func audioSource(value playdate.AudioSource) (*audioPlayer, error) {
	switch source := value.(type) {
	case *audioPlayer:
		return source, nil
	case *filePlayer:
		return source.audioPlayer, nil
	case *synth:
		return source.audioPlayer, nil
	case *delayTap:
		return source.audioPlayer, nil
	case *instrument:
		return source.audioPlayer, nil
	default:
		return nil, playdate.ErrAudioSourceInvalid
	}
}

func channelReaches(from, target *audioChannel) bool {
	if from == target {
		return true
	}
	if from == nil || from.output == nil {
		return false
	}
	for next := range from.output.channels {
		if channelReaches(next, target) {
			return true
		}
	}
	return false
}

func (c *audioChannel) AddSource(value playdate.AudioSource) error {
	handle, err := c.nativeHandle()
	if err != nil {
		return err
	}
	if synth, ok := value.(*synth); ok && len(synth.instruments) != 0 {
		return playdate.ErrAudioRoute
	}
	source, err := audioSource(value)
	if err != nil {
		return err
	}
	if source.ownerChannel != nil && channelReaches(c, source.ownerChannel) {
		return playdate.ErrAudioRoute
	}
	sourceHandle, err := source.nativeHandle()
	if err != nil {
		return err
	}
	if _, ok := c.sources[source]; ok {
		if !c.driver.AddSource(handle, source.driver.Source(sourceHandle)) {
			return playdate.ErrAudioRoute
		}
		return nil
	}
	for previous := range source.channels {
		if previous.closed {
			delete(source.channels, previous)
			continue
		}
		if !previous.driver.RemoveSource(previous.handle, source.driver.Source(sourceHandle)) {
			return playdate.ErrAudioRoute
		}
		delete(previous.sources, source)
		delete(source.channels, previous)
	}
	if !c.driver.AddSource(handle, source.driver.Source(sourceHandle)) {
		return playdate.ErrAudioRoute
	}
	c.sources[source] = struct{}{}
	if source.channels == nil {
		source.channels = make(map[*audioChannel]struct{})
	}
	source.channels[c] = struct{}{}
	return nil
}

func (c *audioChannel) RemoveSource(value playdate.AudioSource) error {
	handle, err := c.nativeHandle()
	if err != nil {
		return err
	}
	source, err := audioSource(value)
	if err != nil {
		return err
	}
	if _, ok := c.sources[source]; !ok {
		return nil
	}
	sourceHandle, err := source.nativeHandle()
	if err != nil {
		return err
	}
	if !c.driver.RemoveSource(handle, source.driver.Source(sourceHandle)) {
		return playdate.ErrAudioRoute
	}
	delete(c.sources, source)
	delete(source.channels, c)
	return nil
}

func (c *audioChannel) SetVolume(volume float32) error {
	if err := ValidateAudioVolume(volume, volume); err != nil {
		return err
	}
	handle, err := c.nativeHandle()
	if err != nil {
		return err
	}
	c.driver.SetVolume(handle, volume)
	return nil
}
func (c *audioChannel) Volume() (float32, error) {
	handle, err := c.nativeHandle()
	if err != nil {
		return 0, err
	}
	return c.driver.Volume(handle), nil
}
func (c *audioChannel) SetPan(pan float32) error {
	if pan < -1 || pan > 1 || pan != pan {
		return playdate.ErrAudioPan
	}
	handle, err := c.nativeHandle()
	if err != nil {
		return err
	}
	c.driver.SetPan(handle, pan)
	return nil
}
func (c *audioChannel) Close() error {
	handle, err := c.nativeHandle()
	if err != nil {
		return err
	}
	if c.output != nil {
		for downstream := range c.output.channels {
			if !downstream.closed {
				downstream.driver.RemoveSource(downstream.handle, c.output.driver.Source(c.output.handle))
				delete(downstream.sources, c.output)
			}
		}
		c.output.channels = nil
		c.output.handle = 0
		c.output.closed = true
	}
	if c.drySignal != nil {
		_ = c.drySignal.Close()
	}
	if c.wetSignal != nil {
		_ = c.wetSignal.Close()
	}
	for source := range c.sources {
		sourceHandle, sourceErr := source.nativeHandle()
		if sourceErr == nil {
			c.driver.RemoveSource(handle, source.driver.Source(sourceHandle))
		}
		delete(source.channels, c)
	}
	c.sources = nil
	for effect := range c.effects {
		if !effect.closed {
			c.driver.RemoveEffect(handle, effect.handle)
		}
		delete(effect.channels, c)
	}
	c.effects = nil
	if c.volumeSignal != nil {
		delete(c.volumeSignal.channels, c)
		c.driver.SetVolumeModulator(handle, 0)
		c.volumeSignal = nil
	}
	if c.panSignal != nil {
		delete(c.panSignal.channels, c)
		c.driver.SetPanModulator(handle, 0)
		c.panSignal = nil
	}
	if c.driver.Remove != nil && !c.driver.Remove(handle) {
		return playdate.ErrAudioRoute
	}
	if c.driver.Free != nil {
		c.driver.Free(handle)
	}
	c.handle = 0
	c.closed = true
	return nil
}

type filePlayer struct{ *audioPlayer }

// AudioSampleDriver contains native operations for an owned sample buffer.
type AudioSampleDriver struct {
	Load       func(uintptr, string) bool
	Data       func(uintptr) ([]byte, playdate.SoundFormat, uint32)
	Length     func(uintptr) float32
	Decompress func(uintptr) bool
	Free       func(uintptr)
}

type audioSample struct {
	handle uintptr
	driver AudioSampleDriver
	users  map[*audioPlayer]struct{}
	closed bool
}

type sampleData struct {
	owner  *audioSample
	data   []byte
	format playdate.SoundFormat
	rate   uint32
}

func NewAudioSample(handle uintptr, driver AudioSampleDriver) playdate.AudioSample {
	return &audioSample{handle: handle, driver: driver, users: make(map[*audioPlayer]struct{})}
}

func (s *audioSample) nativeHandle() (uintptr, error) {
	if s == nil || s.closed || s.handle == 0 {
		return 0, playdate.ErrAudioSampleClosed
	}
	return s.handle, nil
}
func (s *audioSample) Load(path string) error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if !s.driver.Load(h, path) {
		return playdate.AudioLoadError(path)
	}
	return nil
}
func (s *audioSample) Data() (playdate.SampleData, error) {
	h, err := s.nativeHandle()
	if err != nil {
		return nil, err
	}
	data, format, rate := s.driver.Data(h)
	return &sampleData{owner: s, data: data, format: format, rate: rate}, nil
}
func (s *audioSample) Length() (float32, error) {
	h, err := s.nativeHandle()
	if err != nil {
		return 0, err
	}
	return s.driver.Length(h), nil
}
func (s *audioSample) Decompress() error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if !s.driver.Decompress(h) {
		return playdate.ErrAudioCreate
	}
	return nil
}
func (s *audioSample) Close() error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if len(s.users) != 0 {
		return playdate.ErrAudioSampleInUse
	}
	s.driver.Free(h)
	s.handle = 0
	s.closed = true
	return nil
}
func (d *sampleData) Len() int                     { return len(d.data) }
func (d *sampleData) Format() playdate.SoundFormat { return d.format }
func (d *sampleData) SampleRate() uint32           { return d.rate }
func (d *sampleData) CopyTo(dst []byte) (int, error) {
	if _, err := d.owner.nativeHandle(); err != nil {
		return 0, err
	}
	return copy(dst, d.data), nil
}

// NewSoundEffect wraps an owned sample player and its sample as one handle.
func NewSoundEffect(handle uintptr, driver AudioDriver) playdate.SoundEffect {
	return &audioPlayer{handle: handle, driver: driver, allowReverse: true}
}

// NewSamplePlayer wraps an owned sample player and its sample as one handle.
func NewSamplePlayer(handle uintptr, driver AudioDriver) playdate.SamplePlayer {
	return &audioPlayer{handle: handle, driver: driver, allowReverse: true}
}

// NewFilePlayer wraps an owned streaming file player.
func NewFilePlayer(handle uintptr, driver AudioDriver) playdate.FilePlayer {
	return &filePlayer{audioPlayer: &audioPlayer{handle: handle, driver: driver}}
}

func (p *audioPlayer) nativeHandle() (uintptr, error) {
	if p == nil || p.closed || p.handle == 0 {
		return 0, playdate.ErrAudioClosed
	}
	return p.handle, nil
}

func (p *audioPlayer) Play() error {
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	if !p.driver.Play(handle) {
		return playdate.ErrAudioPlay
	}
	p.paused = false
	return nil
}

func (p *audioPlayer) PlayRepeated(repeat int, rate float32) error {
	if repeat < 0 || repeat > 2147483647 {
		return playdate.ErrAudioRepeat
	}
	if err := ValidateAudioRate(rate); err != nil {
		return err
	}
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	if !p.driver.PlayRepeated(handle, repeat, rate) {
		return playdate.ErrAudioPlay
	}
	p.paused = false
	return nil
}

func (p *audioPlayer) Length() (float32, error) {
	handle, err := p.nativeHandle()
	if err != nil {
		return 0, err
	}
	return p.driver.Length(handle), nil
}

func (p *audioPlayer) SetOffset(seconds float32) error {
	if seconds < 0 || seconds != seconds || seconds > 3.4028235e38 {
		return playdate.ErrAudioOffset
	}
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.SetOffset(handle, seconds)
	return nil
}

func (p *audioPlayer) Offset() (float32, error) {
	handle, err := p.nativeHandle()
	if err != nil {
		return 0, err
	}
	return p.driver.Offset(handle), nil
}

func (p *audioPlayer) SetRate(rate float32) error {
	if err := ValidateAudioRate(rate); err != nil {
		return err
	}
	if rate < 0 && !p.allowReverse {
		return playdate.ErrAudioReverseUnsupported
	}
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.SetRate(handle, rate)
	return nil
}

func (p *audioPlayer) Rate() (float32, error) {
	handle, err := p.nativeHandle()
	if err != nil {
		return 0, err
	}
	return p.driver.Rate(handle), nil
}

// SetFinishCallback replaces the callback retained for natural completion.
func (p *audioPlayer) SetFinishCallback(callback func()) error {
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	ForgetAudioCallback(p.finishCallback)
	p.finishCallback = RegisterAudioCallback(callback)
	p.driver.SetFinishCallback(handle, p.finishCallback)
	return nil
}

func (p *audioPlayer) SetLoopCallback(callback func()) error {
	h, err := p.nativeHandle()
	if err != nil {
		return err
	}
	ForgetAudioCallback(p.loopCallback)
	p.loopCallback = RegisterAudioCallback(callback)
	p.driver.SetLoopCallback(h, p.loopCallback)
	return nil
}
func (p *audioPlayer) SetSample(value playdate.AudioSample) error {
	h, err := p.nativeHandle()
	if err != nil {
		return err
	}
	s, ok := value.(*audioSample)
	if !ok {
		return playdate.ErrAudioSourceInvalid
	}
	sh, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if p.sample != nil {
		delete(p.sample.users, p)
	}
	p.driver.SetSample(h, sh)
	p.sample = s
	s.users[p] = struct{}{}
	return nil
}
func (p *audioPlayer) SetPlayRange(start, end int) error {
	if start < 0 || end <= start || end > 2147483647 {
		return playdate.ErrAudioRange
	}
	h, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.SetPlayRange(h, start, end)
	return nil
}

func (p *filePlayer) Load(path string) error {
	h, err := p.nativeHandle()
	if err != nil {
		return err
	}
	if !p.driver.Load(h, path) {
		return playdate.AudioLoadError(path)
	}
	return nil
}
func (p *filePlayer) SetBufferLength(seconds float32) error {
	if seconds <= 0 || !finite(seconds) {
		return playdate.ErrAudioBufferLength
	}
	h, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.SetBufferLength(h, seconds)
	return nil
}
func (p *filePlayer) SetLoopRange(start, end float32) error {
	if start < 0 || end <= start || !finite(start) || !finite(end) {
		return playdate.ErrAudioRange
	}
	h, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.SetLoopRange(h, start, end)
	return nil
}
func (p *filePlayer) DidUnderrun() (bool, error) {
	h, err := p.nativeHandle()
	if err != nil {
		return false, err
	}
	return p.driver.DidUnderrun(h), nil
}
func (p *filePlayer) SetStopOnUnderrun(value bool) error {
	h, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.SetStopOnUnderrun(h, value)
	return nil
}

// FadeVolume starts a streaming-player volume fade measured in audio frames.
func (p *filePlayer) FadeVolume(left, right float32, audioFrames uint32, callback func()) error {
	if err := ValidateAudioVolume(left, right); err != nil {
		return err
	}
	if audioFrames > 2147483647 {
		return playdate.ErrAudioFade
	}
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	ForgetAudioCallback(p.fadeCallback)
	p.fadeCallback = RegisterAudioCallback(callback)
	p.driver.FadeVolume(handle, left, right, audioFrames, p.fadeCallback)
	return nil
}

func (p *audioPlayer) Stop() error {
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.Stop(handle)
	p.paused = false
	return nil
}

func (p *audioPlayer) SetVolume(left, right float32) error {
	if err := ValidateAudioVolume(left, right); err != nil {
		return err
	}
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.SetVolume(handle, left, right)
	return nil
}

func (p *audioPlayer) Volume() (float32, float32, error) {
	handle, err := p.nativeHandle()
	if err != nil {
		return 0, 0, err
	}
	left, right := p.driver.Volume(handle)
	return left, right, nil
}

func (p *audioPlayer) State() (playdate.PlaybackState, error) {
	handle, err := p.nativeHandle()
	if err != nil {
		return 0, err
	}
	if p.paused {
		return playdate.PlaybackPaused, nil
	}
	if p.driver.IsPlaying(handle) {
		return playdate.PlaybackPlaying, nil
	}
	return playdate.PlaybackStopped, nil
}

func (p *audioPlayer) Pause() error {
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	if p.driver.IsPlaying(handle) {
		p.driver.Pause(handle, true)
		p.paused = true
	}
	return nil
}

func (p *audioPlayer) Resume() error {
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	if p.paused {
		p.driver.Pause(handle, false)
		p.paused = false
	}
	return nil
}

func (p *audioPlayer) Close() error {
	handle, err := p.nativeHandle()
	if err != nil {
		return err
	}
	p.driver.Stop(handle)
	if p.sample != nil {
		delete(p.sample.users, p)
		p.sample = nil
	}
	ForgetAudioCallback(p.loopCallback)
	for channel := range p.channels {
		if !channel.driver.RemoveSource(channel.handle, p.driver.Source(handle)) {
			return playdate.ErrAudioRoute
		}
		delete(channel.sources, p)
		delete(p.channels, channel)
	}
	p.channels = nil
	if p.rateSignal != nil {
		delete(p.rateSignal.players, p)
		p.driver.SetRateModulator(handle, 0)
		p.rateSignal = nil
	}
	if p.driver.SetFinishCallback != nil {
		p.driver.SetFinishCallback(handle, 0)
	}
	ForgetAudioCallback(p.finishCallback)
	ForgetAudioCallback(p.fadeCallback)
	p.finishCallback, p.fadeCallback = 0, 0
	p.driver.Free(handle)
	p.handle = 0
	p.closed = true
	p.paused = false
	return nil
}

// ValidateAudioVolume applies the shared Simulator/device volume contract.
func ValidateAudioVolume(left, right float32) error {
	if left < 0 || left > 1 || right < 0 || right > 1 || left != left || right != right {
		return playdate.ErrAudioVolume
	}
	return nil
}

// ValidateAudioRate rejects values that the native audio API cannot use.
func ValidateAudioRate(rate float32) error {
	if rate == 0 || rate != rate || rate > 3.4028235e38 || rate < -3.4028235e38 {
		return playdate.ErrAudioRate
	}
	return nil
}

// BitmapDriver contains platform operations for one native bitmap handle.
type BitmapDriver struct {
	Dimensions func(uintptr) (width, height int)
	Data       func(uintptr) (width, height, rowBytes int, mask, data []byte)
	Fill       func(uintptr, playdate.Color)
	Free       func(uintptr)
}

// Bitmap owns or borrows a native Playdate bitmap.
type Bitmap struct {
	handle               uintptr
	driver               BitmapDriver
	table                *BitmapTable
	parent               *Bitmap
	font                 *font
	mask                 *Bitmap
	maskUsers, maskViews int
	menuImageUsers       int
	owned                bool
	closed               bool
}

// MenuImageState retains the owned bitmap used by the native system menu.
type MenuImageState struct {
	image *Bitmap
	set   func(uintptr, int)
}

// NewMenuImageState creates native menu-image state with deterministic bitmap
// ownership and validation.
func NewMenuImageState(set func(uintptr, int)) *MenuImageState {
	return &MenuImageState{set: set}
}

// SetMenuImage validates and retains a 400x240 owned bitmap.
func (state *MenuImageState) SetMenuImage(bitmap playdate.Bitmap, xOffset int) error {
	if xOffset < 0 || xOffset > 200 {
		return playdate.ErrMenuImageOffset
	}
	value, ok := bitmap.(*Bitmap)
	if !ok {
		return ErrInvalidBitmap
	}
	if !value.owned {
		return playdate.ErrBitmapBorrowed
	}
	handle, err := value.nativeHandle()
	if err != nil {
		return err
	}
	width, height := value.driver.Dimensions(handle)
	if width != 400 || height != 240 {
		return playdate.ErrMenuImageSize
	}
	if state.image != value {
		if state.image != nil {
			state.image.menuImageUsers--
		}
		value.menuImageUsers++
		state.image = value
	}
	state.set(handle, xOffset)
	return nil
}

// ClearMenuImage releases the retained menu bitmap after clearing native state.
func (state *MenuImageState) ClearMenuImage() {
	state.set(0, 0)
	if state.image != nil {
		state.image.menuImageUsers--
		state.image = nil
	}
}

// BitmapTableDriver contains platform operations for one native bitmap table.
type BitmapTableDriver struct {
	Frame func(table uintptr, index int) uintptr
	Free  func(uintptr)
}

// BitmapData is valid only during WithBitmapData.
type BitmapData struct {
	width, height, rowBytes int
	data, mask              []byte
	valid                   bool
	dirty                   bool
}

func (d *BitmapData) Width() int    { return d.width }
func (d *BitmapData) Height() int   { return d.height }
func (d *BitmapData) RowBytes() int { return d.rowBytes }
func (d *BitmapData) Bytes() ([]byte, error) {
	if !d.valid {
		return nil, playdate.ErrBitmapDataExpired
	}
	return d.data, nil
}
func (d *BitmapData) MaskBytes() ([]byte, error) {
	if !d.valid {
		return nil, playdate.ErrBitmapDataExpired
	}
	return d.mask, nil
}
func (d *BitmapData) Dirty() (bool, error) {
	if !d.valid {
		return false, playdate.ErrBitmapDataExpired
	}
	return d.dirty, nil
}
func (d *BitmapData) MarkDirty() error {
	if !d.valid {
		return playdate.ErrBitmapDataExpired
	}
	d.dirty = true
	return nil
}
func (d *BitmapData) Pixel(x, y int) (playdate.Color, error) {
	if !d.valid {
		return 0, playdate.ErrBitmapDataExpired
	}
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return 0, playdate.ErrBitmapBounds
	}
	if d.data[y*d.rowBytes+x/8]&(0x80>>uint(x&7)) != 0 {
		return playdate.ColorBlack, nil
	}
	return playdate.ColorWhite, nil
}
func (d *BitmapData) SetPixel(x, y int, color playdate.Color) error {
	if !d.valid {
		return playdate.ErrBitmapDataExpired
	}
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return playdate.ErrBitmapBounds
	}
	if color != playdate.ColorWhite && color != playdate.ColorBlack {
		return playdate.ErrBitmapColor
	}
	index, bit := y*d.rowBytes+x/8, byte(0x80>>uint(x&7))
	if color == playdate.ColorBlack {
		d.data[index] |= bit
	} else {
		d.data[index] &^= bit
	}
	d.dirty = true
	return nil
}

// WithBitmapData exposes native bitmap storage for one callback.
func WithBitmapData(bitmap playdate.Bitmap, callback func(playdate.BitmapData) error) error {
	if callback == nil {
		return playdate.ErrBitmapDataCallback
	}
	value, ok := bitmap.(*Bitmap)
	if !ok {
		return ErrInvalidBitmap
	}
	if !value.owned {
		return playdate.ErrBitmapBorrowed
	}
	handle, err := value.nativeHandle()
	if err != nil {
		return err
	}
	width, height, rowBytes, mask, data := value.driver.Data(handle)
	view := &BitmapData{width: width, height: height, rowBytes: rowBytes, mask: mask, data: data, valid: true}
	err = callback(view)
	view.valid = false
	view.data, view.mask = nil, nil
	return err
}

// BitmapTable owns or borrows a native Playdate bitmap table.
type BitmapTable struct {
	handle        uintptr
	driver        BitmapTableDriver
	bitmapDriver  BitmapDriver
	tileMapUsers  int
	owned, closed bool
}

// NewOwnedBitmapTable wraps a table and its borrowed frames.
func NewOwnedBitmapTable(handle uintptr, driver BitmapTableDriver, bitmapDriver BitmapDriver) *BitmapTable {
	return &BitmapTable{handle: handle, driver: driver, bitmapDriver: bitmapDriver, owned: true}
}

// NewBorrowedBitmapTable wraps a table controlled by Playdate.
func NewBorrowedBitmapTable(handle uintptr, driver BitmapTableDriver, bitmapDriver BitmapDriver) *BitmapTable {
	return &BitmapTable{handle: handle, driver: driver, bitmapDriver: bitmapDriver}
}

func (t *BitmapTable) Frame(index int) (playdate.Bitmap, error) {
	if t == nil || t.closed || t.handle == 0 {
		return nil, playdate.ErrBitmapTableClosed
	}
	if index < 0 {
		return nil, playdate.ErrBitmapFrameRange
	}
	handle := t.driver.Frame(t.handle, index)
	if handle == 0 {
		return nil, playdate.ErrBitmapFrameRange
	}
	return &Bitmap{handle: handle, driver: t.bitmapDriver, table: t}, nil
}

func (t *BitmapTable) Close() error {
	if t == nil || t.closed || t.handle == 0 {
		return playdate.ErrBitmapTableClosed
	}
	if !t.owned {
		return playdate.ErrBitmapTableBorrowed
	}
	if t.tileMapUsers != 0 {
		return playdate.ErrBitmapTableInUse
	}
	t.driver.Free(t.handle)
	t.closed = true
	t.handle = 0
	return nil
}

// NewOwnedBitmap wraps a bitmap that must be explicitly closed.
func NewOwnedBitmap(handle uintptr, driver BitmapDriver) *Bitmap {
	return &Bitmap{handle: handle, driver: driver, owned: true}
}

// NewBorrowedBitmap wraps a bitmap whose lifetime is controlled by Playdate.
func NewBorrowedBitmap(handle uintptr, driver BitmapDriver) *Bitmap {
	return &Bitmap{handle: handle, driver: driver}
}

// NewBorrowedFontBitmap wraps a glyph bitmap controlled by an owned font.
func NewBorrowedFontBitmap(owner playdate.Font, handle uintptr, driver BitmapDriver) (*Bitmap, error) {
	value, ok := owner.(*font)
	if !ok {
		return nil, playdate.ErrFontInvalid
	}
	if _, err := value.nativeHandle(); err != nil {
		return nil, err
	}
	return &Bitmap{handle: handle, driver: driver, font: value}, nil
}

func (b *Bitmap) nativeHandle() (uintptr, error) {
	if b == nil || b.closed || b.handle == 0 || b.table != nil && b.table.closed || b.parent != nil && (b.parent.closed || b.parent.handle == 0) || b.font != nil && (b.font.closed || b.font.handle == 0) {
		return 0, playdate.ErrBitmapClosed
	}
	return b.handle, nil
}

func (b *Bitmap) Width() (int, error)  { width, _, err := b.dimensions(); return width, err }
func (b *Bitmap) Height() (int, error) { _, height, err := b.dimensions(); return height, err }
func (b *Bitmap) dimensions() (int, int, error) {
	handle, err := b.nativeHandle()
	if err != nil {
		return 0, 0, err
	}
	w, h := b.driver.Dimensions(handle)
	return w, h, nil
}
func (b *Bitmap) Clear() error { return b.Fill(playdate.ColorClear) }
func (b *Bitmap) Fill(color playdate.Color) error {
	handle, err := b.nativeHandle()
	if err != nil {
		return err
	}
	if color > playdate.ColorBlack {
		return playdate.ErrBitmapColor
	}
	b.driver.Fill(handle, color)
	return nil
}
func (b *Bitmap) Close() error {
	if b == nil || b.closed || b.handle == 0 {
		return playdate.ErrBitmapClosed
	}
	if !b.owned {
		return playdate.ErrBitmapBorrowed
	}
	if b.maskUsers > 0 || b.maskViews > 0 {
		return playdate.ErrBitmapMaskInUse
	}
	if b.menuImageUsers > 0 {
		return playdate.ErrBitmapMenuImageInUse
	}
	b.driver.Free(b.handle)
	if b.parent != nil {
		b.parent.maskViews--
	}
	if b.mask != nil {
		b.mask.maskUsers--
		b.mask = nil
	}
	b.closed = true
	b.handle = 0
	return nil
}

// BitmapHandle validates and extracts a runtime bitmap for platform drawing.
func BitmapHandle(bitmap playdate.Bitmap) (uintptr, error) {
	value, ok := bitmap.(*Bitmap)
	if !ok {
		return 0, ErrInvalidBitmap
	}
	return value.nativeHandle()
}

// OwnedBitmapHandle validates a bitmap suitable for an offscreen context.
func OwnedBitmapHandle(bitmap playdate.Bitmap) (uintptr, error) {
	value, ok := bitmap.(*Bitmap)
	if !ok {
		return 0, ErrInvalidBitmap
	}
	if !value.owned {
		return 0, playdate.ErrBitmapBorrowed
	}
	return value.nativeHandle()
}

// SetBitmapMask validates ownership and dimensions before changing a mask.
func SetBitmapMask(bitmap, mask playdate.Bitmap, set func(uintptr, uintptr) bool) error {
	target, ok := bitmap.(*Bitmap)
	if !ok {
		return ErrInvalidBitmap
	}
	if !target.owned {
		return playdate.ErrBitmapBorrowed
	}
	targetHandle, err := target.nativeHandle()
	if err != nil {
		return err
	}
	maskValue, ok := mask.(*Bitmap)
	if !ok {
		return ErrInvalidBitmap
	}
	maskHandle, err := maskValue.nativeHandle()
	if err != nil {
		return err
	}
	tw, th := target.driver.Dimensions(targetHandle)
	mw, mh := maskValue.driver.Dimensions(maskHandle)
	if tw != mw || th != mh {
		return playdate.ErrBitmapMaskSize
	}
	if !set(targetHandle, maskHandle) {
		return playdate.ErrBitmapMask
	}
	if target.mask != nil {
		target.mask.maskUsers--
	}
	target.mask = maskValue
	maskValue.maskUsers++
	return nil
}

// ClearBitmapMask removes the association without closing either bitmap.
func ClearBitmapMask(bitmap playdate.Bitmap, set func(uintptr, uintptr) bool) error {
	target, ok := bitmap.(*Bitmap)
	if !ok {
		return ErrInvalidBitmap
	}
	if !target.owned {
		return playdate.ErrBitmapBorrowed
	}
	handle, err := target.nativeHandle()
	if err != nil {
		return err
	}
	if !set(handle, 0) {
		return playdate.ErrBitmapMask
	}
	if target.mask != nil {
		target.mask.maskUsers--
	}
	target.mask = nil
	return nil
}

// BitmapMask returns a borrowed view tied to the target bitmap lifetime.
func BitmapMask(bitmap playdate.Bitmap, get func(uintptr) uintptr) (playdate.Bitmap, bool, error) {
	target, ok := bitmap.(*Bitmap)
	if !ok {
		return nil, false, ErrInvalidBitmap
	}
	handle, err := target.nativeHandle()
	if err != nil {
		return nil, false, err
	}
	mask := get(handle)
	if mask == 0 {
		return nil, false, nil
	}
	target.maskViews++
	return &Bitmap{handle: mask, driver: target.driver, parent: target, owned: true}, true, nil
}

// OwnedBitmapTableHandle validates a table that native code may replace.
func OwnedBitmapTableHandle(table playdate.BitmapTable) (uintptr, error) {
	value, ok := table.(*BitmapTable)
	if !ok {
		return 0, ErrInvalidBitmap
	}
	if value.closed || value.handle == 0 {
		return 0, playdate.ErrBitmapTableClosed
	}
	if !value.owned {
		return 0, playdate.ErrBitmapTableBorrowed
	}
	return value.handle, nil
}

// VideoDriver contains platform operations for one native video player.
type VideoDriver struct {
	Info             func(uintptr) playdate.VideoInfo
	Error            func(uintptr) string
	SetContext       func(uintptr, uintptr) (bool, string)
	UseScreenContext func(uintptr)
	RenderFrame      func(uintptr, int) (bool, string)
	Free             func(uintptr)
}

// VideoPlayer owns a native PDV decoder.
type VideoPlayer struct {
	handle  uintptr
	driver  VideoDriver
	context *Bitmap
	closed  bool
}

// NewVideoPlayer wraps an owned native PDV decoder.
func NewVideoPlayer(handle uintptr, driver VideoDriver) *VideoPlayer {
	return &VideoPlayer{handle: handle, driver: driver}
}

func (p *VideoPlayer) live() error {
	if p == nil || p.closed || p.handle == 0 {
		return playdate.ErrVideoClosed
	}
	return nil
}
func (p *VideoPlayer) Info() (playdate.VideoInfo, error) {
	if err := p.live(); err != nil {
		return playdate.VideoInfo{}, err
	}
	return p.driver.Info(p.handle), nil
}
func (p *VideoPlayer) LastError() (string, error) {
	if err := p.live(); err != nil {
		return "", err
	}
	return p.driver.Error(p.handle), nil
}
func (p *VideoPlayer) SetContext(bitmap playdate.Bitmap) error {
	if err := p.live(); err != nil {
		return err
	}
	value, ok := bitmap.(*Bitmap)
	if !ok {
		return ErrInvalidBitmap
	}
	handle, err := OwnedBitmapHandle(bitmap)
	if err != nil {
		return err
	}
	if ok, message := p.driver.SetContext(p.handle, handle); !ok {
		return playdate.VideoOperationError{Operation: "set context", Message: message}
	}
	p.context = value
	return nil
}
func (p *VideoPlayer) UseScreenContext() error {
	if err := p.live(); err != nil {
		return err
	}
	p.driver.UseScreenContext(p.handle)
	p.context = nil
	return nil
}
func (p *VideoPlayer) RenderFrame(frame int) error {
	if err := p.live(); err != nil {
		return err
	}
	info := p.driver.Info(p.handle)
	if frame < 0 || frame >= info.FrameCount {
		return playdate.ErrVideoFrame
	}
	if p.context != nil {
		if _, err := p.context.nativeHandle(); err != nil {
			return err
		}
	}
	if ok, message := p.driver.RenderFrame(p.handle, frame); !ok {
		return playdate.VideoOperationError{Operation: "render frame", Message: message}
	}
	return nil
}
func (p *VideoPlayer) Close() error {
	if err := p.live(); err != nil {
		return err
	}
	p.driver.Free(p.handle)
	p.closed = true
	p.handle = 0
	p.context = nil
	return nil
}

// DisplayDriver contains platform display presentation operations.
type DisplayDriver struct {
	Width          func() int
	Height         func() int
	RefreshRate    func() float32
	FPS            func() float32
	SetRefreshRate func(float32)
	SetInverted    func(bool)
	SetScale       func(uint)
	SetMosaic      func(uint, uint)
	SetFlipped     func(bool, bool)
	SetOffset      func(int, int)
}

// Display validates public display settings before forwarding them to an ABI.
type Display struct{ driver DisplayDriver }

// NewDisplay creates a display capability backed by platform operations.
func NewDisplay(driver DisplayDriver) *Display { return &Display{driver: driver} }

func (d *Display) Width() int           { return d.driver.Width() }
func (d *Display) Height() int          { return d.driver.Height() }
func (d *Display) RefreshRate() float32 { return d.driver.RefreshRate() }
func (d *Display) FPS() float32         { return d.driver.FPS() }

func (d *Display) SetRefreshRate(rate float32) error {
	if math.IsNaN(float64(rate)) || math.IsInf(float64(rate), 0) || rate < 0 || rate > 50 {
		return playdate.ErrDisplayRefreshRate
	}
	d.driver.SetRefreshRate(rate)
	return nil
}
func (d *Display) SetInverted(value bool) { d.driver.SetInverted(value) }
func (d *Display) SetScale(scale uint) error {
	if scale != 1 && scale != 2 && scale != 4 && scale != 8 {
		return playdate.ErrDisplayScale
	}
	d.driver.SetScale(scale)
	return nil
}
func (d *Display) SetMosaic(x, y uint) error {
	if x > 3 || y > 3 {
		return playdate.ErrDisplayMosaic
	}
	d.driver.SetMosaic(x, y)
	return nil
}
func (d *Display) SetFlipped(x, y bool) { d.driver.SetFlipped(x, y) }
func (d *Display) SetOffset(x, y int)   { d.driver.SetOffset(x, y) }

// SpriteDriver contains platform operations for one native sprite handle.
type SpriteDriver struct {
	DisplayList          *SpriteDisplayListState
	SetBitmap            func(sprite, bitmap uintptr)
	SetCenter            func(uintptr, float32, float32)
	Center               func(uintptr) (float32, float32)
	SetBounds            func(uintptr, playdate.Rect)
	Bounds               func(uintptr) playdate.Rect
	MoveTo               func(uintptr, float32, float32)
	Position             func(uintptr) (float32, float32)
	MoveBy               func(uintptr, float32, float32)
	SetVisible           func(uintptr, bool)
	Visible              func(uintptr) bool
	SetZIndex            func(uintptr, int)
	ZIndex               func(uintptr) int
	SetImageFlip         func(uintptr, playdate.BitmapFlip)
	ImageFlip            func(uintptr) playdate.BitmapFlip
	SetDrawMode          func(uintptr, playdate.DrawMode)
	SetOpaque            func(uintptr, bool)
	SetStencilImage      func(uintptr, uintptr, bool)
	SetStencilPattern    func(uintptr, [8]byte)
	ClearStencil         func(uintptr)
	SetClipRect          func(uintptr, int, int, int, int)
	ClearClipRect        func(uintptr)
	SetIgnoresDrawOffset func(uintptr, bool)
	SetTileMap           func(uintptr, uintptr)
	TileMap              func(uintptr) uintptr
	TileMaps             SpriteTileMapDriver
	SetUpdatesEnabled    func(uintptr, bool)
	UpdatesEnabled       func(uintptr) bool
	SetCollisionsEnabled func(uintptr, bool)
	CollisionsEnabled    func(uintptr) bool
	SetCollideRect       func(uintptr, playdate.Rect)
	CollideRect          func(uintptr) playdate.Rect
	ClearCollideRect     func(uintptr)
	SetTag               func(uintptr, uint8)
	Tag                  func(uintptr) uint8
	MarkDirty            func(uintptr)
	MarkDirtyRect        func(uintptr, playdate.Rect)
	MoveWithCollisions   func(uintptr, float32, float32) (float32, float32, []NativeCollision)
	CheckCollisions      func(uintptr, float32, float32) (float32, float32, []NativeCollision)
	SetDrawCallback      func(uintptr, bool)
	SetUpdateCallback    func(uintptr, bool)
	SetCollisionCallback func(uintptr, bool)
	Add                  func(uintptr)
	Remove               func(uintptr)
	Free                 func(uintptr)
}

// NativeCollision is the adapter-facing representation of SpriteCollisionInfo.
type NativeCollision struct {
	Other                 uintptr
	ResponseType          playdate.CollisionResponse
	Overlaps              bool
	Time                  float32
	Move, Normal, Touch   playdate.Point
	SpriteRect, OtherRect playdate.Rect
}

// Sprite owns a native Playdate sprite.
type Sprite struct {
	handle            uintptr
	driver            SpriteDriver
	added             bool
	closed            bool
	owned             bool
	tileMap           *SpriteTileMap
	drawCallback      playdate.SpriteDrawCallback
	updateCallback    playdate.SpriteUpdateCallback
	collisionCallback playdate.SpriteCollisionResponseCallback
}

// SpriteDisplayListState tracks owned wrappers so a native remove-all can keep
// their deterministic Add/Remove/Close state synchronized.
type SpriteDisplayListState struct {
	sprites   map[*Sprite]struct{}
	byHandle  map[uintptr]*Sprite
	callbacks int
	active    bool
}

const spriteCallbackLimit = 64

// NewSpriteDisplayListState creates state shared by one native context.
func NewSpriteDisplayListState() *SpriteDisplayListState {
	return &SpriteDisplayListState{sprites: make(map[*Sprite]struct{}), byHandle: make(map[uintptr]*Sprite), active: true}
}

// RemoveAll marks every live owned wrapper as detached.
func (state *SpriteDisplayListState) RemoveAll() {
	if state == nil {
		return
	}
	for sprite := range state.sprites {
		sprite.added = false
	}
}

// NewOwnedSprite wraps a sprite that must be explicitly closed.
func NewOwnedSprite(handle uintptr, driver SpriteDriver) *Sprite {
	sprite := &Sprite{handle: handle, driver: driver, owned: true}
	if driver.DisplayList != nil {
		driver.DisplayList.sprites[sprite] = struct{}{}
		driver.DisplayList.byHandle[handle] = sprite
	}
	return sprite
}

func (s *Sprite) setCallback(previous, next bool) error {
	state := s.driver.DisplayList
	if state == nil || previous == next {
		return nil
	}
	if next && state.callbacks >= spriteCallbackLimit {
		return playdate.ErrSpriteCallbackLimit
	}
	if next {
		state.callbacks++
	} else {
		state.callbacks--
	}
	return nil
}

func (s *Sprite) SetDrawCallback(callback playdate.SpriteDrawCallback) error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if err = s.setCallback(s.drawCallback != nil, callback != nil); err != nil {
		return err
	}
	s.drawCallback = callback
	s.driver.SetDrawCallback(h, callback != nil)
	return nil
}
func (s *Sprite) SetUpdateCallback(callback playdate.SpriteUpdateCallback) error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if err = s.setCallback(s.updateCallback != nil, callback != nil); err != nil {
		return err
	}
	s.updateCallback = callback
	s.driver.SetUpdateCallback(h, callback != nil)
	return nil
}
func (s *Sprite) SetCollisionResponseCallback(callback playdate.SpriteCollisionResponseCallback) error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if err = s.setCallback(s.collisionCallback != nil, callback != nil); err != nil {
		return err
	}
	s.collisionCallback = callback
	s.driver.SetCollisionCallback(h, callback != nil)
	return nil
}

// InvokeSpriteDraw is called synchronously by a native sprite draw function.
func (state *SpriteDisplayListState) InvokeSpriteDraw(handle uintptr, bounds, drawRect playdate.Rect) {
	if state == nil || !state.active {
		return
	}
	if s := state.byHandle[handle]; s != nil && s.drawCallback != nil {
		s.drawCallback(s, bounds, drawRect)
	}
}

// InvokeSpriteUpdate is called synchronously by a native sprite update function.
func (state *SpriteDisplayListState) InvokeSpriteUpdate(handle uintptr) {
	if state == nil || !state.active {
		return
	}
	if s := state.byHandle[handle]; s != nil && s.updateCallback != nil {
		s.updateCallback(s)
	}
}

// InvokeSpriteCollision returns the pair-specific native collision response.
func (state *SpriteDisplayListState) InvokeSpriteCollision(handle, other uintptr) playdate.CollisionResponse {
	if state == nil || !state.active {
		return playdate.CollisionSlide
	}
	s := state.byHandle[handle]
	if s == nil || s.collisionCallback == nil {
		return playdate.CollisionSlide
	}
	response := s.collisionCallback(s, NewBorrowedSprite(other, s.driver))
	if response > playdate.CollisionBounce {
		return playdate.CollisionSlide
	}
	return response
}

// TerminateSpriteCallbacks suppresses callbacks after lifecycle termination.
func (state *SpriteDisplayListState) TerminateSpriteCallbacks() {
	if state != nil {
		state.active = false
	}
}

// NewBorrowedSprite wraps a query result owned by the Playdate display list.
func NewBorrowedSprite(handle uintptr, driver SpriteDriver) *Sprite {
	return &Sprite{handle: handle, driver: driver}
}

// SpriteHandle validates and extracts a runtime sprite for platform queries.
func SpriteHandle(sprite playdate.Sprite) (uintptr, error) {
	value, ok := sprite.(*Sprite)
	if !ok {
		return 0, playdate.ErrSpriteClosed
	}
	return value.nativeHandle()
}

func (s *Sprite) nativeHandle() (uintptr, error) {
	if s == nil || s.closed || s.handle == 0 {
		return 0, playdate.ErrSpriteClosed
	}
	return s.handle, nil
}

func (s *Sprite) SetBitmap(bitmap playdate.Bitmap) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	bitmapHandle, err := BitmapHandle(bitmap)
	if err != nil {
		return err
	}
	s.driver.SetBitmap(handle, bitmapHandle)
	return nil
}
func (s *Sprite) SetCenter(x, y float32) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetCenter(h, x, y)
	}
	return e
}
func (s *Sprite) Center() (float32, float32, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, 0, e
	}
	x, y := s.driver.Center(h)
	return x, y, nil
}
func (s *Sprite) SetBounds(r playdate.Rect) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetBounds(h, r)
	}
	return e
}
func (s *Sprite) Bounds() (playdate.Rect, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return playdate.Rect{}, e
	}
	return s.driver.Bounds(h), nil
}
func (s *Sprite) SetPosition(x, y float32) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.MoveTo(handle, x, y)
	return nil
}
func (s *Sprite) Position() (float32, float32, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, 0, e
	}
	x, y := s.driver.Position(h)
	return x, y, nil
}
func (s *Sprite) MoveBy(dx, dy float32) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.MoveBy(handle, dx, dy)
	return nil
}
func (s *Sprite) SetVisible(visible bool) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetVisible(handle, visible)
	return nil
}
func (s *Sprite) Visible() (bool, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return false, e
	}
	return s.driver.Visible(h), nil
}
func (s *Sprite) SetZIndex(z int) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetZIndex(handle, z)
	return nil
}
func (s *Sprite) ZIndex() (int, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, e
	}
	return s.driver.ZIndex(h), nil
}
func (s *Sprite) SetImageFlip(v playdate.BitmapFlip) error {
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	if e = ValidateBitmapFlip(v); e != nil {
		return e
	}
	s.driver.SetImageFlip(h, v)
	return nil
}
func (s *Sprite) ImageFlip() (playdate.BitmapFlip, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, e
	}
	return s.driver.ImageFlip(h), nil
}
func (s *Sprite) SetDrawMode(v playdate.DrawMode) error {
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	if e = ValidateDrawMode(v); e != nil {
		return e
	}
	s.driver.SetDrawMode(h, v)
	return nil
}
func (s *Sprite) SetOpaque(v bool) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetOpaque(h, v)
	}
	return e
}
func (s *Sprite) SetStencilImage(bitmap playdate.Bitmap, tiled bool) error {
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	b, e := BitmapHandle(bitmap)
	if e != nil {
		return e
	}
	s.driver.SetStencilImage(h, b, tiled)
	return nil
}
func (s *Sprite) SetStencilPattern(pattern [8]byte) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetStencilPattern(h, pattern)
	}
	return e
}
func (s *Sprite) ClearStencil() error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.ClearStencil(h)
	}
	return e
}
func (s *Sprite) SetClipRect(x, y, w, h int) error {
	handle, e := s.nativeHandle()
	if e != nil {
		return e
	}
	if w <= 0 || h <= 0 {
		return playdate.ErrSpriteDirtyRect
	}
	s.driver.SetClipRect(handle, x, y, w, h)
	return nil
}
func (s *Sprite) ClearClipRect() error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.ClearClipRect(h)
	}
	return e
}
func (s *Sprite) SetIgnoresDrawOffset(v bool) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetIgnoresDrawOffset(h, v)
	}
	return e
}
func (s *Sprite) SetTileMap(tileMap playdate.SpriteTileMap) error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	value, err := spriteTileMapValue(tileMap)
	if err != nil {
		return err
	}
	if s.tileMap == value {
		return nil
	}
	if s.tileMap != nil {
		s.tileMap.attachments--
	}
	s.driver.SetTileMap(h, value.handle)
	s.tileMap = value
	value.attachments++
	return nil
}
func (s *Sprite) ClearTileMap() error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if s.tileMap != nil {
		s.tileMap.attachments--
		s.tileMap = nil
	}
	s.driver.SetTileMap(h, 0)
	return nil
}
func (s *Sprite) TileMap() (playdate.SpriteTileMap, bool, error) {
	h, err := s.nativeHandle()
	if err != nil {
		return nil, false, err
	}
	native := s.driver.TileMap(h)
	if native == 0 {
		return nil, false, nil
	}
	if s.tileMap != nil && s.tileMap.handle == native {
		return s.tileMap, true, nil
	}
	return NewBorrowedSpriteTileMap(native, s.driver.tileMapDriver()), true, nil
}
func (s *Sprite) SetUpdatesEnabled(v bool) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetUpdatesEnabled(h, v)
	}
	return e
}
func (s *Sprite) UpdatesEnabled() (bool, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return false, e
	}
	return s.driver.UpdatesEnabled(h), nil
}
func (s *Sprite) SetCollisionsEnabled(v bool) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetCollisionsEnabled(h, v)
	}
	return e
}
func (s *Sprite) CollisionsEnabled() (bool, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return false, e
	}
	return s.driver.CollisionsEnabled(h), nil
}
func (s *Sprite) SetCollideRect(rect playdate.Rect) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetCollideRect(handle, rect)
	return nil
}
func (s *Sprite) CollideRect() (playdate.Rect, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return playdate.Rect{}, e
	}
	return s.driver.CollideRect(h), nil
}
func (s *Sprite) ClearCollideRect() error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.ClearCollideRect(handle)
	return nil
}
func (s *Sprite) SetTag(tag uint8) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetTag(handle, tag)
	return nil
}
func (s *Sprite) Tag() (uint8, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, e
	}
	return s.driver.Tag(h), nil
}
func (s *Sprite) MarkDirty() error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.MarkDirty(handle)
	return nil
}
func (s *Sprite) MarkDirtyRect(rect playdate.Rect) error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if err := ValidateSpriteRect(rect); err != nil {
		return err
	}
	s.driver.MarkDirtyRect(handle, rect)
	return nil
}
func (s *Sprite) MoveWithCollisions(x, y float32) (playdate.MoveResult, error) {
	handle, err := s.nativeHandle()
	if err != nil {
		return playdate.MoveResult{}, err
	}
	actualX, actualY, native := s.driver.MoveWithCollisions(handle, x, y)
	return collisionResult(actualX, actualY, native, s.driver), nil
}
func (s *Sprite) CheckCollisions(x, y float32) (playdate.MoveResult, error) {
	handle, err := s.nativeHandle()
	if err != nil {
		return playdate.MoveResult{}, err
	}
	actualX, actualY, native := s.driver.CheckCollisions(handle, x, y)
	return collisionResult(actualX, actualY, native, s.driver), nil
}
func collisionResult(actualX, actualY float32, native []NativeCollision, driver SpriteDriver) playdate.MoveResult {
	result := playdate.MoveResult{ActualX: actualX, ActualY: actualY, Collisions: make([]playdate.Collision, len(native))}
	for index, collision := range native {
		result.Collisions[index] = playdate.Collision{Other: NewBorrowedSprite(collision.Other, driver), ResponseType: collision.ResponseType, Overlaps: collision.Overlaps, Time: collision.Time, Move: collision.Move, Normal: collision.Normal, Touch: collision.Touch, SpriteRect: collision.SpriteRect, OtherRect: collision.OtherRect}
	}
	return result
}
func (s *Sprite) Add() error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if !s.added {
		s.driver.Add(handle)
		s.added = true
	}
	return nil
}
func (s *Sprite) Remove() error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if s.added {
		s.driver.Remove(handle)
		s.added = false
	}
	return nil
}
func (s *Sprite) Close() error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if !s.owned {
		return playdate.ErrSpriteBorrowed
	}
	if s.added {
		s.driver.Remove(handle)
		s.added = false
	}
	if s.tileMap != nil {
		s.tileMap.attachments--
		s.tileMap = nil
	}
	if s.drawCallback != nil {
		_ = s.setCallback(true, false)
	}
	if s.updateCallback != nil {
		_ = s.setCallback(true, false)
	}
	if s.collisionCallback != nil {
		_ = s.setCallback(true, false)
	}
	s.driver.Free(handle)
	if s.driver.DisplayList != nil {
		delete(s.driver.DisplayList.sprites, s)
		delete(s.driver.DisplayList.byHandle, handle)
	}
	s.closed = true
	s.handle = 0
	return nil
}

// SpriteTileMapDriver contains platform operations for one native LCDTileMap.
type SpriteTileMapDriver struct {
	SetImageTable func(uintptr, uintptr)
	SetSize       func(uintptr, int, int)
	Size          func(uintptr) (int, int)
	PixelSize     func(uintptr) (int, int)
	SetTiles      func(uintptr, []uint16, int)
	SetTile       func(uintptr, int, int, uint16)
	Tile          func(uintptr, int, int) uint16
	Free          func(uintptr)
}

// SpriteTileMap owns a native LCDTileMap and retains its bitmap table.
type SpriteTileMap struct {
	handle        uintptr
	driver        SpriteTileMapDriver
	table         *BitmapTable
	columns, rows int
	attachments   int
	owned, closed bool
}

func NewOwnedSpriteTileMap(handle uintptr, table playdate.BitmapTable, columns, rows int, tiles []uint16, driver SpriteTileMapDriver) (*SpriteTileMap, error) {
	if handle == 0 {
		return nil, playdate.ErrSpriteTileMapCreate
	}
	t, ok := table.(*BitmapTable)
	if !ok {
		driver.Free(handle)
		return nil, ErrInvalidBitmap
	}
	tableHandle, err := OwnedBitmapTableHandle(table)
	if err != nil {
		driver.Free(handle)
		return nil, err
	}
	if columns <= 0 || rows <= 0 || len(tiles) != columns*rows {
		driver.Free(handle)
		return nil, playdate.ErrSpriteTileMapConfig
	}
	value := &SpriteTileMap{handle: handle, driver: driver, table: t, columns: columns, rows: rows, owned: true}
	driver.SetImageTable(handle, tableHandle)
	driver.SetSize(handle, columns, rows)
	driver.SetTiles(handle, tiles, columns)
	t.tileMapUsers++
	return value, nil
}
func NewBorrowedSpriteTileMap(handle uintptr, driver SpriteTileMapDriver) *SpriteTileMap {
	return &SpriteTileMap{handle: handle, driver: driver}
}
func spriteTileMapValue(tileMap playdate.SpriteTileMap) (*SpriteTileMap, error) {
	value, ok := tileMap.(*SpriteTileMap)
	if !ok || value == nil || value.closed || value.handle == 0 {
		return nil, playdate.ErrSpriteTileMapClosed
	}
	return value, nil
}
func (m *SpriteTileMap) Size() (int, int, error) {
	v, e := spriteTileMapValue(m)
	if e != nil {
		return 0, 0, e
	}
	x, y := v.driver.Size(v.handle)
	return x, y, nil
}
func (m *SpriteTileMap) PixelSize() (int, int, error) {
	v, e := spriteTileMapValue(m)
	if e != nil {
		return 0, 0, e
	}
	x, y := v.driver.PixelSize(v.handle)
	return x, y, nil
}
func (m *SpriteTileMap) Tile(column, row int) (uint16, error) {
	v, e := spriteTileMapValue(m)
	if e != nil {
		return 0, e
	}
	if column < 0 || row < 0 || column >= v.columns || row >= v.rows {
		return 0, playdate.ErrSpriteTileMapBounds
	}
	return v.driver.Tile(v.handle, column, row), nil
}
func (m *SpriteTileMap) SetTile(column, row int, index uint16) error {
	v, e := spriteTileMapValue(m)
	if e != nil {
		return e
	}
	if column < 0 || row < 0 || column >= v.columns || row >= v.rows {
		return playdate.ErrSpriteTileMapBounds
	}
	v.driver.SetTile(v.handle, column, row, index)
	return nil
}
func (m *SpriteTileMap) Close() error {
	v, e := spriteTileMapValue(m)
	if e != nil {
		return e
	}
	if !v.owned {
		return playdate.ErrSpriteTileMapBorrowed
	}
	if v.attachments != 0 {
		return playdate.ErrSpriteTileMapInUse
	}
	v.driver.Free(v.handle)
	if v.table != nil {
		v.table.tileMapUsers--
		v.table = nil
	}
	v.closed = true
	v.handle = 0
	return nil
}

func (d SpriteDriver) tileMapDriver() SpriteTileMapDriver { return d.TileMaps }

// BorrowedSprites converts native query handles without transferring ownership.
func BorrowedSprites(handles []uintptr, driver SpriteDriver) []playdate.Sprite {
	result := make([]playdate.Sprite, len(handles))
	for index, handle := range handles {
		result[index] = NewBorrowedSprite(handle, driver)
	}
	return result
}

// SpriteHandles validates a complete batch before a native display-list operation.
func SpriteHandles(sprites []playdate.Sprite) ([]uintptr, error) {
	handles := make([]uintptr, len(sprites))
	for index, sprite := range sprites {
		handle, err := SpriteHandle(sprite)
		if err != nil {
			return nil, err
		}
		handles[index] = handle
	}
	return handles, nil
}

// SetSpritesAdded atomically validates a batch, performs its native operation,
// and synchronizes the owned wrappers' display-list state.
func SetSpritesAdded(sprites []playdate.Sprite, added bool, operation func([]uintptr)) error {
	handles, err := SpriteHandles(sprites)
	if err != nil {
		return err
	}
	effective := make([]uintptr, 0, len(handles))
	for index, sprite := range sprites {
		if sprite.(*Sprite).added != added {
			effective = append(effective, handles[index])
		}
	}
	operation(effective)
	for _, sprite := range sprites {
		sprite.(*Sprite).added = added
	}
	return nil
}

// ValidateSpriteRect rejects geometry that cannot describe a finite dirty area.
func ValidateSpriteRect(rect playdate.Rect) error {
	values := [...]float32{rect.X, rect.Y, rect.Width, rect.Height}
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return playdate.ErrSpriteDirtyRect
		}
	}
	if rect.Width <= 0 || rect.Height <= 0 {
		return playdate.ErrSpriteDirtyRect
	}
	return nil
}

// ValidateScreenDirtyRect rejects empty or overflowing native LCD rectangles.
func ValidateScreenDirtyRect(x, y, width, height int) error {
	const minInt32, maxInt32 = -1 << 31, 1<<31 - 1
	if width <= 0 || height <= 0 || x < minInt32 || x > maxInt32 || y < minInt32 || y > maxInt32 || width > maxInt32 || height > maxInt32 || x > maxInt32-width || y > maxInt32-height {
		return playdate.ErrSpriteDirtyRect
	}
	return nil
}

// ValidateBitmapSize applies the common Simulator/device creation contract.
func ValidateBitmapSize(width, height int) error {
	if width <= 0 || height <= 0 || width > 2147483647 || height > 2147483647 {
		return playdate.ErrBitmapSize
	}
	return nil
}

// ValidateBitmapScale applies the common Simulator/device drawing contract.
func ValidateBitmapScale(scaleX, scaleY float32) error {
	const maxFloat32 = 3.4028235e38
	if scaleX <= 0 || scaleY <= 0 || scaleX != scaleX || scaleY != scaleY || scaleX > maxFloat32 || scaleY > maxFloat32 {
		return playdate.ErrBitmapScale
	}
	return nil
}

// ValidateBitmapTransform applies the shared rotated-bitmap contract.
func ValidateBitmapTransform(degrees, centerX, centerY, scaleX, scaleY float32) error {
	if err := ValidateBitmapScale(scaleX, scaleY); err != nil {
		return err
	}
	const maxFloat32 = 3.4028235e38
	if degrees != degrees || degrees > maxFloat32 || degrees < -maxFloat32 ||
		centerX != centerX || centerX > maxFloat32 || centerX < -maxFloat32 ||
		centerY != centerY || centerY > maxFloat32 || centerY < -maxFloat32 {
		return playdate.ErrGraphicsGeometry
	}
	return nil
}

// ValidateBitmapFlip rejects values outside the official flip enumeration.
func ValidateBitmapFlip(flip playdate.BitmapFlip) error {
	if flip > playdate.BitmapFlippedXY {
		return playdate.ErrBitmapFlip
	}
	return nil
}

// ValidateBitmapTableSize applies the shared table creation contract.
func ValidateBitmapTableSize(count, width, height int) error {
	if count <= 0 || width <= 0 || height <= 0 {
		return playdate.ErrBitmapTableSize
	}
	return nil
}

// ValidateStencil checks the shared tiled-stencil constraint and extracts its handle.
func ValidateStencil(stencil playdate.Bitmap, tiled bool) (uintptr, error) {
	handle, err := BitmapHandle(stencil)
	if err != nil {
		return 0, err
	}
	if !tiled {
		return handle, nil
	}
	width, err := stencil.Width()
	if err != nil {
		return 0, err
	}
	if width%32 != 0 {
		return 0, playdate.ErrGraphicsStencilWidth
	}
	return handle, nil
}

// RawInput contains one platform input sample. Both platform adapters provide
// this same representation before the runtime derives frame transitions.
type RawInput struct {
	Buttons      playdate.Buttons
	CrankAngle   float32
	CrankDelta   float32
	CrankDocked  bool
	DeltaSeconds float32
}

// Callbacks contains application behavior invoked by Runtime.
type Callbacks struct {
	Init      func() error
	Lifecycle func(playdate.LifecycleEvent) error
	Update    func(playdate.Input) (refresh bool, err error)
}

// Runtime enforces the platform-independent application lifecycle.
type Runtime struct {
	callbacks   Callbacks
	initialized bool
	failed      bool
	terminated  bool
	input       playdate.Input
	hasInput    bool
}

// Application is the platform-independent entry point shared by Simulator and
// device ABI adapters.
type Application struct {
	runtime *Runtime
}

type applicationContext struct {
	playdate.Context
	input                playdate.Input
	menuItems            []playdate.MenuItem
	microphoneRecorder   playdate.MicrophoneRecorder
	accelerometerEnabled bool
	autoLockDisabled     bool
	crankSoundsTracked   bool
	crankSoundsInitial   bool
	systemTerminating    bool
	stencilActive        bool
	terminated           bool
}

func (context *applicationContext) QuerySpritesAlongLine(x1, y1, x2, y2 float32) []playdate.Sprite {
	if queries, ok := context.Context.(playdate.SpriteQueries); ok && !context.terminated {
		return queries.QuerySpritesAlongLine(x1, y1, x2, y2)
	}
	return nil
}
func (context *applicationContext) QuerySpriteInfoAlongLine(x1, y1, x2, y2 float32) []playdate.SpriteQueryInfo {
	if queries, ok := context.Context.(playdate.SpriteQueries); ok && !context.terminated {
		return queries.QuerySpriteInfoAlongLine(x1, y1, x2, y2)
	}
	return nil
}
func (context *applicationContext) spriteDisplayList() (playdate.SpriteDisplayList, error) {
	controls, ok := context.Context.(playdate.SpriteDisplayList)
	if !ok || context.terminated {
		return nil, playdate.ErrSpriteDisplayListUnavailable
	}
	return controls, nil
}
func (context *applicationContext) SpriteCount() int {
	controls, err := context.spriteDisplayList()
	if err != nil {
		return 0
	}
	return controls.SpriteCount()
}
func (context *applicationContext) AddSprites(sprites []playdate.Sprite) error {
	controls, err := context.spriteDisplayList()
	if err != nil {
		return err
	}
	return controls.AddSprites(sprites)
}
func (context *applicationContext) RemoveSprites(sprites []playdate.Sprite) error {
	controls, err := context.spriteDisplayList()
	if err != nil {
		return err
	}
	return controls.RemoveSprites(sprites)
}
func (context *applicationContext) RemoveAllSprites() {
	if controls, err := context.spriteDisplayList(); err == nil {
		controls.RemoveAllSprites()
	}
}
func (context *applicationContext) ResetCollisionWorld() {
	if controls, err := context.spriteDisplayList(); err == nil {
		controls.ResetCollisionWorld()
	}
}

func (context *applicationContext) LoadVideo(path string) (playdate.VideoPlayer, error) {
	videos, ok := context.Context.(playdate.Videos)
	if !ok || context.terminated {
		return nil, playdate.ErrVideoUnavailable
	}
	if path == "" {
		return nil, playdate.ErrVideoPath
	}
	return videos.LoadVideo(path)
}

func (context *applicationContext) display() (playdate.Display, error) {
	display, ok := context.Context.(playdate.Display)
	if !ok {
		return nil, playdate.ErrDisplayUnavailable
	}
	return display, nil
}
func (context *applicationContext) SetRefreshRate(rate float32) error {
	display, err := context.display()
	if err != nil {
		return err
	}
	return display.SetRefreshRate(rate)
}
func (context *applicationContext) Width() int {
	display, err := context.display()
	if err != nil {
		return 0
	}
	return display.Width()
}
func (context *applicationContext) Height() int {
	display, err := context.display()
	if err != nil {
		return 0
	}
	return display.Height()
}
func (context *applicationContext) RefreshRate() float32 {
	display, err := context.display()
	if err != nil {
		return 0
	}
	return display.RefreshRate()
}
func (context *applicationContext) FPS() float32 {
	display, err := context.display()
	if err != nil {
		return 0
	}
	return display.FPS()
}
func (context *applicationContext) SetInverted(value bool) {
	if display, err := context.display(); err == nil {
		display.SetInverted(value)
	}
}
func (context *applicationContext) SetScale(scale uint) error {
	display, err := context.display()
	if err != nil {
		return err
	}
	return display.SetScale(scale)
}
func (context *applicationContext) SetMosaic(x, y uint) error {
	display, err := context.display()
	if err != nil {
		return err
	}
	return display.SetMosaic(x, y)
}
func (context *applicationContext) SetFlipped(x, y bool) {
	if display, err := context.display(); err == nil {
		display.SetFlipped(x, y)
	}
}
func (context *applicationContext) SetOffset(x, y int) {
	if display, err := context.display(); err == nil {
		display.SetOffset(x, y)
	}
}
func (context *applicationContext) SetAlwaysRedraw(value bool) {
	if redraw, ok := context.Context.(playdate.SpriteRedraw); ok {
		redraw.SetAlwaysRedraw(value)
	}
}
func (context *applicationContext) AddDirtyRect(x, y, width, height int) error {
	redraw, ok := context.Context.(playdate.SpriteRedraw)
	if !ok {
		return playdate.ErrSpriteRedrawUnavailable
	}
	return redraw.AddDirtyRect(x, y, width, height)
}
func (context *applicationContext) NewSpriteTileMap(table playdate.BitmapTable, columns, rows int, tiles []uint16) (playdate.SpriteTileMap, error) {
	tileMaps, ok := context.Context.(playdate.SpriteTileMaps)
	if !ok || context.terminated {
		return nil, playdate.ErrSpriteTileMapUnavailable
	}
	return tileMaps.NewSpriteTileMap(table, columns, rows, tiles)
}

func (context *applicationContext) Input() playdate.Input { return context.input }

func (context *applicationContext) CurrentAudioTime() (uint32, error) {
	clock, ok := context.Context.(playdate.AudioClock)
	if !ok {
		return 0, playdate.ErrAudioUnavailable
	}
	return clock.CurrentAudioTime()
}

func (context *applicationContext) LoadSamplePlayer(path string) (playdate.SamplePlayer, error) {
	samples, _ := context.Context.(playdate.SamplePlayers)
	if samples == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return samples.LoadSamplePlayer(path)
}

func (context *applicationContext) NewPCMPlayer(samples []int16, sampleRate uint32) (playdate.SamplePlayer, error) {
	players, _ := context.Context.(playdate.PCMPlayers)
	if players == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	if len(samples) == 0 || sampleRate == 0 {
		return nil, playdate.ErrAudioParameter
	}
	return players.NewPCMPlayer(samples, sampleRate)
}

func (context *applicationContext) NewPCMCallbackSource(channel playdate.AudioChannel, stereo bool, callback playdate.PCMRenderCallback) (playdate.PCMCallbackSource, error) {
	callbacks, _ := context.Context.(playdate.CallbackAudio)
	if callbacks == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return callbacks.NewPCMCallbackSource(channel, stereo, callback)
}

func (context *applicationContext) NewSample(byteCount int) (playdate.AudioSample, error) {
	samples, ok := context.Context.(playdate.AudioSamples)
	if !ok || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return samples.NewSample(byteCount)
}
func (context *applicationContext) LoadSample(path string) (playdate.AudioSample, error) {
	samples, ok := context.Context.(playdate.AudioSamples)
	if !ok || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return samples.LoadSample(path)
}
func (context *applicationContext) NewSampleFromData(data []byte, format playdate.SoundFormat, rate uint32) (playdate.AudioSample, error) {
	samples, ok := context.Context.(playdate.AudioSamples)
	if !ok || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return samples.NewSampleFromData(data, format, rate)
}
func (context *applicationContext) NewSamplePlayer(sample playdate.AudioSample) (playdate.SamplePlayer, error) {
	players, ok := context.Context.(playdate.SamplePlayerFactory)
	if !ok || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return players.NewSamplePlayer(sample)
}

func (context *applicationContext) NewAudioChannel() (playdate.AudioChannel, error) {
	channels, _ := context.Context.(playdate.AudioChannels)
	if channels == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return channels.NewAudioChannel()
}

func (context *applicationContext) DefaultAudioChannel() (playdate.AudioChannel, error) {
	outputs, _ := context.Context.(playdate.AudioOutputs)
	if outputs == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return outputs.DefaultAudioChannel()
}

func (context *applicationContext) AudioOutputState() (playdate.AudioOutputState, error) {
	outputs, _ := context.Context.(playdate.AudioOutputs)
	if outputs == nil || context.terminated {
		return playdate.AudioOutputState{}, playdate.ErrAudioUnavailable
	}
	return outputs.AudioOutputState()
}

func (context *applicationContext) SetAudioOutputsActive(headphones, speaker bool) error {
	outputs, _ := context.Context.(playdate.AudioOutputs)
	if outputs == nil || context.terminated {
		return playdate.ErrAudioUnavailable
	}
	return outputs.SetAudioOutputsActive(headphones, speaker)
}

func (context *applicationContext) NewSynth(waveform playdate.Waveform) (playdate.Synth, error) {
	synths, _ := context.Context.(playdate.Synthesizers)
	if synths == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return synths.NewSynth(waveform)
}
func (context *applicationContext) NewGeneratorSynth(stereo bool, callback playdate.GeneratorRenderCallback) (playdate.GeneratorSynth, error) {
	generators, _ := context.Context.(playdate.GeneratorSynthesizers)
	if generators == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return generators.NewGeneratorSynth(stereo, callback)
}
func (context *applicationContext) NewLFO(lfoType playdate.LFOType) (playdate.LFO, error) {
	synths, _ := context.Context.(playdate.Synthesizers)
	if synths == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return synths.NewLFO(lfoType)
}
func (context *applicationContext) NewEnvelope(a, d, s, r float32) (playdate.Envelope, error) {
	synths, _ := context.Context.(playdate.Synthesizers)
	if synths == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return synths.NewEnvelope(a, d, s, r)
}
func (context *applicationContext) NewControlSignal() (playdate.ControlSignal, error) {
	synths, _ := context.Context.(playdate.Synthesizers)
	if synths == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return synths.NewControlSignal()
}

func (context *applicationContext) NewInstrument() (playdate.Instrument, error) {
	v, _ := context.Context.(playdate.Sequencers)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewInstrument()
}
func (context *applicationContext) NewSequenceTrack() (playdate.SequenceTrack, error) {
	v, _ := context.Context.(playdate.Sequencers)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewSequenceTrack()
}
func (context *applicationContext) NewSequence() (playdate.Sequence, error) {
	v, _ := context.Context.(playdate.Sequencers)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewSequence()
}
func (context *applicationContext) NewTwoPoleFilter(kind playdate.FilterType) (playdate.TwoPoleFilter, error) {
	v, _ := context.Context.(playdate.AudioEffects)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewTwoPoleFilter(kind)
}
func (context *applicationContext) NewOnePoleFilter() (playdate.OnePoleFilter, error) {
	v, _ := context.Context.(playdate.AudioEffects)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewOnePoleFilter()
}
func (context *applicationContext) NewBitCrusher() (playdate.BitCrusher, error) {
	v, _ := context.Context.(playdate.AudioEffects)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewBitCrusher()
}
func (context *applicationContext) NewRingModulator() (playdate.RingModulator, error) {
	v, _ := context.Context.(playdate.AudioEffects)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewRingModulator()
}
func (context *applicationContext) NewDelayLine(length int, stereo bool) (playdate.DelayLine, error) {
	v, _ := context.Context.(playdate.AudioEffects)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewDelayLine(length, stereo)
}
func (context *applicationContext) NewOverdrive() (playdate.Overdrive, error) {
	v, _ := context.Context.(playdate.AudioEffects)
	if v == nil || context.terminated {
		return nil, playdate.ErrAudioUnavailable
	}
	return v.NewOverdrive()
}

func (context *applicationContext) RequestMicrophoneAccess(purpose string, callback func(playdate.MicrophonePermission)) (playdate.MicrophonePermission, error) {
	if callback == nil {
		return playdate.MicrophonePermissionPending, playdate.ErrMicrophoneCallback
	}
	microphones, _ := context.Context.(playdate.Microphones)
	if microphones == nil || context.terminated {
		return playdate.MicrophonePermissionPending, playdate.ErrMicrophoneUnavailable
	}
	return microphones.RequestMicrophoneAccess(purpose, func(permission playdate.MicrophonePermission) {
		if !context.terminated {
			callback(permission)
		}
	})
}

func (context *applicationContext) StartMicrophoneRecording(source playdate.MicrophoneSource, callback func(playdate.MicrophoneSamples) bool) (playdate.MicrophoneRecorder, error) {
	if callback == nil {
		return nil, playdate.ErrMicrophoneCallback
	}
	microphones, _ := context.Context.(playdate.Microphones)
	if microphones == nil || context.terminated {
		return nil, playdate.ErrMicrophoneUnavailable
	}
	if context.microphoneRecorder != nil {
		_ = context.microphoneRecorder.Close()
		context.microphoneRecorder = nil
	}
	recorder, err := microphones.StartMicrophoneRecording(source, func(samples playdate.MicrophoneSamples) bool {
		return !context.terminated && callback(samples)
	})
	if err != nil {
		return nil, err
	}
	context.microphoneRecorder = recorder
	return recorder, nil
}

func (context *applicationContext) PollDebugMessage() (string, bool) {
	messages, _ := context.Context.(playdate.DebugMessages)
	if messages == nil || context.terminated {
		return "", false
	}
	return messages.PollDebugMessage()
}

func (context *applicationContext) AddScore(boardID string, value uint32, callback func(playdate.Score, error)) error {
	if callback == nil {
		return playdate.ErrScoreboardCallback
	}
	service, _ := context.Context.(playdate.Scoreboards)
	if service == nil {
		return playdate.ErrScoreboardUnavailable
	}
	return service.AddScore(boardID, value, func(score playdate.Score, err error) {
		if !context.terminated {
			callback(score, err)
		}
	})
}
func (context *applicationContext) GetPersonalBest(boardID string, callback func(playdate.Score, error)) error {
	if callback == nil {
		return playdate.ErrScoreboardCallback
	}
	service, _ := context.Context.(playdate.Scoreboards)
	if service == nil {
		return playdate.ErrScoreboardUnavailable
	}
	return service.GetPersonalBest(boardID, func(score playdate.Score, err error) {
		if !context.terminated {
			callback(score, err)
		}
	})
}
func (context *applicationContext) GetScoreboards(callback func(playdate.BoardsList, error)) error {
	if callback == nil {
		return playdate.ErrScoreboardCallback
	}
	service, _ := context.Context.(playdate.Scoreboards)
	if service == nil {
		return playdate.ErrScoreboardUnavailable
	}
	return service.GetScoreboards(func(list playdate.BoardsList, err error) {
		if !context.terminated {
			callback(list, err)
		}
	})
}
func (context *applicationContext) GetScores(boardID string, callback func(playdate.ScoresList, error)) error {
	if callback == nil {
		return playdate.ErrScoreboardCallback
	}
	service, _ := context.Context.(playdate.Scoreboards)
	if service == nil {
		return playdate.ErrScoreboardUnavailable
	}
	return service.GetScores(boardID, func(list playdate.ScoresList, err error) {
		if !context.terminated {
			callback(list, err)
		}
	})
}

func (context *applicationContext) ExitToLauncher() {
	launcher, ok := context.Context.(playdate.Launcher)
	if ok {
		launcher.ExitToLauncher()
	}
}

// ValidateLaunchArguments rejects strings that cannot cross the native C API.
func ValidateLaunchArguments(arguments string) error {
	if strings.IndexByte(arguments, 0) >= 0 {
		return playdate.ErrLaunchArguments
	}
	return nil
}

// ValidateButtonCallbackConfig applies the bounded native queue contract.
func ValidateButtonCallbackConfig(callback playdate.ButtonCallback, queueSize int) error {
	if callback == nil && queueSize == 0 {
		return nil
	}
	if callback == nil || queueSize < 1 || queueSize > playdate.ButtonCallbackQueueLimit {
		return playdate.ErrButtonCallbackConfig
	}
	return nil
}

// ValidateDateTime applies the representable Playdate epoch and calendar
// contract. Weekday is output metadata and is intentionally ignored.
func ValidateDateTime(value playdate.DateTime) error {
	if value.Year < 2000 || value.Month < 1 || value.Month > 12 || value.Day < 1 ||
		value.Hour > 23 || value.Minute > 59 || value.Second > 59 {
		return playdate.ErrDateTime
	}
	days := [...]uint8{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if value.Year%4 == 0 && (value.Year%100 != 0 || value.Year%400 == 0) {
		days[2] = 29
	}
	if value.Day > days[value.Month] {
		return playdate.ErrDateTime
	}
	if value.Year > 2136 || value.Year == 2136 && (value.Month > 2 || value.Month == 2 && (value.Day > 7 || value.Day == 7 && (value.Hour > 6 || value.Hour == 6 && (value.Minute > 28 || value.Minute == 28 && value.Second > 15)))) {
		return playdate.ErrDateTime
	}
	return nil
}

func (context *applicationContext) systemControls() (playdate.SystemControls, error) {
	controls, ok := context.Context.(playdate.SystemControls)
	if !ok || context.systemTerminating || context.terminated {
		return nil, playdate.ErrSystemControlsUnavailable
	}
	return controls, nil
}

func (context *applicationContext) LaunchArguments() (arguments, path string) {
	controls, err := context.systemControls()
	if err != nil {
		return "", ""
	}
	return controls.LaunchArguments()
}

func (context *applicationContext) RestartGame(arguments string) error {
	if err := ValidateLaunchArguments(arguments); err != nil {
		return err
	}
	controls, err := context.systemControls()
	if err != nil {
		return err
	}
	return controls.RestartGame(arguments)
}

func (context *applicationContext) SetMenuImage(bitmap playdate.Bitmap, xOffset int) error {
	controls, err := context.systemControls()
	if err != nil {
		return err
	}
	return controls.SetMenuImage(bitmap, xOffset)
}

func (context *applicationContext) ClearMenuImage() {
	if controls, err := context.systemControls(); err == nil {
		controls.ClearMenuImage()
	}
}

func (context *applicationContext) SetAutoLockDisabled(disabled bool) {
	controls, err := context.systemControls()
	if err != nil {
		return
	}
	controls.SetAutoLockDisabled(disabled)
	context.autoLockDisabled = disabled
}

func (context *applicationContext) SetCrankSoundsDisabled(disabled bool) (previous bool) {
	controls, err := context.systemControls()
	if err != nil {
		return false
	}
	previous = controls.SetCrankSoundsDisabled(disabled)
	if !context.crankSoundsTracked {
		context.crankSoundsTracked = true
		context.crankSoundsInitial = previous
	}
	return previous
}

func (context *applicationContext) SetButtonCallback(callback playdate.ButtonCallback, queueSize int) error {
	if err := ValidateButtonCallbackConfig(callback, queueSize); err != nil {
		return err
	}
	controls, err := context.systemControls()
	if err != nil {
		return err
	}
	return controls.SetButtonCallback(callback, queueSize)
}

func (context *applicationContext) ButtonCallbackOverflow() uint32 {
	controls, err := context.systemControls()
	if err != nil {
		return 0
	}
	return controls.ButtonCallbackOverflow()
}

func (context *applicationContext) systemEnvironment() playdate.SystemEnvironment {
	environment, _ := context.Context.(playdate.SystemEnvironment)
	return environment
}

func (context *applicationContext) CurrentEpochTime() playdate.EpochTime {
	if environment := context.systemEnvironment(); environment != nil {
		return environment.CurrentEpochTime()
	}
	return playdate.EpochTime{}
}

func (context *applicationContext) EpochToDateTime(epoch uint32) playdate.DateTime {
	if environment := context.systemEnvironment(); environment != nil {
		return environment.EpochToDateTime(epoch)
	}
	return playdate.DateTime{}
}

func (context *applicationContext) DateTimeToEpoch(value playdate.DateTime) (uint32, error) {
	if err := ValidateDateTime(value); err != nil {
		return 0, err
	}
	if environment := context.systemEnvironment(); environment != nil {
		return environment.DateTimeToEpoch(value)
	}
	return 0, playdate.ErrSystemEnvironmentUnavailable
}

func (context *applicationContext) ResetElapsedTime() {
	if environment := context.systemEnvironment(); environment != nil {
		environment.ResetElapsedTime()
	}
}

func (context *applicationContext) ElapsedTime() float32 {
	if environment := context.systemEnvironment(); environment != nil {
		return environment.ElapsedTime()
	}
	return 0
}

func (context *applicationContext) SystemInfo() playdate.SystemInfo {
	if environment := context.systemEnvironment(); environment != nil {
		return environment.SystemInfo()
	}
	return playdate.SystemInfo{}
}

func (context *applicationContext) beginSystemTermination() {
	controls, ok := context.Context.(playdate.SystemControls)
	context.systemTerminating = true
	if !ok {
		return
	}
	_ = controls.SetButtonCallback(nil, 0)
	controls.ClearMenuImage()
	if context.autoLockDisabled {
		controls.SetAutoLockDisabled(false)
		context.autoLockDisabled = false
	}
	if context.crankSoundsTracked {
		controls.SetCrankSoundsDisabled(context.crankSoundsInitial)
		context.crankSoundsTracked = false
	}
}

func (context *applicationContext) SetAccelerometerEnabled(enabled bool) {
	accelerometer, ok := context.Context.(playdate.Accelerometer)
	if !ok {
		return
	}
	accelerometer.SetAccelerometerEnabled(enabled)
	context.accelerometerEnabled = enabled
}

func (context *applicationContext) AccelerometerXYZ() (x, y, z float32) {
	accelerometer, ok := context.Context.(playdate.Accelerometer)
	if !ok || !context.accelerometerEnabled {
		return 0, 0, 0
	}
	return accelerometer.AccelerometerXYZ()
}

func (context *applicationContext) PowerStatus() playdate.PowerStatus {
	monitor, _ := context.Context.(playdate.PowerMonitor)
	if monitor == nil {
		return 0
	}
	return monitor.PowerStatus()
}
func (context *applicationContext) BatteryPercentage() float32 {
	monitor, _ := context.Context.(playdate.PowerMonitor)
	if monitor == nil {
		return 0
	}
	return monitor.BatteryPercentage()
}
func (context *applicationContext) BatteryVoltage() float32 {
	monitor, _ := context.Context.(playdate.PowerMonitor)
	if monitor == nil {
		return 0
	}
	return monitor.BatteryVoltage()
}
func (context *applicationContext) SystemVolume() float32 {
	settings, _ := context.Context.(playdate.SystemPreferences)
	if settings == nil {
		return 0
	}
	return settings.SystemVolume()
}
func (context *applicationContext) ReduceFlashing() bool {
	settings, _ := context.Context.(playdate.SystemPreferences)
	return settings != nil && settings.ReduceFlashing()
}
func (context *applicationContext) TimezoneOffsetSeconds() int32 {
	settings, _ := context.Context.(playdate.SystemPreferences)
	if settings == nil {
		return 0
	}
	return settings.TimezoneOffsetSeconds()
}
func (context *applicationContext) Uses24HourTime() bool {
	settings, _ := context.Context.(playdate.SystemPreferences)
	return settings != nil && settings.Uses24HourTime()
}

func (context *applicationContext) disableAccelerometer() {
	if context.accelerometerEnabled {
		context.SetAccelerometerEnabled(false)
	}
}

func (context *applicationContext) systemMenu() (playdate.SystemMenu, error) {
	menu, ok := context.Context.(playdate.SystemMenu)
	if !ok {
		return nil, playdate.ErrMenuItemCreate
	}
	return menu, nil
}

func (context *applicationContext) AddActionMenuItem(title string, callback func()) (playdate.MenuItem, error) {
	menu, err := context.systemMenu()
	if err != nil {
		return nil, err
	}
	item, err := menu.AddActionMenuItem(title, callback)
	if err == nil {
		context.menuItems = append(context.menuItems, item)
	}
	return item, err
}

func (context *applicationContext) AddCheckmarkMenuItem(title string, value bool, callback func()) (playdate.CheckmarkMenuItem, error) {
	menu, err := context.systemMenu()
	if err != nil {
		return nil, err
	}
	item, err := menu.AddCheckmarkMenuItem(title, value, callback)
	if err == nil {
		context.menuItems = append(context.menuItems, item)
	}
	return item, err
}

func (context *applicationContext) AddOptionsMenuItem(title string, options []string, callback func()) (playdate.OptionsMenuItem, error) {
	menu, err := context.systemMenu()
	if err != nil {
		return nil, err
	}
	item, err := menu.AddOptionsMenuItem(title, options, callback)
	if err == nil {
		context.menuItems = append(context.menuItems, item)
	}
	return item, err
}

func (context *applicationContext) Language() playdate.Language {
	localization, ok := context.Context.(playdate.Localization)
	if !ok {
		return playdate.LanguageEnglish
	}
	return localization.Language()
}

func (context *applicationContext) LocalizedText(key string, language playdate.Language) (string, bool) {
	localization, ok := context.Context.(playdate.Localization)
	if !ok {
		return "", false
	}
	return localization.LocalizedText(key, language)
}

func (context *applicationContext) removeMenuItems() {
	for _, item := range context.menuItems {
		item.Remove()
	}
	context.menuItems = nil
}

func (context *applicationContext) fileSystem() (playdate.FileSystem, error) {
	files, ok := context.Context.(playdate.FileSystem)
	if !ok {
		return nil, playdate.ErrFileUnavailable
	}
	return files, nil
}

func (context *applicationContext) OpenFile(filePath string, options playdate.FileOptions) (playdate.File, error) {
	files, err := context.fileSystem()
	if err != nil {
		return nil, err
	}
	return files.OpenFile(filePath, options)
}

func (context *applicationContext) Stat(filePath string) (playdate.FileInfo, error) {
	files, err := context.fileSystem()
	if err != nil {
		return playdate.FileInfo{}, err
	}
	return files.Stat(filePath)
}

func (context *applicationContext) List(filePath string, showHidden bool) ([]string, error) {
	files, err := context.fileSystem()
	if err != nil {
		return nil, err
	}
	return files.List(filePath, showHidden)
}

func (context *applicationContext) Mkdir(filePath string) error {
	files, err := context.fileSystem()
	if err != nil {
		return err
	}
	return files.Mkdir(filePath)
}

func (context *applicationContext) Remove(filePath string, recursive bool) error {
	files, err := context.fileSystem()
	if err != nil {
		return err
	}
	return files.Remove(filePath, recursive)
}

func (context *applicationContext) Rename(from, to string) error {
	files, err := context.fileSystem()
	if err != nil {
		return err
	}
	return files.Rename(from, to)
}

func (context *applicationContext) LoadFont(path string) (playdate.Font, error) {
	graphics, ok := context.Context.(playdate.FontGraphics)
	if !ok {
		return nil, playdate.ErrFontLoad
	}
	return graphics.LoadFont(path)
}

func (context *applicationContext) DrawTextFont(font playdate.Font, text string, x, y int) error {
	graphics, ok := context.Context.(playdate.FontGraphics)
	if !ok {
		return playdate.ErrFontLoad
	}
	return graphics.DrawTextFont(font, text, x, y)
}

func (context *applicationContext) textGraphics() (playdate.TextGraphics, error) {
	graphics, ok := context.Context.(playdate.TextGraphics)
	if !ok {
		return nil, playdate.ErrGraphicsUnavailable
	}
	return graphics, nil
}
func (context *applicationContext) SetTextTracking(value int) {
	if graphics, err := context.textGraphics(); err == nil {
		graphics.SetTextTracking(value)
	}
}
func (context *applicationContext) TextTracking() int {
	if graphics, err := context.textGraphics(); err == nil {
		return graphics.TextTracking()
	}
	return 0
}
func (context *applicationContext) SetTextLeading(value int) {
	if graphics, err := context.textGraphics(); err == nil {
		graphics.SetTextLeading(value)
	}
}
func (context *applicationContext) DrawTextInRect(text string, x, y, width, height int, wrapping playdate.TextWrappingMode, alignment playdate.TextAlignment) error {
	graphics, err := context.textGraphics()
	if err != nil {
		return err
	}
	return graphics.DrawTextInRect(text, x, y, width, height, wrapping, alignment)
}
func (context *applicationContext) TextHeight(font playdate.Font, text string, maxWidth int, wrapping playdate.TextWrappingMode, tracking, leading int) (int, error) {
	graphics, err := context.textGraphics()
	if err != nil {
		return 0, err
	}
	return graphics.TextHeight(font, text, maxWidth, wrapping, tracking, leading)
}
func (context *applicationContext) Glyph(font playdate.Font, codepoint, next rune) (playdate.FontGlyph, error) {
	graphics, err := context.textGraphics()
	if err != nil {
		return playdate.FontGlyph{}, err
	}
	return graphics.Glyph(font, codepoint, next)
}

func (context *applicationContext) primitiveGraphics() (playdate.PrimitiveGraphics, error) {
	graphics, ok := context.Context.(playdate.PrimitiveGraphics)
	if !ok {
		return nil, playdate.ErrGraphicsUnavailable
	}
	return graphics, nil
}

func (context *applicationContext) DrawLine(x1, y1, x2, y2, width int, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.DrawLine(x1, y1, x2, y2, width, paint)
}
func (context *applicationContext) DrawRect(x, y, width, height int, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.DrawRect(x, y, width, height, paint)
}
func (context *applicationContext) FillRect(x, y, width, height int, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.FillRect(x, y, width, height, paint)
}
func (context *applicationContext) DrawEllipse(x, y, width, height, lineWidth int, startAngle, endAngle float32, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.DrawEllipse(x, y, width, height, lineWidth, startAngle, endAngle, paint)
}
func (context *applicationContext) FillEllipse(x, y, width, height int, startAngle, endAngle float32, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.FillEllipse(x, y, width, height, startAngle, endAngle, paint)
}
func (context *applicationContext) DrawTriangle(x1, y1, x2, y2, x3, y3, width int, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.DrawTriangle(x1, y1, x2, y2, x3, y3, width, paint)
}
func (context *applicationContext) FillTriangle(x1, y1, x2, y2, x3, y3 int, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.FillTriangle(x1, y1, x2, y2, x3, y3, paint)
}
func (context *applicationContext) FillPolygon(points []playdate.GraphicsPoint, rule playdate.PolygonFillRule, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.FillPolygon(points, rule, paint)
}
func (context *applicationContext) DrawRoundedRect(x, y, width, height, radius, lineWidth int, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.DrawRoundedRect(x, y, width, height, radius, lineWidth, paint)
}
func (context *applicationContext) FillRoundedRect(x, y, width, height, radius int, paint playdate.Paint) error {
	graphics, err := context.primitiveGraphics()
	if err != nil {
		return err
	}
	return graphics.FillRoundedRect(x, y, width, height, radius, paint)
}

func (context *applicationContext) graphicsState() (playdate.GraphicsState, error) {
	graphics, ok := context.Context.(playdate.GraphicsState)
	if !ok {
		return nil, playdate.ErrGraphicsUnavailable
	}
	return graphics, nil
}
func (context *applicationContext) SetClipRect(x, y, width, height int) error {
	graphics, err := context.graphicsState()
	if err != nil {
		return err
	}
	return graphics.SetClipRect(x, y, width, height)
}
func (context *applicationContext) ClearClipRect() {
	if graphics, err := context.graphicsState(); err == nil {
		graphics.ClearClipRect()
	}
}
func (context *applicationContext) SetDrawOffset(dx, dy int) {
	if graphics, err := context.graphicsState(); err == nil {
		graphics.SetDrawOffset(dx, dy)
	}
}
func (context *applicationContext) SetDrawMode(mode playdate.DrawMode) error {
	graphics, err := context.graphicsState()
	if err != nil {
		return err
	}
	return graphics.SetDrawMode(mode)
}
func (context *applicationContext) SetLineCapStyle(style playdate.LineCapStyle) error {
	graphics, err := context.graphicsState()
	if err != nil {
		return err
	}
	return graphics.SetLineCapStyle(style)
}
func (context *applicationContext) SetBackgroundColor(color playdate.Color) error {
	graphics, err := context.graphicsState()
	if err != nil {
		return err
	}
	return graphics.SetBackgroundColor(color)
}
func (context *applicationContext) SetScreenClipRect(x, y, width, height int) error {
	graphics, err := context.graphicsState()
	if err != nil {
		return err
	}
	return graphics.SetScreenClipRect(x, y, width, height)
}

func (context *applicationContext) WithFramebuffer(callback func(playdate.Framebuffer) error) error {
	graphics, ok := context.Context.(playdate.FramebufferGraphics)
	if !ok {
		return playdate.ErrGraphicsUnavailable
	}
	return graphics.WithFramebuffer(callback)
}

func (context *applicationContext) DrawInto(bitmap playdate.Bitmap, callback func() error) error {
	graphics, ok := context.Context.(playdate.OffscreenGraphics)
	if !ok {
		return playdate.ErrGraphicsUnavailable
	}
	return graphics.DrawInto(bitmap, callback)
}

func (context *applicationContext) DrawRotatedBitmap(bitmap playdate.Bitmap, x, y int, degrees, centerX, centerY, scaleX, scaleY float32) error {
	graphics, ok := context.Context.(playdate.BitmapCompositor)
	if !ok {
		return playdate.ErrGraphicsUnavailable
	}
	return graphics.DrawRotatedBitmap(bitmap, x, y, degrees, centerX, centerY, scaleX, scaleY)
}

func (context *applicationContext) WithStencil(stencil playdate.Bitmap, tiled bool, callback func() error) error {
	if callback == nil {
		return playdate.ErrGraphicsStencilCallback
	}
	if context.stencilActive {
		return playdate.ErrGraphicsStencilActive
	}
	graphics, ok := context.Context.(playdate.BitmapCompositor)
	if !ok {
		return playdate.ErrGraphicsUnavailable
	}
	context.stencilActive = true
	err := graphics.WithStencil(stencil, tiled, callback)
	context.stencilActive = false
	return err
}

func (context *applicationContext) bitmapDataGraphics() (playdate.BitmapDataGraphics, error) {
	graphics, ok := context.Context.(playdate.BitmapDataGraphics)
	if !ok {
		return nil, playdate.ErrGraphicsUnavailable
	}
	return graphics, nil
}
func (context *applicationContext) WithBitmapData(bitmap playdate.Bitmap, callback func(playdate.BitmapData) error) error {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return err
	}
	return graphics.WithBitmapData(bitmap, callback)
}
func (context *applicationContext) CopyBitmap(bitmap playdate.Bitmap) (playdate.Bitmap, error) {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return nil, err
	}
	return graphics.CopyBitmap(bitmap)
}
func (context *applicationContext) LoadIntoBitmap(path string, bitmap playdate.Bitmap) error {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return err
	}
	return graphics.LoadIntoBitmap(path, bitmap)
}
func (context *applicationContext) NewBitmapTable(count, width, height int) (playdate.BitmapTable, error) {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return nil, err
	}
	return graphics.NewBitmapTable(count, width, height)
}
func (context *applicationContext) LoadIntoBitmapTable(path string, table playdate.BitmapTable) error {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return err
	}
	return graphics.LoadIntoBitmapTable(path, table)
}
func (context *applicationContext) SetBitmapMask(bitmap, mask playdate.Bitmap) error {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return err
	}
	return graphics.SetBitmapMask(bitmap, mask)
}
func (context *applicationContext) ClearBitmapMask(bitmap playdate.Bitmap) error {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return err
	}
	return graphics.ClearBitmapMask(bitmap)
}
func (context *applicationContext) BitmapMask(bitmap playdate.Bitmap) (playdate.Bitmap, bool, error) {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return nil, false, err
	}
	return graphics.BitmapMask(bitmap)
}
func (context *applicationContext) CheckBitmapMaskCollision(a playdate.Bitmap, ax, ay int, af playdate.BitmapFlip, b playdate.Bitmap, bx, by int, bf playdate.BitmapFlip, rx, ry, rw, rh int) (bool, error) {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return false, err
	}
	return graphics.CheckBitmapMaskCollision(a, ax, ay, af, b, bx, by, bf, rx, ry, rw, rh)
}
func (context *applicationContext) RotatedBitmap(bitmap playdate.Bitmap, degrees, sx, sy float32) (playdate.Bitmap, int, error) {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return nil, 0, err
	}
	return graphics.RotatedBitmap(bitmap, degrees, sx, sy)
}
func (context *applicationContext) CopyDisplayBuffer() (playdate.Bitmap, error) {
	graphics, err := context.bitmapDataGraphics()
	if err != nil {
		return nil, err
	}
	return graphics.CopyDisplayBuffer()
}

// NewApplication composes a public game with its platform context. beforeInit
// runs immediately before the game's initialization callback when a platform
// adapter needs to prepare callback state.
func NewApplication(game playdate.Game, context playdate.Context, beforeInit func()) (*Application, error) {
	gameContext := &applicationContext{Context: context}
	lifecycle, _ := game.(playdate.LifecycleGame)
	runtime, err := New(Callbacks{
		Init: func() error {
			if beforeInit != nil {
				beforeInit()
			}
			return game.Init(gameContext)
		},
		Lifecycle: func(event playdate.LifecycleEvent) error {
			if event == playdate.LifecycleTerminate {
				gameContext.beginSystemTermination()
			}
			var err error
			if lifecycle != nil {
				err = lifecycle.HandleLifecycle(gameContext, event)
			}
			if event == playdate.LifecycleTerminate {
				if gameContext.microphoneRecorder != nil {
					_ = gameContext.microphoneRecorder.Close()
					gameContext.microphoneRecorder = nil
				}
				if cleanup, ok := gameContext.Context.(interface{ CloseMicrophone() }); ok {
					cleanup.CloseMicrophone()
				}
				gameContext.disableAccelerometer()
				gameContext.removeMenuItems()
				if messages, ok := gameContext.Context.(playdate.DebugMessages); ok {
					for {
						if _, more := messages.PollDebugMessage(); !more {
							break
						}
					}
				}
				gameContext.terminated = true
			}
			return err
		},
		Update: func(input playdate.Input) (bool, error) {
			gameContext.input = input
			return game.Update(gameContext)
		},
	})
	if err != nil {
		return nil, err
	}
	return &Application{runtime: runtime}, nil
}

// Handle delivers a Playdate system event to the application lifecycle.
func (a *Application) Handle(event Event, arg uint32) error {
	return a.runtime.Handle(event, arg)
}

// Update invokes the application's frame callback.
func (a *Application) Update(input RawInput) (int32, error) {
	return a.runtime.Update(input)
}

// New validates callbacks and returns an uninitialized runtime.
func New(callbacks Callbacks) (*Runtime, error) {
	if callbacks.Init == nil {
		return nil, ErrInitRequired
	}
	if callbacks.Update == nil {
		return nil, ErrUpdateRequired
	}
	if callbacks.Lifecycle == nil {
		callbacks.Lifecycle = func(playdate.LifecycleEvent) error { return nil }
	}
	return &Runtime{callbacks: callbacks}, nil
}

// Handle delivers a Playdate system event to the lifecycle.
func (r *Runtime) Handle(event Event, _ uint32) error {
	if r.failed {
		return ErrFailed
	}
	if r.terminated {
		return ErrTerminated
	}
	if event == EventInit {
		if r.initialized {
			return ErrAlreadyInitialized
		}
		if err := r.callbacks.Init(); err != nil {
			r.failed = true
			return err
		}
		r.initialized = true
		return nil
	}
	if !r.initialized {
		return ErrNotInitialized
	}
	lifecycleEvent, ok := lifecycleEvent(event)
	if !ok {
		return nil
	}
	if err := r.callbacks.Lifecycle(lifecycleEvent); err != nil {
		r.failed = true
		return err
	}
	if event == EventTerminate {
		r.terminated = true
	}
	return nil
}

func lifecycleEvent(event Event) (playdate.LifecycleEvent, bool) {
	switch event {
	case EventPause:
		return playdate.LifecyclePause, true
	case EventResume:
		return playdate.LifecycleResume, true
	case EventLock:
		return playdate.LifecycleLock, true
	case EventUnlock:
		return playdate.LifecycleUnlock, true
	case EventTerminate:
		return playdate.LifecycleTerminate, true
	case EventLowPower:
		return playdate.LifecycleLowPower, true
	case EventMirrorStarted:
		return playdate.LifecycleMirrorStarted, true
	case EventMirrorEnded:
		return playdate.LifecycleMirrorEnded, true
	default:
		return 0, false
	}
}

// Update derives and delivers the next frame's portable input snapshot.
func (r *Runtime) Update(raw RawInput) (int32, error) {
	if r.failed {
		return 0, ErrFailed
	}
	if r.terminated {
		return 0, ErrTerminated
	}
	if !r.initialized {
		return 0, ErrNotInitialized
	}
	DrainAudioCallbacks()
	RefillPCMCallbackSources()
	RefillGeneratorSynths()
	previousButtons := r.input.Buttons
	previousDocked := raw.CrankDocked
	if r.hasInput {
		previousDocked = r.input.CrankDocked
	}
	r.input = playdate.Input{
		Buttons: raw.Buttons, Pressed: raw.Buttons &^ previousButtons,
		Released: previousButtons &^ raw.Buttons, Held: raw.Buttons & previousButtons,
		CrankAngle: raw.CrankAngle, CrankDelta: raw.CrankDelta,
		CrankDocked: raw.CrankDocked, CrankDockedThisFrame: r.hasInput && raw.CrankDocked && !previousDocked,
		CrankUndocked: r.hasInput && !raw.CrankDocked && previousDocked,
		DeltaSeconds:  raw.DeltaSeconds,
	}
	r.hasInput = true
	shouldRefresh, err := r.callbacks.Update(r.input)
	if err != nil {
		r.failed = true
		return 0, err
	}
	if shouldRefresh {
		return 1, nil
	}
	return 0, nil
}
