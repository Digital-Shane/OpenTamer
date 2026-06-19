//go:build !darwin || !cgo

package process

import "github.com/Digital-Shane/open-tamer/internal/core"

type ProcessEnumerator struct{}

func NewProcessEnumerator() *ProcessEnumerator {
	return &ProcessEnumerator{}
}

func (enumerator *ProcessEnumerator) Processes() ([]core.ProcessRef, error) {
	return nil, ErrUnsupported
}

func (enumerator *ProcessEnumerator) ProcessGeneration(pid int) (core.ProcessID, error) {
	return core.ProcessID{}, ErrUnsupported
}
