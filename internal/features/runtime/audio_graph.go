package runtime

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

// SignalDriver contains native operations shared by owned modulation nodes.
type SignalDriver struct {
	Value     func(uintptr) float32
	SetScale  func(uintptr, float32)
	SetOffset func(uintptr, float32)
	Free      func(uintptr)
}

// LFODriver contains native operations specific to a low-frequency oscillator.
type LFODriver struct {
	Signal          SignalDriver
	SetRate         func(uintptr, float32)
	SetPhase        func(uintptr, float32)
	SetStartPhase   func(uintptr, float32)
	SetRandomSeed   func(uintptr, uint16)
	SetGlobal       func(uintptr, bool)
	SetCenter       func(uintptr, float32)
	SetDepth        func(uintptr, float32)
	SetRetrigger    func(uintptr, bool)
	SetArpeggiation func(uintptr, []float32)
}

// EnvelopeDriver contains native operations specific to an ADSR envelope.
type EnvelopeDriver struct {
	Signal                               SignalDriver
	SetAttack                            func(uintptr, float32)
	SetDecay                             func(uintptr, float32)
	SetSustain                           func(uintptr, float32)
	SetRelease                           func(uintptr, float32)
	SetLegato                            func(uintptr, bool)
	SetRetrigger                         func(uintptr, bool)
	SetCurvature, SetVelocitySensitivity func(uintptr, float32)
	SetRateScaling                       func(uintptr, float32, uint8, uint8)
}

// ControlSignalDriver contains native operations for a step timeline.
type ControlSignalDriver struct {
	Signal      SignalDriver
	AddEvent    func(uintptr, int, float32, bool)
	RemoveEvent func(uintptr, int)
	ClearEvents func(uintptr)
}

type signalNode struct {
	handle        uintptr
	driver        SignalDriver
	synths        map[*synth]uint8
	effects       map[*effectNode]uint8
	delayTaps     map[*delayTap]struct{}
	channels      map[*audioChannel]uint8
	players       map[*audioPlayer]struct{}
	closed        bool
	borrowedOwner *audioChannel
	borrowedKind  uint8
}

type lfo struct {
	*signalNode
	driver LFODriver
}

type envelope struct {
	*signalNode
	driver EnvelopeDriver
}

type controlSignal struct {
	*signalNode
	driver ControlSignalDriver
}

func newSignalNode(handle uintptr, driver SignalDriver) *signalNode {
	return &signalNode{handle: handle, driver: driver, synths: make(map[*synth]uint8), effects: make(map[*effectNode]uint8), delayTaps: make(map[*delayTap]struct{}), channels: make(map[*audioChannel]uint8), players: make(map[*audioPlayer]struct{})}
}

func newBorrowedSignalNode(handle uintptr, driver SignalDriver, owner *audioChannel, kind uint8) *signalNode {
	signal := newSignalNode(handle, driver)
	signal.borrowedOwner = owner
	signal.borrowedKind = kind
	return signal
}

// NewLFO wraps an owned native low-frequency oscillator.
func NewLFO(handle uintptr, driver LFODriver) playdate.LFO {
	return &lfo{signalNode: newSignalNode(handle, driver.Signal), driver: driver}
}

// NewEnvelope wraps an owned native ADSR envelope.
func NewEnvelope(handle uintptr, driver EnvelopeDriver) playdate.Envelope {
	return &envelope{signalNode: newSignalNode(handle, driver.Signal), driver: driver}
}

// NewControlSignal wraps an owned native control timeline.
func NewControlSignal(handle uintptr, driver ControlSignalDriver) playdate.ControlSignal {
	return &controlSignal{signalNode: newSignalNode(handle, driver.Signal), driver: driver}
}

func (s *signalNode) nativeHandle() (uintptr, error) {
	if s == nil || s.closed || s.handle == 0 {
		return 0, playdate.ErrAudioGraphClosed
	}
	return s.handle, nil
}

func finite(value float32) bool {
	return value == value && value <= 3.4028235e38 && value >= -3.4028235e38
}

func (s *signalNode) Value() (float32, error) {
	handle, err := s.nativeHandle()
	if err != nil {
		return 0, err
	}
	return s.driver.Value(handle), nil
}
func (s *signalNode) SetScale(value float32) error {
	if !finite(value) {
		return playdate.ErrAudioParameter
	}
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetScale(handle, value)
	return nil
}
func (s *signalNode) SetOffset(value float32) error {
	if !finite(value) {
		return playdate.ErrAudioParameter
	}
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetOffset(handle, value)
	return nil
}
func (s *signalNode) Close() error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	for synth, edges := range s.synths {
		if edges&1 != 0 {
			synth.driver.SetFrequencyModulator(synth.handle, 0)
			synth.frequency = nil
		}
		if edges&2 != 0 {
			synth.driver.SetAmplitudeModulator(synth.handle, 0)
			synth.amplitude = nil
		}
	}
	for effect, slots := range s.effects {
		effect.detachSignal(s, slots)
	}
	for tap := range s.delayTaps {
		if !tap.closed {
			tap.driver.SetTapDelayModulator(tap.handle, 0)
			tap.signal = nil
		}
	}
	for channel, slots := range s.channels {
		if slots&1 != 0 {
			channel.driver.SetVolumeModulator(channel.handle, 0)
			channel.volumeSignal = nil
		}
		if slots&2 != 0 {
			channel.driver.SetPanModulator(channel.handle, 0)
			channel.panSignal = nil
		}
	}
	for player := range s.players {
		if !player.closed {
			player.driver.SetRateModulator(player.handle, 0)
			player.rateSignal = nil
		}
	}
	s.synths = nil
	s.effects = nil
	s.delayTaps = nil
	s.channels = nil
	s.players = nil
	if s.borrowedOwner == nil && s.driver.Free != nil {
		s.driver.Free(handle)
	}
	if s.borrowedOwner != nil {
		if s.borrowedKind == 1 && s.borrowedOwner.drySignal == s {
			s.borrowedOwner.drySignal = nil
		}
		if s.borrowedKind == 2 && s.borrowedOwner.wetSignal == s {
			s.borrowedOwner.wetSignal = nil
		}
		s.borrowedOwner = nil
	}
	s.handle = 0
	s.closed = true
	return nil
}

func (l *lfo) SetRate(value float32) error {
	if !finite(value) || value < 0 {
		return playdate.ErrAudioParameter
	}
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetRate(h, value)
	return nil
}
func (l *lfo) SetPhase(value float32) error {
	if !finite(value) {
		return playdate.ErrAudioParameter
	}
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetPhase(h, value)
	return nil
}
func (l *lfo) SetStartPhase(value float32) error {
	if !finite(value) || value < 0 || value > 1 {
		return playdate.ErrAudioParameter
	}
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetStartPhase(h, value)
	return nil
}
func (l *lfo) SetRandomSeed(value uint16) error {
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetRandomSeed(h, value)
	return nil
}
func (l *lfo) SetGlobal(value bool) error {
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetGlobal(h, value)
	return nil
}
func (l *lfo) SetCenter(value float32) error {
	if !finite(value) {
		return playdate.ErrAudioParameter
	}
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetCenter(h, value)
	return nil
}
func (l *lfo) SetDepth(value float32) error {
	if !finite(value) {
		return playdate.ErrAudioParameter
	}
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetDepth(h, value)
	return nil
}
func (l *lfo) SetRetrigger(value bool) error {
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetRetrigger(h, value)
	return nil
}
func (l *lfo) SetArpeggiation(steps []float32) error {
	if len(steps) == 0 {
		return playdate.ErrAudioParameter
	}
	for _, step := range steps {
		if !finite(step) {
			return playdate.ErrAudioParameter
		}
	}
	h, err := l.nativeHandle()
	if err != nil {
		return err
	}
	l.driver.SetArpeggiation(h, steps)
	return nil
}

func envelopeTime(value float32) error {
	if !finite(value) || value < 0 {
		return playdate.ErrAudioParameter
	}
	return nil
}

// ValidateEnvelope rejects ADSR values that the native API cannot use.
func ValidateEnvelope(attack, decay, sustain, release float32) error {
	if envelopeTime(attack) != nil || envelopeTime(decay) != nil || envelopeTime(release) != nil || ValidateAudioVolume(sustain, sustain) != nil {
		return playdate.ErrAudioParameter
	}
	return nil
}
func (e *envelope) SetAttack(value float32) error {
	if err := envelopeTime(value); err != nil {
		return err
	}
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	e.driver.SetAttack(h, value)
	return nil
}
func (e *envelope) SetDecay(value float32) error {
	if err := envelopeTime(value); err != nil {
		return err
	}
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	e.driver.SetDecay(h, value)
	return nil
}
func (e *envelope) SetSustain(value float32) error {
	if err := ValidateAudioVolume(value, value); err != nil {
		return err
	}
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	e.driver.SetSustain(h, value)
	return nil
}
func (e *envelope) SetRelease(value float32) error {
	if err := envelopeTime(value); err != nil {
		return err
	}
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	e.driver.SetRelease(h, value)
	return nil
}
func (e *envelope) SetLegato(value bool) error {
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	e.driver.SetLegato(h, value)
	return nil
}
func (e *envelope) SetRetrigger(value bool) error {
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	e.driver.SetRetrigger(h, value)
	return nil
}
func (e *envelope) SetCurvature(v float32) error { return e.setExtra(v, e.driver.SetCurvature) }
func (e *envelope) SetVelocitySensitivity(v float32) error {
	return e.setExtra(v, e.driver.SetVelocitySensitivity)
}
func (e *envelope) setExtra(v float32, set func(uintptr, float32)) error {
	if !finite(v) {
		return playdate.ErrAudioParameter
	}
	h, err := e.nativeHandle()
	if err == nil {
		set(h, v)
	}
	return err
}
func (e *envelope) SetRateScaling(v float32, start, end uint8) error {
	if !finite(v) || start > end {
		return playdate.ErrAudioParameter
	}
	h, err := e.nativeHandle()
	if err == nil {
		e.driver.SetRateScaling(h, v, start, end)
	}
	return err
}

func (c *controlSignal) AddEvent(step int, value float32, interpolate bool) error {
	if step < 0 {
		return playdate.ErrAudioEventStep
	}
	if !finite(value) {
		return playdate.ErrAudioParameter
	}
	h, err := c.nativeHandle()
	if err != nil {
		return err
	}
	c.driver.AddEvent(h, step, value, interpolate)
	return nil
}
func (c *controlSignal) RemoveEvent(step int) error {
	if step < 0 {
		return playdate.ErrAudioEventStep
	}
	h, err := c.nativeHandle()
	if err != nil {
		return err
	}
	c.driver.RemoveEvent(h, step)
	return nil
}
func (c *controlSignal) ClearEvents() error {
	h, err := c.nativeHandle()
	if err != nil {
		return err
	}
	c.driver.ClearEvents(h)
	return nil
}

// SynthDriver contains native operations for an owned waveform synthesizer.
type SynthDriver struct {
	Audio                          AudioDriver
	SetWaveform                    func(uintptr, playdate.Waveform)
	SetEnvelope                    func(uintptr, float32, float32, float32, float32)
	SetEnvelopeCurvature           func(uintptr, float32)
	SetEnvelopeVelocitySensitivity func(uintptr, float32)
	SetEnvelopeRateScaling         func(uintptr, float32, uint8, uint8)
	SetTranspose                   func(uintptr, float32)
	SetFrequencyModulator          func(uintptr, uintptr)
	SetAmplitudeModulator          func(uintptr, uintptr)
	PlayMIDINote                   func(uintptr, float32, float32, float32, uint32)
	NoteOff                        func(uintptr, uint32)
	SetWavetable                   func(uintptr, uintptr, int, int, int) bool
	SetParameter                   func(uintptr, int, float32) bool
	SetParameterModulator          func(uintptr, int, uintptr)
}

func (s *synth) SetWavetable(value playdate.AudioSample, log2Size, columns, rows int) error {
	if s.generator != nil {
		return playdate.ErrAudioUnavailable
	}
	if log2Size < 0 || columns < 1 || rows < 1 {
		return playdate.ErrAudioParameter
	}
	sample, ok := value.(*audioSample)
	if !ok {
		return playdate.ErrAudioSourceInvalid
	}
	sh, err := sample.nativeHandle()
	if err != nil {
		return err
	}
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if s.driver.SetWavetable == nil || !s.driver.SetWavetable(h, sh, log2Size, columns, rows) {
		return playdate.ErrAudioParameter
	}
	return nil
}
func (s *synth) SetParameter(parameter int, value float32) error {
	if parameter < 1 || !finite(value) {
		return playdate.ErrAudioParameter
	}
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if s.driver.SetParameter == nil || !s.driver.SetParameter(h, parameter, value) {
		return playdate.ErrAudioParameter
	}
	return nil
}
func (s *synth) SetParameterModulator(parameter int, value playdate.Signal) error {
	if parameter < 1 {
		return playdate.ErrAudioParameter
	}
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	n, err := signalFrom(value)
	if err != nil {
		return err
	}
	var sh uintptr
	if n != nil {
		sh = n.handle
	}
	if s.driver.SetParameterModulator == nil {
		return playdate.ErrAudioUnavailable
	}
	s.driver.SetParameterModulator(h, parameter, sh)
	return nil
}

type synth struct {
	*audioPlayer
	driver      SynthDriver
	frequency   *signalNode
	amplitude   *signalNode
	instruments map[*instrument]struct{}
	generator   *generatorSynthSource
}

// NewSynth wraps an owned native waveform synthesizer.
func NewSynth(handle uintptr, driver SynthDriver) playdate.Synth {
	return &synth{audioPlayer: &audioPlayer{handle: handle, driver: driver.Audio}, driver: driver}
}

func (s *synth) SetWaveform(value playdate.Waveform) error {
	if s.generator != nil {
		return playdate.ErrAudioUnavailable
	}
	if value > playdate.WaveformPOVosim {
		return playdate.ErrAudioWaveform
	}
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetWaveform(h, value)
	return nil
}
func (s *synth) SetEnvelope(a, d, sustain, r float32) error {
	if err := ValidateEnvelope(a, d, sustain, r); err != nil {
		return err
	}
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetEnvelope(h, a, d, sustain, r)
	return nil
}
func (s *synth) SetEnvelopeCurvature(v float32) error {
	if !finite(v) {
		return playdate.ErrAudioParameter
	}
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetEnvelopeCurvature(h, v)
	}
	return e
}
func (s *synth) SetEnvelopeVelocitySensitivity(v float32) error {
	if !finite(v) {
		return playdate.ErrAudioParameter
	}
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetEnvelopeVelocitySensitivity(h, v)
	}
	return e
}
func (s *synth) SetEnvelopeRateScaling(v float32, a, b uint8) error {
	if !finite(v) || a > b {
		return playdate.ErrAudioParameter
	}
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetEnvelopeRateScaling(h, v, a, b)
	}
	return e
}
func (s *synth) SetTranspose(value float32) error {
	if !finite(value) {
		return playdate.ErrAudioParameter
	}
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.SetTranspose(h, value)
	return nil
}

func signalFrom(value playdate.Signal) (*signalNode, error) {
	if value == nil {
		return nil, nil
	}
	switch signal := value.(type) {
	case *signalNode:
		return signal, nil
	case *lfo:
		return signal.signalNode, nil
	case *envelope:
		return signal.signalNode, nil
	case *controlSignal:
		return signal.signalNode, nil
	default:
		return nil, playdate.ErrAudioSourceInvalid
	}
}
func (s *synth) setModulator(value playdate.Signal, amplitude bool) error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	signal, err := signalFrom(value)
	if err != nil {
		return err
	}
	var sh uintptr
	if signal != nil {
		sh, err = signal.nativeHandle()
		if err != nil {
			return err
		}
	}
	old := s.frequency
	bit := uint8(1)
	setter := s.driver.SetFrequencyModulator
	if amplitude {
		old = s.amplitude
		bit = 2
		setter = s.driver.SetAmplitudeModulator
	}
	if old != nil {
		old.synths[s] &^= bit
		if old.synths[s] == 0 {
			delete(old.synths, s)
		}
	}
	setter(h, sh)
	if amplitude {
		s.amplitude = signal
	} else {
		s.frequency = signal
	}
	if signal != nil {
		signal.synths[s] |= bit
	}
	return nil
}
func (s *synth) SetFrequencyModulator(value playdate.Signal) error {
	return s.setModulator(value, false)
}
func (s *synth) SetAmplitudeModulator(value playdate.Signal) error {
	return s.setModulator(value, true)
}
func (s *synth) PlayMIDINote(note, velocity, length float32, when uint32) error {
	if !finite(note) || !finite(length) || length < -1 || ValidateAudioVolume(velocity, velocity) != nil {
		return playdate.ErrAudioParameter
	}
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.PlayMIDINote(h, note, velocity, length, when)
	s.paused = false
	return nil
}
func (s *synth) NoteOff(when uint32) error {
	h, err := s.nativeHandle()
	if err != nil {
		return err
	}
	s.driver.NoteOff(h, when)
	return nil
}
func (s *synth) Close() error {
	handle, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if len(s.instruments) != 0 {
		return playdate.ErrAudioRoute
	}
	if s.generator != nil {
		delete(generatorSynthSources, s.generator)
		s.generator.callback = nil
		s.generator = nil
	}
	if s.frequency != nil {
		delete(s.frequency.synths, s)
		s.driver.SetFrequencyModulator(handle, 0)
	}
	if s.amplitude != nil {
		delete(s.amplitude.synths, s)
		s.driver.SetAmplitudeModulator(handle, 0)
	}
	return s.audioPlayer.Close()
}
