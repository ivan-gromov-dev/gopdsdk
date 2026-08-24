package runtime

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

// InstrumentDriver contains native operations for an owned voice bank.
type InstrumentDriver struct {
	AddVoice          func(uintptr, uintptr, uint8, uint8, float32) bool
	SetPitchBend      func(uintptr, float32)
	SetPitchBendRange func(uintptr, float32)
	SetTranspose      func(uintptr, float32)
	NoteOff           func(uintptr, uint8, uint32)
	AllNotesOff       func(uintptr, uint32)
	SetVolume         func(uintptr, float32, float32)
	Volume            func(uintptr) (float32, float32)
	ActiveVoiceCount  func(uintptr) int
	Free              func(uintptr)
}

type instrument struct {
	*audioPlayer
	handle uintptr
	driver InstrumentDriver
	voices map[*synth]struct{}
	tracks map[*sequenceTrack]struct{}
	closed bool
}

// NewInstrument wraps an owned native voice bank.
func NewInstrument(handle uintptr, driver InstrumentDriver) playdate.Instrument {
	audio := AudioDriver{Source: func(h uintptr) uintptr { return h }, Stop: func(h uintptr) {
		if driver.AllNotesOff != nil {
			driver.AllNotesOff(h, 0)
		}
	}, SetVolume: driver.SetVolume, Volume: driver.Volume, IsPlaying: func(h uintptr) bool { return driver.ActiveVoiceCount != nil && driver.ActiveVoiceCount(h) != 0 }, Pause: func(uintptr, bool) {}, Free: func(h uintptr) {
		if driver.Free != nil {
			driver.Free(h)
		}
	}}
	return &instrument{audioPlayer: &audioPlayer{handle: handle, driver: audio}, handle: handle, driver: driver, voices: make(map[*synth]struct{}), tracks: make(map[*sequenceTrack]struct{})}
}

func (i *instrument) nativeHandle() (uintptr, error) {
	if i == nil || i.closed || i.handle == 0 {
		return 0, playdate.ErrAudioGraphClosed
	}
	return i.handle, nil
}

func instrumentFrom(value playdate.Instrument) (*instrument, error) {
	if value == nil {
		return nil, nil
	}
	i, ok := value.(*instrument)
	if !ok {
		return nil, playdate.ErrAudioSourceInvalid
	}
	if _, err := i.nativeHandle(); err != nil {
		return nil, err
	}
	return i, nil
}

func (i *instrument) AddVoice(value playdate.Synth, start, end uint8, transpose float32) error {
	h, err := i.nativeHandle()
	if err != nil {
		return err
	}
	if start > end || !finite(transpose) {
		return playdate.ErrAudioParameter
	}
	s, ok := value.(*synth)
	if !ok {
		return playdate.ErrAudioSourceInvalid
	}
	sh, err := s.nativeHandle()
	if err != nil {
		return err
	}
	if len(s.channels) != 0 || len(s.instruments) != 0 {
		return playdate.ErrAudioRoute
	}
	if !i.driver.AddVoice(h, sh, start, end, transpose) {
		return playdate.ErrAudioRoute
	}
	i.voices[s] = struct{}{}
	if s.instruments == nil {
		s.instruments = make(map[*instrument]struct{})
	}
	s.instruments[i] = struct{}{}
	return nil
}

func (i *instrument) SetPitchBend(v float32) error {
	return instrumentFloat(i, v, i.driver.SetPitchBend)
}
func (i *instrument) SetPitchBendRange(v float32) error {
	if v < 0 {
		return playdate.ErrAudioParameter
	}
	return instrumentFloat(i, v, i.driver.SetPitchBendRange)
}
func (i *instrument) SetTranspose(v float32) error {
	return instrumentFloat(i, v, i.driver.SetTranspose)
}
func instrumentFloat(i *instrument, v float32, set func(uintptr, float32)) error {
	if !finite(v) {
		return playdate.ErrAudioParameter
	}
	h, err := i.nativeHandle()
	if err != nil {
		return err
	}
	set(h, v)
	return nil
}
func (i *instrument) NoteOff(note uint8, when uint32) error {
	h, e := i.nativeHandle()
	if e == nil {
		i.driver.NoteOff(h, note, when)
	}
	return e
}
func (i *instrument) AllNotesOff(when uint32) error {
	h, e := i.nativeHandle()
	if e == nil {
		i.driver.AllNotesOff(h, when)
	}
	return e
}
func (i *instrument) SetVolume(l, r float32) error {
	if e := ValidateAudioVolume(l, r); e != nil {
		return e
	}
	h, e := i.nativeHandle()
	if e == nil {
		i.driver.SetVolume(h, l, r)
	}
	return e
}
func (i *instrument) Volume() (float32, float32, error) {
	h, e := i.nativeHandle()
	if e != nil {
		return 0, 0, e
	}
	l, r := i.driver.Volume(h)
	return l, r, nil
}
func (i *instrument) ActiveVoiceCount() (int, error) {
	h, e := i.nativeHandle()
	if e != nil {
		return 0, e
	}
	return i.driver.ActiveVoiceCount(h), nil
}
func (i *instrument) Close() error {
	_, err := i.nativeHandle()
	if err != nil {
		return err
	}
	for track := range i.tracks {
		if !track.closed {
			track.driver.SetInstrument(track.handle, 0)
			track.instrument = nil
		}
	}
	for voice := range i.voices {
		delete(voice.instruments, i)
	}
	if err := i.audioPlayer.Close(); err != nil {
		return err
	}
	i.handle = 0
	i.closed = true
	i.tracks = nil
	i.voices = nil
	return nil
}

// TrackDriver contains native operations for an owned sequence track.
type TrackDriver struct {
	SetInstrument               func(uintptr, uintptr)
	AddNote                     func(uintptr, uint32, uint32, uint8, float32)
	RemoveNote                  func(uintptr, uint32, uint8)
	ClearNotes                  func(uintptr)
	AddControlEvent             func(uintptr, int, int, float32, bool) bool
	RemoveControlEvent          func(uintptr, int, int) bool
	ClearControlEvents          func(uintptr)
	SetMuted                    func(uintptr, bool)
	Length                      func(uintptr) uint32
	Instrument                  func(uintptr) uintptr
	ControlSignalCount          func(uintptr) int
	ControlSignal               func(uintptr, int) uintptr
	SignalForController         func(uintptr, int, bool) uintptr
	Polyphony, ActiveVoiceCount func(uintptr) int
	NoteIndexAtStep             func(uintptr, uint32) int
	NoteAt                      func(uintptr, int) (uint32, uint32, uint8, float32, bool)
	Free                        func(uintptr)
}

func (t *sequenceTrack) Instrument() (playdate.Instrument, error) {
	_, e := t.nativeHandle()
	if e != nil {
		return nil, e
	}
	if t.instrument == nil {
		return nil, nil
	}
	return t.instrument, nil
}
func (t *sequenceTrack) ControlSignalCount() (int, error) {
	h, e := t.nativeHandle()
	if e != nil {
		return 0, e
	}
	return t.driver.ControlSignalCount(h), nil
}
func (t *sequenceTrack) borrowedSignal(h uintptr) playdate.ControlSignal {
	if h == 0 {
		return nil
	}
	d := ControlSignalDriver{Signal: SignalDriver{Value: func(uintptr) float32 { return 0 }, SetScale: func(uintptr, float32) {}, SetOffset: func(uintptr, float32) {}, Free: func(uintptr) {}}}
	return NewControlSignal(h, d)
}
func (t *sequenceTrack) ControlSignal(index int) (playdate.ControlSignal, error) {
	if index < 0 {
		return nil, playdate.ErrAudioParameter
	}
	h, e := t.nativeHandle()
	if e != nil {
		return nil, e
	}
	return t.borrowedSignal(t.driver.ControlSignal(h, index)), nil
}
func (t *sequenceTrack) SignalForController(c int, create bool) (playdate.ControlSignal, error) {
	if c < 0 {
		return nil, playdate.ErrAudioParameter
	}
	h, e := t.nativeHandle()
	if e != nil {
		return nil, e
	}
	return t.borrowedSignal(t.driver.SignalForController(h, c, create)), nil
}
func (t *sequenceTrack) Polyphony() (int, error) {
	h, e := t.nativeHandle()
	if e != nil {
		return 0, e
	}
	return t.driver.Polyphony(h), nil
}
func (t *sequenceTrack) ActiveVoiceCount() (int, error) {
	h, e := t.nativeHandle()
	if e != nil {
		return 0, e
	}
	return t.driver.ActiveVoiceCount(h), nil
}
func (t *sequenceTrack) NoteIndexAtStep(step uint32) (int, error) {
	h, e := t.nativeHandle()
	if e != nil {
		return 0, e
	}
	return t.driver.NoteIndexAtStep(h, step), nil
}
func (t *sequenceTrack) NoteAt(index int) (playdate.SequenceNote, bool, error) {
	if index < 0 {
		return playdate.SequenceNote{}, false, playdate.ErrAudioParameter
	}
	h, e := t.nativeHandle()
	if e != nil {
		return playdate.SequenceNote{}, false, e
	}
	s, l, n, v, ok := t.driver.NoteAt(h, index)
	return playdate.SequenceNote{Step: s, Length: l, Note: n, Velocity: v}, ok, nil
}

type sequenceTrack struct {
	handle     uintptr
	driver     TrackDriver
	instrument *instrument
	sequences  map[*sequence]uint
	closed     bool
}

func NewSequenceTrack(h uintptr, d TrackDriver) playdate.SequenceTrack {
	return &sequenceTrack{handle: h, driver: d, sequences: make(map[*sequence]uint)}
}
func (t *sequenceTrack) nativeHandle() (uintptr, error) {
	if t == nil || t.closed || t.handle == 0 {
		return 0, playdate.ErrAudioGraphClosed
	}
	return t.handle, nil
}
func trackFrom(v playdate.SequenceTrack) (*sequenceTrack, error) {
	if v == nil {
		return nil, nil
	}
	t, ok := v.(*sequenceTrack)
	if !ok {
		return nil, playdate.ErrAudioSourceInvalid
	}
	if _, e := t.nativeHandle(); e != nil {
		return nil, e
	}
	return t, nil
}
func (t *sequenceTrack) SetInstrument(v playdate.Instrument) error {
	h, e := t.nativeHandle()
	if e != nil {
		return e
	}
	i, e := instrumentFrom(v)
	if e != nil {
		return e
	}
	var ih uintptr
	if i != nil {
		ih = i.handle
	}
	if t.instrument != nil {
		delete(t.instrument.tracks, t)
	}
	t.driver.SetInstrument(h, ih)
	t.instrument = i
	if i != nil {
		i.tracks[t] = struct{}{}
	}
	return nil
}
func (t *sequenceTrack) AddNote(step, length uint32, note uint8, velocity float32) error {
	if length == 0 || ValidateAudioVolume(velocity, velocity) != nil {
		return playdate.ErrAudioParameter
	}
	h, e := t.nativeHandle()
	if e == nil {
		t.driver.AddNote(h, step, length, note, velocity)
	}
	return e
}
func (t *sequenceTrack) RemoveNote(step uint32, note uint8) error {
	h, e := t.nativeHandle()
	if e == nil {
		t.driver.RemoveNote(h, step, note)
	}
	return e
}
func (t *sequenceTrack) ClearNotes() error {
	h, e := t.nativeHandle()
	if e == nil {
		t.driver.ClearNotes(h)
	}
	return e
}
func (t *sequenceTrack) AddControlEvent(controller, step int, value float32, interpolate bool) error {
	if controller < 0 || step < 0 || !finite(value) {
		return playdate.ErrAudioParameter
	}
	h, e := t.nativeHandle()
	if e != nil {
		return e
	}
	if !t.driver.AddControlEvent(h, controller, step, value, interpolate) {
		return playdate.ErrAudioCreate
	}
	return nil
}
func (t *sequenceTrack) RemoveControlEvent(controller, step int) error {
	if controller < 0 || step < 0 {
		return playdate.ErrAudioParameter
	}
	h, e := t.nativeHandle()
	if e != nil {
		return e
	}
	if !t.driver.RemoveControlEvent(h, controller, step) {
		return playdate.ErrAudioRoute
	}
	return nil
}
func (t *sequenceTrack) ClearControlEvents() error {
	h, e := t.nativeHandle()
	if e == nil {
		t.driver.ClearControlEvents(h)
	}
	return e
}
func (t *sequenceTrack) SetMuted(v bool) error {
	h, e := t.nativeHandle()
	if e == nil {
		t.driver.SetMuted(h, v)
	}
	return e
}
func (t *sequenceTrack) Length() (uint32, error) {
	h, e := t.nativeHandle()
	if e != nil {
		return 0, e
	}
	return t.driver.Length(h), nil
}
func (t *sequenceTrack) Close() error {
	h, e := t.nativeHandle()
	if e != nil {
		return e
	}
	for s, index := range t.sequences {
		if !s.closed {
			s.driver.SetTrack(s.handle, index, 0)
			delete(s.tracks, index)
		}
	}
	if t.instrument != nil {
		t.driver.SetInstrument(h, 0)
		delete(t.instrument.tracks, t)
	}
	t.driver.Free(h)
	t.handle = 0
	t.closed = true
	t.sequences = nil
	return nil
}

// SequenceDriver contains native operations for an owned music sequence.
type SequenceDriver struct {
	LoadMIDI    func(uintptr, string) bool
	SetTempo    func(uintptr, float32)
	Tempo       func(uintptr) float32
	SetLoops    func(uintptr, int, int, int)
	TrackCount  func(uintptr) uint
	AddTrack    func(uintptr) uintptr
	SetTrack    func(uintptr, uint, uintptr)
	Play        func(uintptr, uint32)
	Stop        func(uintptr)
	IsPlaying   func(uintptr) bool
	Time        func(uintptr) uint32
	SetTime     func(uintptr, uint32)
	Length      func(uintptr) uint32
	Free        func(uintptr)
	GetTrack    func(uintptr, uint) uintptr
	CurrentStep func(uintptr) (int, int)
	AllNotesOff func(uintptr)
	Track       TrackDriver
}

func (s *sequence) TrackCount() (uint, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, e
	}
	return s.driver.TrackCount(h), nil
}
func (s *sequence) Track(index uint) (playdate.SequenceTrack, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return nil, e
	}
	if t := s.tracks[index]; t != nil {
		return t, nil
	}
	th := s.driver.GetTrack(h, index)
	if th == 0 {
		return nil, nil
	}
	t := NewSequenceTrack(th, s.driver.Track).(*sequenceTrack)
	t.driver.Free = func(uintptr) {}
	return t, nil
}
func (s *sequence) CurrentStep() (int, int, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, 0, e
	}
	a, b := s.driver.CurrentStep(h)
	return a, b, nil
}
func (s *sequence) AllNotesOff() error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.AllNotesOff(h)
	}
	return e
}

type sequence struct {
	handle   uintptr
	driver   SequenceDriver
	tracks   map[uint]*sequenceTrack
	callback uint32
	closed   bool
}

func NewSequence(h uintptr, d SequenceDriver) playdate.Sequence {
	return &sequence{handle: h, driver: d, tracks: make(map[uint]*sequenceTrack)}
}
func (s *sequence) nativeHandle() (uintptr, error) {
	if s == nil || s.closed || s.handle == 0 {
		return 0, playdate.ErrAudioGraphClosed
	}
	return s.handle, nil
}
func (s *sequence) LoadMIDI(path string) error {
	if path == "" {
		return playdate.ErrAudioParameter
	}
	if len(s.tracks) != 0 {
		return playdate.ErrAudioRoute
	}
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	if !s.driver.LoadMIDI(h, path) {
		return playdate.AudioLoadError(path)
	}
	return nil
}
func (s *sequence) SetTempo(v float32) error {
	if !finite(v) || v <= 0 {
		return playdate.ErrAudioParameter
	}
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetTempo(h, v)
	}
	return e
}
func (s *sequence) Tempo() (float32, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, e
	}
	return s.driver.Tempo(h), nil
}
func (s *sequence) SetLoops(start, end, count int) error {
	if start < 0 || end < start || count < 0 {
		return playdate.ErrAudioParameter
	}
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetLoops(h, start, end, count)
	}
	return e
}
func (s *sequence) SetTrack(index uint, value playdate.SequenceTrack) error {
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	t, e := trackFrom(value)
	if e != nil {
		return e
	}
	if t != nil && s.driver.TrackCount != nil && s.driver.AddTrack != nil {
		for s.driver.TrackCount(h) <= index {
			if s.driver.AddTrack(h) == 0 {
				return playdate.ErrAudioCreate
			}
		}
	}
	if old := s.tracks[index]; old != nil {
		delete(old.sequences, s)
	}
	var th uintptr
	if t != nil {
		th = t.handle
		s.tracks[index] = t
		t.sequences[s] = index
	} else {
		delete(s.tracks, index)
	}
	s.driver.SetTrack(h, index, th)
	return nil
}
func (s *sequence) Play(callback func()) error {
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	ForgetAudioCallback(s.callback)
	s.callback = RegisterAudioCallback(callback)
	s.driver.Play(h, s.callback)
	return nil
}
func (s *sequence) Stop() error {
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	s.driver.Stop(h)
	ForgetAudioCallback(s.callback)
	s.callback = 0
	return nil
}
func (s *sequence) IsPlaying() (bool, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return false, e
	}
	return s.driver.IsPlaying(h), nil
}
func (s *sequence) Time() (uint32, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, e
	}
	return s.driver.Time(h), nil
}
func (s *sequence) SetTime(v uint32) error {
	h, e := s.nativeHandle()
	if e == nil {
		s.driver.SetTime(h, v)
	}
	return e
}
func (s *sequence) Length() (uint32, error) {
	h, e := s.nativeHandle()
	if e != nil {
		return 0, e
	}
	return s.driver.Length(h), nil
}
func (s *sequence) Close() error {
	h, e := s.nativeHandle()
	if e != nil {
		return e
	}
	s.driver.Stop(h)
	ForgetAudioCallback(s.callback)
	for index, t := range s.tracks {
		s.driver.SetTrack(h, index, 0)
		delete(t.sequences, s)
	}
	s.driver.Free(h)
	s.handle = 0
	s.closed = true
	s.tracks = nil
	s.callback = 0
	return nil
}
