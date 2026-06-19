//go:build darwin

package control

import (
	"os/exec"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestSignalControllerStopsAndContinuesSpawnedProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = NewSignalController().Continue(core.ProcessRef{ID: core.ProcessID{PID: cmd.Process.Pid}})
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	process := core.ProcessRef{ID: core.ProcessID{PID: cmd.Process.Pid}, Name: "sleep"}
	controller := NewSignalController()
	if err := controller.Stop(process); err != nil {
		t.Fatalf("stop: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if err := controller.Continue(process); err != nil {
		t.Fatalf("continue: %v", err)
	}
}
