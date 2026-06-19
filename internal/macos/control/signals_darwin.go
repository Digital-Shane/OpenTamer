//go:build darwin

package control

import (
	"errors"
	"syscall"

	appcontrol "github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
)

type DarwinSignalController struct{}

func NewSignalController() *DarwinSignalController {
	return &DarwinSignalController{}
}

func (controller *DarwinSignalController) Stop(process core.ProcessRef) error {
	return signalProcess(process.ID.PID, syscall.SIGSTOP)
}

func (controller *DarwinSignalController) Continue(process core.ProcessRef) error {
	return signalProcess(process.ID.PID, syscall.SIGCONT)
}

func signalProcess(pid int, signal syscall.Signal) error {
	if pid <= 0 {
		return appcontrol.ErrProcessExited
	}
	err := syscall.Kill(pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return appcontrol.ErrProcessExited
	}
	return err
}
