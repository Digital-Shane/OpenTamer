package app

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type controllerRecoverySignals struct {
	limiterSignals
	failContinue atomic.Bool
	failStop     atomic.Bool
	attempts     atomic.Int64
	stopAttempts atomic.Int64
}

func (signals *controllerRecoverySignals) Stop(process core.ProcessRef) error {
	signals.stopAttempts.Add(1)
	if signals.failStop.Load() {
		return errors.New("temporary stop failure")
	}
	return signals.limiterSignals.Stop(process)
}

func TestControllerReportsAsynchronousLimiterFailure(t *testing.T) {
	controller, _ := budgetTestController(t)
	signals := &controllerRecoverySignals{}
	signals.failStop.Store(true)
	controller.limiter = NewCPULimiter(signals)
	controller.groupCtl.Signals = signals
	t.Cleanup(func() {
		signals.failStop.Store(false)
		controller.Shutdown(core.ControlReasonGlobalDisabled)
	})
	controller.Refresh()
	controller.Refresh()
	deadline := time.Now().Add(time.Second)
	for signals.stopAttempts.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("limiter did not attempt a stop")
		}
		time.Sleep(time.Millisecond)
	}
	controller.Refresh()
	group := controller.lastGroups[0]
	if !strings.Contains(controller.InitialState().StatusMessage, "temporary stop failure") || controller.lastStatuses[group.ID.Key()] == core.AppStatusCPULimited {
		t.Fatal("asynchronous limiter failure was not exposed")
	}
	signals.failStop.Store(false)
	if !signals.waitForStops(1, time.Second) {
		t.Fatal("limiter did not recover from a stop failure")
	}
	controller.Refresh()
	if controller.controlError != "" {
		t.Fatalf("recovered failure remained visible: %s", controller.controlError)
	}
}

type recoveryApplications struct{}

func (recoveryApplications) AppProcessHints() ([]core.AppProcessHint, error) { return nil, nil }

func (recoveryApplications) FrontmostAppID() (core.AppID, error) { return core.AppID{}, nil }

func (signals *controllerRecoverySignals) Continue(process core.ProcessRef) error {
	signals.attempts.Add(1)
	if signals.failContinue.Load() {
		return errors.New("temporary resume failure")
	}
	return signals.limiterSignals.Continue(process)
}

func TestShutdownRetainsFailedRecoveryAndAllowsRetry(t *testing.T) {
	controller, _ := budgetTestController(t)
	signals := &controllerRecoverySignals{}
	signals.failContinue.Store(true)
	controller.limiter = NewCPULimiter(signals)
	controller.groupCtl.Signals = signals
	t.Cleanup(func() {
		signals.failContinue.Store(false)
		controller.Shutdown(core.ControlReasonGlobalDisabled)
	})
	controller.Refresh()
	controller.Refresh()
	if !signals.waitForStops(1, time.Second) {
		t.Fatal("limiter did not stop the test process")
	}
	if controller.Shutdown(core.ControlReasonGlobalDisabled) {
		t.Fatal("shutdown succeeded while a process could not be resumed")
	}
	if controller.stopped || !controller.shutdownRequested || len(controller.limiter.PendingRecovery()) != 1 {
		t.Fatal("failed shutdown lost its recovery state")
	}
	state := controller.InitialState()
	if state.Enabled || !strings.Contains(state.StatusMessage, "temporary resume failure") {
		t.Fatalf("recovery failure was not exposed: %+v", state)
	}
	stops := signals.stopCount()
	signals.failContinue.Store(false)
	controller.Refresh()
	if len(controller.limiter.PendingRecovery()) != 0 || signals.stopCount() != stops {
		t.Fatal("recovery refresh started new limits or failed to resume")
	}
	if !controller.Shutdown(core.ControlReasonGlobalDisabled) || !controller.stopped {
		t.Fatal("shutdown did not succeed after recovery")
	}
}

func TestPendingRecoveryDefersConflictingActions(t *testing.T) {
	process := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}}
	replacement := core.ProcessID{PID: 42, StartTime: time.Unix(2, 0)}
	actions := []core.ControlAction{
		{Type: core.ControlActionPause, Processes: []core.ProcessID{process.ID}},
		{Type: core.ControlActionQuit, Processes: []core.ProcessID{process.ID}},
		{Type: core.ControlActionPause, Processes: []core.ProcessID{replacement}},
		{Type: core.ControlActionNoop},
	}
	got := actionsWithoutRecoveryConflicts(actions, []core.ProcessRef{process})
	if len(got) != 2 || !got[0].Processes[0].SameGeneration(replacement) || got[1].Type != core.ControlActionNoop {
		t.Fatalf("unexpected actions after recovery filtering: %+v", got)
	}
}

func TestShutdownRecoveryRejectsNewManagementCommands(t *testing.T) {
	controller, _ := budgetTestController(t)
	controller.shutdownRequested = true
	controller.HandleMenuCommand("toggle-management")
	if !controller.cfg.Preferences.ManagementEnabled {
		t.Fatal("management command changed configuration during shutdown recovery")
	}
}

func TestRecoveryDefersPauseRuleUntilLimiterReleasesProcess(t *testing.T) {
	controller, _ := budgetTestController(t)
	controller.applications = recoveryApplications{}
	signals := &controllerRecoverySignals{}
	signals.failContinue.Store(true)
	controller.limiter = NewCPULimiter(signals)
	controller.groupCtl.Signals = signals
	t.Cleanup(func() {
		signals.failContinue.Store(false)
		controller.Shutdown(core.ControlReasonGlobalDisabled)
	})
	controller.Refresh()
	controller.Refresh()
	if !signals.waitForStops(1, time.Second) {
		t.Fatal("limiter did not stop the test process")
	}
	group := controller.lastGroups[0]
	controller.cfg.Rules = []core.AppRule{{AppID: group.ID, Mode: core.RuleModePauseInBackground}}
	controller.Refresh()
	if signals.stopCount() != 1 || controller.runtime.IsPausedByOpenTamer(group.Processes[0].ID) {
		t.Fatal("pause rule took ownership before limiter recovery")
	}
	if len(controller.limiter.PendingRecovery()) != 1 {
		t.Fatal("pause transition lost pending recovery")
	}
	signals.failContinue.Store(false)
	signals.failStop.Store(true)
	controller.Refresh()
	if controller.runtime.IsPausedByOpenTamer(group.Processes[0].ID) || controller.stats.AutomaticPauses != 0 {
		t.Fatal("failed pause was recorded as successful")
	}
	if controller.lastStatuses[group.ID.Key()] == core.AppStatusPaused || !strings.Contains(controller.InitialState().StatusMessage, "temporary stop failure") {
		t.Fatal("failed pause was not exposed in the panel")
	}
	signals.failStop.Store(false)
	controller.Refresh()
	if signals.stopCount() != 2 || !controller.runtime.IsPausedByOpenTamer(group.Processes[0].ID) {
		t.Fatal("pause rule did not take ownership after successful recovery")
	}
	if len(controller.limiter.PendingRecovery()) != 0 {
		t.Fatal("recovery ownership was not released")
	}
	controller.cfg.Rules = nil
	controller.cfg.Preferences.GlobalCPUPercent = nil
	controller.Refresh()
	if controller.runtime.IsPausedByOpenTamer(group.Processes[0].ID) {
		t.Fatal("removing pause rule did not resume the process")
	}
}
