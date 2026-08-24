package runtime

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

// EffectDriver contains operations common to native channel effects.
type EffectDriver struct {
	SetMix          func(uintptr, float32)
	SetMixModulator func(uintptr, uintptr)
	Free            func(uintptr)
}

type effectNode struct {
	handle   uintptr
	driver   EffectDriver
	channels map[*audioChannel]struct{}
	signals  [3]*signalNode
	setters  [3]func(uintptr, uintptr)
	closed   bool
}

func newEffectNode(handle uintptr, driver EffectDriver) *effectNode {
	e := &effectNode{handle: handle, driver: driver, channels: make(map[*audioChannel]struct{})}
	e.setters[0] = driver.SetMixModulator
	return e
}

func (e *effectNode) nativeHandle() (uintptr, error) {
	if e == nil || e.closed || e.handle == 0 {
		return 0, playdate.ErrAudioGraphClosed
	}
	return e.handle, nil
}

func (e *effectNode) SetMix(value float32) error {
	if err := ValidateAudioVolume(value, value); err != nil {
		return err
	}
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	e.driver.SetMix(h, value)
	return nil
}

func (e *effectNode) setSignal(slot int, value playdate.Signal) error {
	h, err := e.nativeHandle()
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
	old := e.signals[slot]
	if old != nil {
		old.effects[e] &^= 1 << slot
		if old.effects[e] == 0 {
			delete(old.effects, e)
		}
	}
	e.setters[slot](h, sh)
	e.signals[slot] = signal
	if signal != nil {
		signal.effects[e] |= 1 << slot
	}
	return nil
}

func (e *effectNode) SetMixModulator(value playdate.Signal) error { return e.setSignal(0, value) }

func (e *effectNode) detachSignal(signal *signalNode, slots uint8) {
	if e.closed {
		return
	}
	for slot := range e.signals {
		if slots&(1<<slot) != 0 && e.signals[slot] == signal {
			e.setters[slot](e.handle, 0)
			e.signals[slot] = nil
		}
	}
}

func (e *effectNode) Close() error {
	h, err := e.nativeHandle()
	if err != nil {
		return err
	}
	for channel := range e.channels {
		if !channel.closed {
			channel.driver.RemoveEffect(channel.handle, h)
			delete(channel.effects, e)
		}
	}
	for slot, signal := range e.signals {
		if signal != nil {
			signal.effects[e] &^= 1 << slot
			if signal.effects[e] == 0 {
				delete(signal.effects, e)
			}
			e.setters[slot](h, 0)
		}
	}
	e.driver.Free(h)
	e.handle = 0
	e.closed = true
	e.channels = nil
	return nil
}

func effectFrom(value playdate.AudioEffect) (*effectNode, error) {
	switch e := value.(type) {
	case *effectNode:
		return e, nil
	case *twoPoleFilter:
		return e.effectNode, nil
	case *onePoleFilter:
		return e.effectNode, nil
	case *bitCrusher:
		return e.effectNode, nil
	case *ringModulator:
		return e.effectNode, nil
	case *delayLine:
		return e.effectNode, nil
	case *overdrive:
		return e.effectNode, nil
	default:
		return nil, playdate.ErrAudioSourceInvalid
	}
}

type OnePoleFilterDriver struct {
	Effect                EffectDriver
	SetParameter          func(uintptr, float32)
	SetParameterModulator func(uintptr, uintptr)
}
type onePoleFilter struct {
	*effectNode
	driver OnePoleFilterDriver
}

func NewOnePoleFilter(h uintptr, d OnePoleFilterDriver) playdate.OnePoleFilter {
	e := newEffectNode(h, d.Effect)
	e.setters[1] = d.SetParameterModulator
	return &onePoleFilter{e, d}
}
func (e *onePoleFilter) SetParameter(v float32) error {
	if !finite(v) || v < -1 || v > 1 {
		return playdate.ErrAudioParameter
	}
	return effectFloat(e.effectNode, v, -1, e.driver.SetParameter)
}
func (e *onePoleFilter) SetParameterModulator(v playdate.Signal) error { return e.setSignal(1, v) }

type TwoPoleFilterDriver struct {
	Effect                                       EffectDriver
	SetType                                      func(uintptr, playdate.FilterType)
	SetFrequency, SetGain, SetResonance          func(uintptr, float32)
	SetFrequencyModulator, SetResonanceModulator func(uintptr, uintptr)
}
type twoPoleFilter struct {
	*effectNode
	driver TwoPoleFilterDriver
}

func NewTwoPoleFilter(h uintptr, d TwoPoleFilterDriver) playdate.TwoPoleFilter {
	e := newEffectNode(h, d.Effect)
	e.setters[1] = d.SetFrequencyModulator
	e.setters[2] = d.SetResonanceModulator
	return &twoPoleFilter{e, d}
}
func (e *twoPoleFilter) SetType(v playdate.FilterType) error {
	if v > playdate.FilterHighShelf {
		return playdate.ErrAudioParameter
	}
	h, x := e.nativeHandle()
	if x == nil {
		e.driver.SetType(h, v)
	}
	return x
}
func (e *twoPoleFilter) SetFrequency(v float32) error {
	return effectFloat(e.effectNode, v, 0, e.driver.SetFrequency)
}
func (e *twoPoleFilter) SetFrequencyModulator(v playdate.Signal) error { return e.setSignal(1, v) }
func (e *twoPoleFilter) SetGain(v float32) error {
	return effectFloat(e.effectNode, v, -1, e.driver.SetGain)
}
func (e *twoPoleFilter) SetResonance(v float32) error {
	return effectFloat(e.effectNode, v, 0, e.driver.SetResonance)
}
func (e *twoPoleFilter) SetResonanceModulator(v playdate.Signal) error { return e.setSignal(2, v) }

type BitCrusherDriver struct {
	Effect                                      EffectDriver
	SetExponential                              func(uintptr, bool)
	SetDepth, SetDownsampling                   func(uintptr, float32)
	SetDepthModulator, SetDownsamplingModulator func(uintptr, uintptr)
}
type bitCrusher struct {
	*effectNode
	driver BitCrusherDriver
}

func NewBitCrusher(h uintptr, d BitCrusherDriver) playdate.BitCrusher {
	e := newEffectNode(h, d.Effect)
	e.setters[1] = d.SetDepthModulator
	e.setters[2] = d.SetDownsamplingModulator
	return &bitCrusher{e, d}
}
func (e *bitCrusher) SetExponential(v bool) error {
	h, x := e.nativeHandle()
	if x == nil {
		e.driver.SetExponential(h, v)
	}
	return x
}
func (e *bitCrusher) SetDepth(v float32) error                  { return effectUnit(e.effectNode, v, e.driver.SetDepth) }
func (e *bitCrusher) SetDepthModulator(v playdate.Signal) error { return e.setSignal(1, v) }
func (e *bitCrusher) SetDownsampling(v float32) error {
	return effectUnit(e.effectNode, v, e.driver.SetDownsampling)
}
func (e *bitCrusher) SetDownsamplingModulator(v playdate.Signal) error { return e.setSignal(2, v) }

type RingModulatorDriver struct {
	Effect                EffectDriver
	SetFrequency          func(uintptr, float32)
	SetFrequencyModulator func(uintptr, uintptr)
}
type ringModulator struct {
	*effectNode
	driver RingModulatorDriver
}

func NewRingModulator(h uintptr, d RingModulatorDriver) playdate.RingModulator {
	e := newEffectNode(h, d.Effect)
	e.setters[1] = d.SetFrequencyModulator
	return &ringModulator{e, d}
}
func (e *ringModulator) SetFrequency(v float32) error {
	return effectFloat(e.effectNode, v, 0, e.driver.SetFrequency)
}
func (e *ringModulator) SetFrequencyModulator(v playdate.Signal) error { return e.setSignal(1, v) }

type OverdriveDriver struct {
	Effect                                EffectDriver
	SetGain, SetLimit, SetOffset          func(uintptr, float32)
	SetLimitModulator, SetOffsetModulator func(uintptr, uintptr)
}
type overdrive struct {
	*effectNode
	driver OverdriveDriver
}

func NewOverdrive(h uintptr, d OverdriveDriver) playdate.Overdrive {
	e := newEffectNode(h, d.Effect)
	e.setters[1] = d.SetLimitModulator
	e.setters[2] = d.SetOffsetModulator
	return &overdrive{e, d}
}
func (e *overdrive) SetGain(v float32) error {
	return effectFloat(e.effectNode, v, -1, e.driver.SetGain)
}
func (e *overdrive) SetLimit(v float32) error                  { return effectUnit(e.effectNode, v, e.driver.SetLimit) }
func (e *overdrive) SetLimitModulator(v playdate.Signal) error { return e.setSignal(1, v) }
func (e *overdrive) SetOffset(v float32) error {
	return effectFloat(e.effectNode, v, -1, e.driver.SetOffset)
}
func (e *overdrive) SetOffsetModulator(v playdate.Signal) error { return e.setSignal(2, v) }

func effectFloat(e *effectNode, v, min float32, set func(uintptr, float32)) error {
	if !finite(v) || v < min {
		return playdate.ErrAudioParameter
	}
	h, x := e.nativeHandle()
	if x == nil {
		set(h, v)
	}
	return x
}
func effectUnit(e *effectNode, v float32, set func(uintptr, float32)) error {
	if v < 0 || v > 1 || !finite(v) {
		return playdate.ErrAudioParameter
	}
	h, x := e.nativeHandle()
	if x == nil {
		set(h, v)
	}
	return x
}

type DelayLineDriver struct {
	Effect                EffectDriver
	SetLength             func(uintptr, int)
	SetFeedback           func(uintptr, float32)
	AddTap                func(uintptr, int) uintptr
	Tap                   AudioDriver
	SetTapDelay           func(uintptr, int)
	SetTapDelayModulator  func(uintptr, uintptr)
	SetTapChannelsFlipped func(uintptr, bool)
}
type delayLine struct {
	*effectNode
	driver DelayLineDriver
	taps   map[*delayTap]struct{}
}
type delayTap struct {
	*audioPlayer
	parent *delayLine
	driver DelayLineDriver
	signal *signalNode
}

func NewDelayLine(h uintptr, d DelayLineDriver) playdate.DelayLine {
	return &delayLine{effectNode: newEffectNode(h, d.Effect), driver: d, taps: make(map[*delayTap]struct{})}
}
func (d *delayLine) SetLength(v int) error {
	if v < 1 {
		return playdate.ErrAudioParameter
	}
	h, x := d.nativeHandle()
	if x == nil {
		d.driver.SetLength(h, v)
	}
	return x
}
func (d *delayLine) SetFeedback(v float32) error {
	return effectUnit(d.effectNode, v, d.driver.SetFeedback)
}
func (d *delayLine) AddTap(v int) (playdate.DelayTap, error) {
	if v < 0 {
		return nil, playdate.ErrAudioParameter
	}
	h, x := d.nativeHandle()
	if x != nil {
		return nil, x
	}
	th := d.driver.AddTap(h, v)
	if th == 0 {
		return nil, playdate.ErrAudioCreate
	}
	t := &delayTap{audioPlayer: &audioPlayer{handle: th, driver: d.driver.Tap}, parent: d, driver: d.driver}
	d.taps[t] = struct{}{}
	return t, nil
}
func (t *delayTap) SetDelay(v int) error {
	if v < 0 {
		return playdate.ErrAudioParameter
	}
	h, x := t.nativeHandle()
	if x == nil {
		t.driver.SetTapDelay(h, v)
	}
	return x
}
func (t *delayTap) SetDelayModulator(v playdate.Signal) error {
	h, x := t.nativeHandle()
	if x != nil {
		return x
	}
	s, x := signalFrom(v)
	if x != nil {
		return x
	}
	var sh uintptr
	if s != nil {
		sh, x = s.nativeHandle()
		if x != nil {
			return x
		}
	}
	if t.signal != nil {
		delete(t.signal.delayTaps, t)
	}
	t.driver.SetTapDelayModulator(h, sh)
	t.signal = s
	if s != nil {
		s.delayTaps[t] = struct{}{}
	}
	return nil
}
func (t *delayTap) SetChannelsFlipped(v bool) error {
	h, x := t.nativeHandle()
	if x == nil {
		t.driver.SetTapChannelsFlipped(h, v)
	}
	return x
}
func (t *delayTap) Close() error {
	if t.signal != nil {
		delete(t.signal.delayTaps, t)
		if !t.closed {
			t.driver.SetTapDelayModulator(t.handle, 0)
		}
		t.signal = nil
	}
	if t.parent != nil {
		delete(t.parent.taps, t)
	}
	return t.audioPlayer.Close()
}
func (d *delayLine) Close() error {
	for t := range d.taps {
		_ = t.Close()
	}
	return d.effectNode.Close()
}
