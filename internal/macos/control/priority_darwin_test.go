//go:build darwin

package control

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestPriorityControllerSetsLowerPriorityForSpawnedProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	process := core.ProcessRef{ID: core.ProcessID{PID: cmd.Process.Pid}, Name: "sleep"}
	controller := NewPriorityController()
	original, err := controller.GetPriority(process)
	if err != nil {
		t.Fatalf("get priority: %v", err)
	}
	if err := controller.SetPriority(process, core.DefaultBackgroundNice); err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("priority policy denied in this environment: %v", err)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Skipf("taskpolicy denied in this environment: %v", exitErr)
		}
		t.Fatalf("set priority: %v", err)
	}
	if err := controller.SetPriority(process, original); err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("priority restore denied in this environment: %v", err)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Skipf("taskpolicy restore denied in this environment: %v", exitErr)
		}
		t.Fatalf("restore priority: %v", err)
	}
}
