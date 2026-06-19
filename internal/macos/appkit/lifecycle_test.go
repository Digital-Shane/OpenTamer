package appkit

import (
	"errors"
	"testing"

	"github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestLifecycleControllerSatisfiesCoreInterface(t *testing.T) {
	var _ app.LifecycleController = NewLifecycleController(nil)
}

func TestNewLifecycleControllerDefaultsBridge(t *testing.T) {
	controller := NewLifecycleController(nil)
	if controller.Bridge == nil {
		t.Fatal("expected default AppKit bridge")
	}
}

func TestLifecycleControllerForwardsProcessPID(t *testing.T) {
	bridge := &fakeAppKitController{}
	controller := &AppKitLifecycleController{Bridge: bridge}
	process := core.ProcessRef{ID: core.ProcessID{PID: 4321}}

	if err := controller.Hide(process); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if err := controller.Activate(process); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := controller.Terminate(process); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	want := []int{4321, 4321, 4321}
	if len(bridge.pids) != len(want) {
		t.Fatalf("forwarded pids = %#v, want %#v", bridge.pids, want)
	}
	for i := range want {
		if bridge.pids[i] != want[i] {
			t.Fatalf("forwarded pids = %#v, want %#v", bridge.pids, want)
		}
	}
}

func TestLifecycleControllerPropagatesBridgeErrors(t *testing.T) {
	want := errors.New("native appkit action failed")
	controller := &AppKitLifecycleController{Bridge: &fakeAppKitController{err: want}}

	err := controller.Hide(core.ProcessRef{ID: core.ProcessID{PID: 4321}})
	if !errors.Is(err, want) {
		t.Fatalf("hide error = %v, want %v", err, want)
	}
}

type fakeAppKitController struct {
	pids []int
	err  error
}

func (controller *fakeAppKitController) Hide(pid int) error {
	controller.pids = append(controller.pids, pid)
	return controller.err
}

func (controller *fakeAppKitController) Activate(pid int) error {
	controller.pids = append(controller.pids, pid)
	return controller.err
}

func (controller *fakeAppKitController) Terminate(pid int) error {
	controller.pids = append(controller.pids, pid)
	return controller.err
}
