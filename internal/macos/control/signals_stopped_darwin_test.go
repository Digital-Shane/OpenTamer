//go:build darwin && cgo

package control

import (
	"errors"
	"os/exec"
	"testing"
	"time"

	appcontrol "github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
	macprocess "github.com/Digital-Shane/open-tamer/internal/macos/process"
)

func TestSignalControllerIsStoppedSpawnedProcess(t *testing.T) {
	cmd := startStoppedTestWorker(t)
	id, err := macprocess.NewProcessEnumerator().ProcessGeneration(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("read worker generation: %v", err)
	}
	process := core.ProcessRef{ID: id}
	controller := NewSignalController()
	waitForStoppedState(t, controller, process, false)

	if err := controller.Stop(process); err != nil {
		t.Fatalf("stop worker: %v", err)
	}
	waitForStoppedState(t, controller, process, true)

	// Repeated reads must leave a manually stopped worker stopped.
	for range 5 {
		waitForStoppedState(t, controller, process, true)
	}
	waitForStoppedState(t, controller, core.ProcessRef{ID: core.ProcessID{PID: id.PID}}, true)
	stale := process
	stale.ID.StartTime = id.StartTime.Add(-time.Second)
	if stopped, err := controller.IsStopped(stale); stopped || !errors.Is(err, appcontrol.ErrProcessGenerationMismatch) {
		t.Fatalf("stale generation: stopped=%v, err=%v", stopped, err)
	}

	if err := controller.Continue(process); err != nil {
		t.Fatalf("continue worker: %v", err)
	}
	waitForStoppedState(t, controller, process, false)
}

func TestSignalControllerIsStoppedInvalidPID(t *testing.T) {
	for _, pid := range []int{-1, 0, 1 << 31, 1<<32 + 1} {
		stopped, err := NewSignalController().IsStopped(core.ProcessRef{ID: core.ProcessID{PID: pid}})
		if stopped || !errors.Is(err, appcontrol.ErrProcessExited) {
			t.Errorf("pid %d: stopped=%v, err=%v", pid, stopped, err)
		}
	}
}

func TestSignalControllerIsStoppedExitedPID(t *testing.T) {
	cmd := startStoppedTestWorker(t)
	process := core.ProcessRef{ID: core.ProcessID{PID: cmd.Process.Pid}}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill worker: %v", err)
	}
	_ = cmd.Wait()
	stopped, err := NewSignalController().IsStopped(process)
	if stopped || !errors.Is(err, appcontrol.ErrProcessExited) {
		t.Fatalf("exited worker: stopped=%v, err=%v", stopped, err)
	}
}

func startStoppedTestWorker(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd
}

func waitForStoppedState(t *testing.T, controller *DarwinSignalController, process core.ProcessRef, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		stopped, err := controller.IsStopped(process)
		if err != nil {
			t.Fatalf("read worker state: %v", err)
		}
		if stopped == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("worker stopped=%v, want %v", stopped, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
