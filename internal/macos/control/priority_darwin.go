//go:build darwin

package control

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"

	appcontrol "github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
)

type DarwinPriorityController struct{}

func NewPriorityController() *DarwinPriorityController {
	return &DarwinPriorityController{}
}

func (controller *DarwinPriorityController) GetPriority(process core.ProcessRef) (int, error) {
	if process.ID.PID <= 0 {
		return 0, appcontrol.ErrProcessExited
	}
	priority, err := syscall.Getpriority(syscall.PRIO_PROCESS, process.ID.PID)
	if errors.Is(err, syscall.ESRCH) {
		return 0, appcontrol.ErrProcessExited
	}
	return priority, err
}

func (controller *DarwinPriorityController) SetPriority(process core.ProcessRef, value int) error {
	if process.ID.PID <= 0 {
		return appcontrol.ErrProcessExited
	}
	if value >= core.DefaultBackgroundNice {
		return runTaskPolicy("-b", process.ID.PID)
	}
	if value <= 0 {
		return runTaskPolicy("-B", process.ID.PID)
	}
	err := syscall.Setpriority(syscall.PRIO_PROCESS, process.ID.PID, value)
	if errors.Is(err, syscall.ESRCH) {
		return appcontrol.ErrProcessExited
	}
	return err
}

func runTaskPolicy(flag string, pid int) error {
	err := exec.Command("/usr/sbin/taskpolicy", flag, "-p", fmt.Sprintf("%d", pid)).Run()
	if errors.Is(err, syscall.ESRCH) {
		return appcontrol.ErrProcessExited
	}
	return err
}
