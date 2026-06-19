package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

func processControlGroup(app core.AppID, pid int) core.AppGroup {
	return core.AppGroup{
		ID:              app,
		Kind:            core.AppKindUserApp,
		Controllability: core.ControllabilityNormal,
		Status:          core.AppStatusObserved,
		Processes: []core.ProcessRef{{
			ID:   core.ProcessID{PID: pid},
			Name: app.DisplayName(),
		}},
	}
}

func TestAppGroupControllerPauseRecordsOwnedState(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Sleeper"}, 42)
	signals := &fakeSignals{}
	result := (&AppGroupController{Signals: signals}).PauseGroup(group, core.NewRuntimeState(), core.ControlReasonUserRule, time.Unix(1, 0))

	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if !result.Runtime.IsPausedByOpenTamer(group.Processes[0].ID) {
		t.Fatal("expected pause ownership to be recorded")
	}
	if len(signals.stopped) != 1 {
		t.Fatalf("stopped = %d, want 1", len(signals.stopped))
	}
}

func TestAppGroupControllerPauseRecordsOnlySuccessfulProcesses(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Sleeper"}, 42)
	group.Processes = append(group.Processes, core.ProcessRef{ID: core.ProcessID{PID: 43, StartTime: time.Unix(43, 0)}, Name: "helper"})
	signals := &fakeSignals{stopErrors: map[int]error{43: errors.New("permission denied")}}

	result := (&AppGroupController{Signals: signals}).PauseGroup(group, core.NewRuntimeState(), core.ControlReasonUserRule, time.Unix(1, 0))
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(result.Failures))
	}
	if !result.Runtime.IsPausedByOpenTamer(group.Processes[0].ID) {
		t.Fatal("expected first process to be recorded")
	}
	if result.Runtime.IsPausedByOpenTamer(group.Processes[1].ID) {
		t.Fatal("failed process should not be recorded as paused")
	}
}

func TestAppGroupControllerResumeClearsExitedProcess(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Sleeper"}, 42)
	state := core.NewRuntimeState()
	state.RecordPause(group.Processes[0], group.ID, core.ControlReasonUserRule, time.Unix(1, 0))
	signals := &fakeSignals{continueErrors: map[int]error{42: ErrProcessExited}}

	result := (&AppGroupController{Signals: signals}).ResumeGroup(group, state, core.ControlReasonForeground, time.Unix(2, 0))
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if result.Runtime.IsPausedByOpenTamer(group.Processes[0].ID) {
		t.Fatal("exited process should be cleared from pause state")
	}
}

func TestExecuteActionsSkipsResumeForMismatchedProcessGeneration(t *testing.T) {
	app := core.AppID{Name: "Sleeper"}
	oldProcess := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, Name: "Sleeper"}
	newProcess := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(2, 0)}, Name: "Sleeper"}
	state := core.NewRuntimeState()
	state.RecordPause(oldProcess, app, core.ControlReasonUserRule, time.Unix(1, 0))
	signals := &fakeSignals{}
	controller := &Controller{groupCtl: &AppGroupController{
		Signals:          signals,
		ProcessValidator: fakeGenerationValidator{current: map[int]core.ProcessID{42: newProcess.ID}},
	}}

	runtimeState, failures := controller.executeActions(
		[]core.AppGroup{{ID: app, Processes: []core.ProcessRef{newProcess}}},
		[]core.ControlAction{{
			Type:      core.ControlActionResume,
			AppID:     app,
			Processes: []core.ProcessID{oldProcess.ID},
			Reason:    core.ControlReasonGlobalDisabled,
			At:        time.Unix(3, 0),
		}},
		state,
	)

	if len(signals.continued) != 0 {
		t.Fatalf("continued = %#v, want no signal for reused PID", signals.continued)
	}
	if runtimeState.IsPausedByOpenTamer(oldProcess.ID) {
		t.Fatal("stale pause state should be cleared")
	}
	if len(failures) != 1 || !errors.Is(failures[0].Err, ErrProcessGenerationMismatch) {
		t.Fatalf("failures = %#v, want process generation mismatch", failures)
	}
}

func TestSchedulerEmitsPeriodicWakeForPausedApp(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := processControlGroup(app, 42)
	state := core.NewRuntimeState()
	state.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(1, 0))
	state.AppStates[app.Key()] = core.AppControlRuntime{
		AppID:              app,
		FirstSeenAt:        time.Unix(1, 0),
		BackgroundSince:    time.Unix(1, 0),
		LastPeriodicWakeAt: time.Unix(5, 0),
		Status:             core.AppStatusPaused,
	}

	result := apppolicy.NewScheduler().Evaluate(apppolicy.SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground, PeriodicWakeEvery: 10 * time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     state,
		Now:         time.Unix(16, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionWakeBriefly {
		t.Fatalf("actions = %#v, want wake briefly", result.Actions)
	}
	if result.Statuses[app.Key()] != core.AppStatusTemporarilyAwake {
		t.Fatalf("status = %q", result.Statuses[app.Key()])
	}
}

func TestAppGroupControllerSetPriorityRecordsOriginalNice(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Worker"}, 42)
	priority := &fakePriority{values: map[int]int{42: 0}}
	controller := &AppGroupController{Priority: priority}

	result := controller.SetPriorityGroup(group, core.NewRuntimeState(), 10, core.ControlReasonUserRule, time.Unix(1, 0))
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	change, ok := result.Runtime.PriorityChanges[group.Processes[0].ID.Key()]
	if !ok {
		t.Fatal("expected priority change to be recorded")
	}
	if change.OriginalNice != 0 || change.TargetNice != 10 {
		t.Fatalf("change = %#v", change)
	}
}

func TestAppGroupControllerDoesNotImprovePriorityWithoutPrivilege(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Worker"}, 42)
	priority := &fakePriority{values: map[int]int{42: 10}}
	controller := &AppGroupController{Priority: priority}

	result := controller.SetPriorityGroup(group, core.NewRuntimeState(), 0, core.ControlReasonUserRule, time.Unix(1, 0))
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(result.Failures))
	}
	if !errors.Is(result.Failures[0].Err, ErrPriorityImprovementRequiresPrivilege) {
		t.Fatalf("failure err = %v", result.Failures[0].Err)
	}
	if priority.values[42] != 10 {
		t.Fatalf("priority was changed to %d", priority.values[42])
	}
}

func TestAppGroupControllerRestorePriorityClearsStateWhenAllowed(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Worker"}, 42)
	state := core.NewRuntimeState()
	state.RecordPriorityChange(group.Processes[0], group.ID, 8, 10, core.ControlReasonUserRule, time.Unix(1, 0))
	priority := &fakePriority{values: map[int]int{42: 10}}
	controller := &AppGroupController{Priority: priority, AllowPriorityImprovement: true}

	result := controller.RestorePriorityGroup(group, state, core.ControlReasonForeground, time.Unix(2, 0))
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if _, ok := result.Runtime.PriorityChanges[group.Processes[0].ID.Key()]; ok {
		t.Fatal("expected priority state to be cleared")
	}
	if priority.values[42] != 8 {
		t.Fatalf("priority = %d, want 8", priority.values[42])
	}
}

func TestAppGroupControllerRestorePrioritySkipsMismatchedProcessGeneration(t *testing.T) {
	app := core.AppID{Name: "Worker"}
	oldProcess := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, Name: "Worker"}
	newProcess := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(2, 0)}, Name: "Worker"}
	group := core.AppGroup{ID: app, Processes: []core.ProcessRef{oldProcess}}
	state := core.NewRuntimeState()
	state.RecordPriorityChange(oldProcess, app, 8, 10, core.ControlReasonUserRule, time.Unix(1, 0))
	priority := &fakePriority{values: map[int]int{42: 10}}
	controller := &AppGroupController{
		Priority:                 priority,
		ProcessValidator:         fakeGenerationValidator{current: map[int]core.ProcessID{42: newProcess.ID}},
		AllowPriorityImprovement: true,
	}

	result := controller.RestorePriorityGroup(group, state, core.ControlReasonForeground, time.Unix(2, 0))
	if len(result.Failures) != 1 || !errors.Is(result.Failures[0].Err, ErrProcessGenerationMismatch) {
		t.Fatalf("failures = %#v, want process generation mismatch", result.Failures)
	}
	if _, ok := result.Runtime.PriorityChanges[oldProcess.ID.Key()]; ok {
		t.Fatal("stale priority state should be cleared")
	}
	if priority.values[42] != 10 {
		t.Fatalf("priority = %d, want unchanged 10", priority.values[42])
	}
}

func TestSchedulerEmitsSetPriorityAction(t *testing.T) {
	app := core.AppID{BundleID: "com.example.worker", Name: "Worker"}
	group := processControlGroup(app, 42)
	nice := 12
	result := apppolicy.NewScheduler().Evaluate(apppolicy.SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeLowerPriorityInBackground, NiceValue: &nice}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(1, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionSetPriority {
		t.Fatalf("actions = %#v, want set priority", result.Actions)
	}
	if result.Actions[0].NiceValue == nil || *result.Actions[0].NiceValue != 12 {
		t.Fatalf("nice value = %#v, want 12", result.Actions[0].NiceValue)
	}
}

func TestAppGroupControllerHideRecordsHiddenState(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Editor"}, 42)
	lifecycle := &fakeLifecycle{}
	controller := &AppGroupController{Lifecycle: lifecycle}

	result := controller.HideGroup(group, core.NewRuntimeState(), core.ControlReasonUserRule, time.Unix(1, 0))
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if !result.Runtime.IsHiddenByOpenTamer(group.ID) {
		t.Fatal("expected hidden state")
	}
	if len(lifecycle.hidden) != 1 {
		t.Fatalf("hidden calls = %d, want 1", len(lifecycle.hidden))
	}
}

func TestAppGroupControllerActivateClearsHiddenState(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Editor"}, 42)
	state := core.NewRuntimeState()
	state.RecordHidden(group.Processes[0], group.ID, core.ControlReasonUserRule, time.Unix(1, 0))
	lifecycle := &fakeLifecycle{}

	result := (&AppGroupController{Lifecycle: lifecycle}).ActivateGroup(group, state, core.ControlReasonForeground, time.Unix(2, 0))
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if result.Runtime.IsHiddenByOpenTamer(group.ID) {
		t.Fatal("hidden state should be cleared")
	}
}

func TestAppGroupControllerQuitUsesGracefulTerminate(t *testing.T) {
	group := processControlGroup(core.AppID{Name: "Editor"}, 42)
	lifecycle := &fakeLifecycle{}

	result := (&AppGroupController{Lifecycle: lifecycle}).QuitGroup(group, core.NewRuntimeState(), core.ControlReasonUserRule, time.Unix(1, 0))
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if len(lifecycle.terminated) != 1 {
		t.Fatalf("terminated = %d, want 1", len(lifecycle.terminated))
	}
}

func TestSchedulerHideWhenStoppedEmitsHideAfterPause(t *testing.T) {
	app := core.AppID{Name: "Editor"}
	group := processControlGroup(app, 42)
	result := apppolicy.NewScheduler().Evaluate(apppolicy.SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground, HideWhenStopped: true}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(1, 0),
	})

	if len(result.Actions) != 2 {
		t.Fatalf("actions = %#v, want pause and hide", result.Actions)
	}
	if result.Actions[0].Type != core.ControlActionPause || result.Actions[1].Type != core.ControlActionHide {
		t.Fatalf("actions = %#v", result.Actions)
	}
}

func TestSchedulerQuitAfterIdleEmitsGracefulQuitOnly(t *testing.T) {
	app := core.AppID{Name: "Editor"}
	group := processControlGroup(app, 42)
	result := apppolicy.NewScheduler().Evaluate(apppolicy.SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeQuitAfterIdle}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(1, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionQuit {
		t.Fatalf("actions = %#v, want graceful quit", result.Actions)
	}
}

type fakeSignals struct {
	stopped        []int
	continued      []int
	stopErrors     map[int]error
	continueErrors map[int]error
}

type fakePriority struct {
	values map[int]int
	errors map[int]error
}

type fakeLifecycle struct {
	hidden     []int
	activated  []int
	terminated []int
	errors     map[int]error
}

type fakeGenerationValidator struct {
	current map[int]core.ProcessID
}

func (lifecycle *fakeLifecycle) Hide(process core.ProcessRef) error {
	if err := lifecycle.errors[process.ID.PID]; err != nil {
		return err
	}
	lifecycle.hidden = append(lifecycle.hidden, process.ID.PID)
	return nil
}

func (lifecycle *fakeLifecycle) Activate(process core.ProcessRef) error {
	if err := lifecycle.errors[process.ID.PID]; err != nil {
		return err
	}
	lifecycle.activated = append(lifecycle.activated, process.ID.PID)
	return nil
}

func (lifecycle *fakeLifecycle) Terminate(process core.ProcessRef) error {
	if err := lifecycle.errors[process.ID.PID]; err != nil {
		return err
	}
	lifecycle.terminated = append(lifecycle.terminated, process.ID.PID)
	return nil
}

func (priority *fakePriority) GetPriority(process core.ProcessRef) (int, error) {
	if err := priority.errors[process.ID.PID]; err != nil {
		return 0, err
	}
	return priority.values[process.ID.PID], nil
}

func (priority *fakePriority) SetPriority(process core.ProcessRef, value int) error {
	if err := priority.errors[process.ID.PID]; err != nil {
		return err
	}
	priority.values[process.ID.PID] = value
	return nil
}

func (signals *fakeSignals) Stop(process core.ProcessRef) error {
	if err := signals.stopErrors[process.ID.PID]; err != nil {
		return err
	}
	signals.stopped = append(signals.stopped, process.ID.PID)
	return nil
}

func (signals *fakeSignals) Continue(process core.ProcessRef) error {
	if err := signals.continueErrors[process.ID.PID]; err != nil {
		return err
	}
	signals.continued = append(signals.continued, process.ID.PID)
	return nil
}

func (validator fakeGenerationValidator) ValidateProcessGeneration(process core.ProcessRef) error {
	current, ok := validator.current[process.ID.PID]
	if !ok {
		return ErrProcessExited
	}
	if !process.ID.SameGeneration(current) {
		return ErrProcessGenerationMismatch
	}
	return nil
}
