//go:build !darwin

package control

import "github.com/Digital-Shane/open-tamer/internal/core"

type DarwinSignalController struct{}

func NewSignalController() *DarwinSignalController {
	return &DarwinSignalController{}
}

func (controller *DarwinSignalController) Stop(process core.ProcessRef) error {
	return ErrUnsupported
}

func (controller *DarwinSignalController) Continue(process core.ProcessRef) error {
	return ErrUnsupported
}
