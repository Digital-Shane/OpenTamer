//go:build darwin && cgo

package audio

/*
#cgo darwin LDFLAGS: -framework CoreAudio
#include "audio_bridge.h"
*/
import "C"

import "errors"

var ErrAudioSamplingUnavailable = errors.New("audio output sampling is unavailable")

type AudioObserver struct{}

func NewAudioObserver() *AudioObserver {
	return &AudioObserver{}
}

func (observer *AudioObserver) AudioActive() (bool, error) {
	result := C.OpenTamerAudioOutputIsRunning()
	if result < 0 {
		return false, ErrAudioSamplingUnavailable
	}
	return result == 1, nil
}
