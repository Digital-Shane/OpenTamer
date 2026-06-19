//go:build !darwin || !cgo

package audio

type AudioObserver struct{}

func NewAudioObserver() *AudioObserver {
	return &AudioObserver{}
}

func (observer *AudioObserver) AudioActive() (bool, error) {
	return false, ErrUnsupported
}
