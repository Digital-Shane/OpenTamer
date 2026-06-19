//go:build darwin && cgo

package audio

import (
	"errors"
	"testing"
)

func TestAudioObserverSamplesGlobalOutputState(t *testing.T) {
	_, err := NewAudioObserver().AudioActive()
	if err != nil {
		if errors.Is(err, ErrAudioSamplingUnavailable) {
			t.Skipf("audio output sampling unavailable in this environment: %v", err)
		}
		t.Fatalf("audio active: %v", err)
	}
}
