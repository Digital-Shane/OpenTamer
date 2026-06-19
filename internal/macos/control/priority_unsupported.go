//go:build !darwin

package control

import "github.com/Digital-Shane/open-tamer/internal/core"

type DarwinPriorityController struct{}

func NewPriorityController() *DarwinPriorityController {
	return &DarwinPriorityController{}
}

func (controller *DarwinPriorityController) GetPriority(process core.ProcessRef) (int, error) {
	return 0, ErrUnsupported
}

func (controller *DarwinPriorityController) SetPriority(process core.ProcessRef, value int) error {
	return ErrUnsupported
}
