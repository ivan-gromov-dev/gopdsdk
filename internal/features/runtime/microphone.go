package runtime

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

// MicrophoneDriver adapts the single native microphone callback slot.
type MicrophoneDriver struct {
	Request func(string, func(bool)) playdate.MicrophonePermission
	Start   func(playdate.MicrophoneSource, func([]int16) bool) playdate.MicrophoneSource
	Stop    func()
}

// MicrophoneService owns permission callbacks and the active recorder.
type MicrophoneService struct {
	driver     MicrophoneDriver
	permission playdate.MicrophonePermission
	requestID  uint64
	active     *microphoneRecorder
}

// NewMicrophoneService creates an optional microphone capability.
func NewMicrophoneService(driver MicrophoneDriver) *MicrophoneService {
	return &MicrophoneService{driver: driver, permission: playdate.MicrophonePermissionPending}
}

func (service *MicrophoneService) RequestMicrophoneAccess(purpose string, callback func(playdate.MicrophonePermission)) (playdate.MicrophonePermission, error) {
	if callback == nil {
		return service.permission, playdate.ErrMicrophoneCallback
	}
	service.requestID++
	id := service.requestID
	result := service.driver.Request(purpose, func(allowed bool) {
		if id != service.requestID {
			return
		}
		result := playdate.MicrophonePermissionDenied
		if allowed {
			result = playdate.MicrophonePermissionGranted
		} else {
			service.stopActive()
		}
		service.permission = result
		callback(result)
	})
	service.permission = result
	if result != playdate.MicrophonePermissionPending {
		service.requestID++
		callback(result)
	}
	return result, nil
}

func (service *MicrophoneService) StartMicrophoneRecording(source playdate.MicrophoneSource, callback func(playdate.MicrophoneSamples) bool) (playdate.MicrophoneRecorder, error) {
	if source > playdate.MicrophoneSourceHeadset {
		return nil, playdate.ErrMicrophoneSource
	}
	if callback == nil {
		return nil, playdate.ErrMicrophoneCallback
	}
	if service.permission == playdate.MicrophonePermissionDenied {
		return nil, playdate.ErrMicrophoneDenied
	}
	service.stopActive()
	recorder := &microphoneRecorder{service: service}
	used := service.driver.Start(source, func(data []int16) bool {
		if recorder.closed || service.active != recorder {
			return false
		}
		samples := microphoneSamples{data: data, active: true}
		keep := callback(&samples)
		samples.active = false
		samples.data = nil
		if !keep {
			recorder.closed = true
			service.active = nil
		}
		return keep
	})
	if used == playdate.MicrophoneSourceAutomatic {
		recorder.closed = true
		return nil, playdate.ErrMicrophoneStart
	}
	recorder.source = used
	service.active = recorder
	return recorder, nil
}

func (service *MicrophoneService) stopActive() {
	if service == nil || service.active == nil {
		return
	}
	service.driver.Stop()
	service.active.closed = true
	service.active = nil
}

// Close stops recording and invalidates pending permission callbacks.
func (service *MicrophoneService) Close() {
	if service == nil {
		return
	}
	service.requestID++
	service.stopActive()
}

type microphoneRecorder struct {
	service *MicrophoneService
	source  playdate.MicrophoneSource
	closed  bool
}

func (recorder *microphoneRecorder) Source() playdate.MicrophoneSource { return recorder.source }

func (recorder *microphoneRecorder) Stop() error {
	if recorder == nil || recorder.closed {
		return playdate.ErrMicrophoneClosed
	}
	recorder.service.stopActive()
	return nil
}

func (recorder *microphoneRecorder) Close() error { return recorder.Stop() }

type microphoneSamples struct {
	data   []int16
	active bool
}

func (samples *microphoneSamples) Len() int {
	if samples == nil || !samples.active {
		return 0
	}
	return len(samples.data)
}

func (samples *microphoneSamples) CopyTo(destination []int16) (int, error) {
	if samples == nil || !samples.active {
		return 0, playdate.ErrMicrophoneSamplesExpired
	}
	return copy(destination, samples.data), nil
}
