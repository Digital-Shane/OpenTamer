package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestSchedulerNeverPausesForegroundAppByDefault(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	result := NewScheduler().Evaluate(SchedulerInput{
		Groups: []core.AppGroup{group},
		Rules:  []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
		Preferences: core.GlobalPreferences{
			ManagementEnabled: true,
		},
		Runtime:   core.NewRuntimeState(),
		Frontmost: app,
		Now:       time.Unix(10, 0),
	})

	if len(result.Actions) != 0 {
		t.Fatalf("actions = %#v, want none", result.Actions)
	}
	if result.Statuses[app.Key()] != core.AppStatusEligible {
		t.Fatalf("status = %q", result.Statuses[app.Key()])
	}
}

func TestSchedulerResumesPausedAppWhenForeground(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	runtime := core.NewRuntimeState()
	runtime.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(5, 0))

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     runtime,
		Frontmost:   app,
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionResume {
		t.Fatalf("actions = %#v, want resume", result.Actions)
	}
}

func TestSchedulerGlobalOffRestoresOwnedActions(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	runtime := core.NewRuntimeState()
	runtime.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(5, 0))
	runtime.RecordPriorityChange(group.Processes[0], app, 0, 10, core.ControlReasonUserRule, time.Unix(5, 0))

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Preferences: core.GlobalPreferences{ManagementEnabled: false},
		Runtime:     runtime,
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 2 {
		t.Fatalf("len(actions) = %d, want 2: %#v", len(result.Actions), result.Actions)
	}
	types := map[core.ControlActionType]bool{}
	for _, action := range result.Actions {
		types[action.Type] = true
	}
	if !types[core.ControlActionResume] || !types[core.ControlActionRestorePriority] {
		t.Fatalf("actions = %#v", result.Actions)
	}
}

func TestSchedulerRestoresOwnedActionsWhenRuleRemoved(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	runtime := core.NewRuntimeState()
	runtime.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(5, 0))
	runtime.RecordPriorityChange(group.Processes[0], app, 0, core.DefaultBackgroundNice, core.ControlReasonUserRule, time.Unix(5, 0))
	runtime.RecordHidden(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(5, 0))
	runtime.AppStates[app.Key()] = core.AppControlRuntime{
		AppID:           app,
		Status:          core.AppStatusCPULimited,
		CPULimitTarget:  1,
		CPULimitRunFor:  time.Second,
		CPULimitStopFor: 9 * time.Second,
	}

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       nil,
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     runtime,
		Now:         time.Unix(10, 0),
	})

	types := map[core.ControlActionType]bool{}
	for _, action := range result.Actions {
		types[action.Type] = true
	}
	if !types[core.ControlActionResume] || !types[core.ControlActionRestorePriority] || !types[core.ControlActionActivate] {
		t.Fatalf("actions = %#v, want resume, restore priority, and activate", result.Actions)
	}
	appRuntime := result.Runtime.AppStates[app.Key()]
	if appRuntime.CPULimitTarget != 0 || appRuntime.CPULimitRunFor != 0 || appRuntime.CPULimitStopFor != 0 {
		t.Fatalf("CPU limit runtime was not cleared: %#v", appRuntime)
	}
	if result.Statuses[app.Key()] != core.AppStatusObserved {
		t.Fatalf("status = %q, want observed", result.Statuses[app.Key()])
	}
}

func TestSchedulerRestoresStalePauseWhenSwitchingToPriorityRule(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	runtime := core.NewRuntimeState()
	runtime.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(5, 0))
	nice := core.DefaultBackgroundNice

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups: []core.AppGroup{group},
		Rules: []core.AppRule{{
			AppID:     app,
			Mode:      core.RuleModeLowerPriorityInBackground,
			NiceValue: &nice,
		}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     runtime,
		Now:         time.Unix(10, 0),
	})

	types := map[core.ControlActionType]bool{}
	for _, action := range result.Actions {
		types[action.Type] = true
	}
	if !types[core.ControlActionResume] || !types[core.ControlActionSetPriority] {
		t.Fatalf("actions = %#v, want resume and set priority", result.Actions)
	}
}

func TestSchedulerWaitsBeforeApplyingRule(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	scheduler := NewScheduler()
	runtime := core.NewRuntimeState()
	first := scheduler.Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground, WaitBeforeApply: 5 * time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     runtime,
		Now:         time.Unix(10, 0),
	})

	if first.Statuses[app.Key()] != core.AppStatusWaiting {
		t.Fatalf("first status = %q, want waiting", first.Statuses[app.Key()])
	}
	second := scheduler.Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground, WaitBeforeApply: 5 * time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     first.Runtime,
		Now:         time.Unix(16, 0),
	})
	if len(second.Actions) != 1 || second.Actions[0].Type != core.ControlActionPause {
		t.Fatalf("second actions = %#v, want pause", second.Actions)
	}
}

func TestSchedulerSafetyVetoesPause(t *testing.T) {
	app := core.AppID{BundleID: "com.example.browser", Name: "Browser"}
	group := schedulerGroup(app, 10)
	group.Controllability = core.ControllabilitySlowOnly

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionNoop {
		t.Fatalf("actions = %#v, want safety noop", result.Actions)
	}
	if result.Statuses[app.Key()] != core.AppStatusBlockedBySafety {
		t.Fatalf("status = %q, want blocked by safety", result.Statuses[app.Key()])
	}
}

func TestSchedulerEmitsCPULimitAction(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	target := 25.0
	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}},
		AppSamples:  []core.AppCPUSample{{AppID: app, CPUPercent: 100, SampledAt: time.Unix(10, 0), SampleWindow: time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionLimitCPU {
		t.Fatalf("actions = %#v, want CPU limit", result.Actions)
	}
	if result.Actions[0].CPUPercent == nil || *result.Actions[0].CPUPercent != 25 {
		t.Fatalf("cpu percent = %#v, want 25", result.Actions[0].CPUPercent)
	}
	if result.Actions[0].RunFor != 2500*time.Millisecond || result.Actions[0].StopFor != 7500*time.Millisecond {
		t.Fatalf("duty cycle = run %s stop %s", result.Actions[0].RunFor, result.Actions[0].StopFor)
	}
	if result.Statuses[app.Key()] != core.AppStatusCPULimited {
		t.Fatalf("status = %q, want CPU limited", result.Statuses[app.Key()])
	}
}

func TestSchedulerEmitsCPULimitActionForForegroundApp(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	target := 0.01
	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}},
		AppSamples:  []core.AppCPUSample{{AppID: app, CPUPercent: 8, SampledAt: time.Unix(10, 0), SampleWindow: time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Frontmost:   app,
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionLimitCPU {
		t.Fatalf("actions = %#v, want foreground CPU limit", result.Actions)
	}
	if result.Actions[0].RunFor != 12500*time.Microsecond || result.Actions[0].StopFor != 9987500*time.Microsecond {
		t.Fatalf("duty cycle = run %s stop %s, want 12.5ms/9987.5ms", result.Actions[0].RunFor, result.Actions[0].StopFor)
	}
	if result.Statuses[app.Key()] != core.AppStatusCPULimited {
		t.Fatalf("status = %q, want CPU limited", result.Statuses[app.Key()])
	}
}

func TestSchedulerSkipsCPULimitBelowTarget(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	target := 25.0
	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}},
		AppSamples:  []core.AppCPUSample{{AppID: app, CPUPercent: 10, SampledAt: time.Unix(10, 0), SampleWindow: time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 0 {
		t.Fatalf("actions = %#v, want none", result.Actions)
	}
}

func TestSchedulerKeepsActiveCPULimitDutyCycleWhenObservedDrops(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	target := 25.0
	runtime := core.NewRuntimeState()
	runtime.AppStates[app.Key()] = core.AppControlRuntime{
		AppID:           app,
		Status:          core.AppStatusCPULimited,
		CPULimitTarget:  target,
		CPULimitRunFor:  2500 * time.Millisecond,
		CPULimitStopFor: 7500 * time.Millisecond,
	}

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}},
		AppSamples:  []core.AppCPUSample{{AppID: app, CPUPercent: 0, SampledAt: time.Unix(13, 0), SampleWindow: time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
		Runtime:     runtime,
		Now:         time.Unix(13, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionLimitCPU {
		t.Fatalf("actions = %#v, want continuing CPU limit", result.Actions)
	}
	if result.Actions[0].RunFor != 2500*time.Millisecond || result.Actions[0].StopFor != 7500*time.Millisecond {
		t.Fatalf("duty cycle = run %s stop %s", result.Actions[0].RunFor, result.Actions[0].StopFor)
	}
}

func TestSchedulerKeepsStricterActiveCPULimitDutyCycleWhenObservedRises(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	target := 1.0
	runtime := core.NewRuntimeState()
	runtime.AppStates[app.Key()] = core.AppControlRuntime{
		AppID:           app,
		Status:          core.AppStatusCPULimited,
		CPULimitTarget:  target,
		CPULimitRunFor:  100 * time.Millisecond,
		CPULimitStopFor: 9900 * time.Millisecond,
	}

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}},
		AppSamples:  []core.AppCPUSample{{AppID: app, CPUPercent: 4, SampledAt: time.Unix(13, 0), SampleWindow: time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
		Runtime:     runtime,
		Now:         time.Unix(13, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionLimitCPU {
		t.Fatalf("actions = %#v, want continuing CPU limit", result.Actions)
	}
	if result.Actions[0].RunFor != 100*time.Millisecond || result.Actions[0].StopFor != 9900*time.Millisecond {
		t.Fatalf("duty cycle = run %s stop %s, want previous stricter cycle", result.Actions[0].RunFor, result.Actions[0].StopFor)
	}
}

func TestSchedulerTightensActiveCPULimitDutyCycleWhenObservedRequiresIt(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	target := 25.0
	runtime := core.NewRuntimeState()
	runtime.AppStates[app.Key()] = core.AppControlRuntime{
		AppID:           app,
		Status:          core.AppStatusCPULimited,
		CPULimitTarget:  target,
		CPULimitRunFor:  5 * time.Second,
		CPULimitStopFor: 5 * time.Second,
	}

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}},
		AppSamples:  []core.AppCPUSample{{AppID: app, CPUPercent: 100, SampledAt: time.Unix(13, 0), SampleWindow: time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
		Runtime:     runtime,
		Now:         time.Unix(13, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionLimitCPU {
		t.Fatalf("actions = %#v, want CPU limit", result.Actions)
	}
	if result.Actions[0].RunFor != 2500*time.Millisecond || result.Actions[0].StopFor != 7500*time.Millisecond {
		t.Fatalf("duty cycle = run %s stop %s, want tighter cycle", result.Actions[0].RunFor, result.Actions[0].StopFor)
	}
}

func TestSchedulerRestoresBackgroundOnlyPriorityWhenForeground(t *testing.T) {
	app := core.AppID{BundleID: "com.example.worker", Name: "Worker"}
	group := schedulerGroup(app, 10)
	runtime := core.NewRuntimeState()
	runtime.RecordPriorityChange(group.Processes[0], app, 0, core.DefaultBackgroundNice, core.ControlReasonUserRule, time.Unix(5, 0))
	nice := core.DefaultBackgroundNice

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups: []core.AppGroup{group},
		Rules: []core.AppRule{{
			AppID:          app,
			Mode:           core.RuleModeLowerPriorityInBackground,
			BackgroundOnly: true,
			NiceValue:      &nice,
		}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     runtime,
		Frontmost:   app,
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionRestorePriority {
		t.Fatalf("actions = %#v, want restore priority", result.Actions)
	}
	if result.Statuses[app.Key()] != core.AppStatusRestoring {
		t.Fatalf("status = %q, want restoring", result.Statuses[app.Key()])
	}
}

func TestSchedulerAppliesAlwaysPriorityWhenForeground(t *testing.T) {
	app := core.AppID{BundleID: "com.example.worker", Name: "Worker"}
	group := schedulerGroup(app, 10)
	nice := core.DefaultBackgroundNice

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups: []core.AppGroup{group},
		Rules: []core.AppRule{{
			AppID:          app,
			Mode:           core.RuleModeLowerPriorityInBackground,
			BackgroundOnly: false,
			NiceValue:      &nice,
		}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Frontmost:   app,
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionSetPriority {
		t.Fatalf("actions = %#v, want set priority", result.Actions)
	}
	if result.Statuses[app.Key()] != core.AppStatusPriorityLowered {
		t.Fatalf("status = %q, want priority lowered", result.Statuses[app.Key()])
	}
}

func TestSchedulerKeepsAlwaysPriorityAppliedWhenForeground(t *testing.T) {
	app := core.AppID{BundleID: "com.example.worker", Name: "Worker"}
	group := schedulerGroup(app, 10)
	runtime := core.NewRuntimeState()
	runtime.RecordPriorityChange(group.Processes[0], app, 0, core.DefaultBackgroundNice, core.ControlReasonUserRule, time.Unix(5, 0))
	nice := core.DefaultBackgroundNice

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups: []core.AppGroup{group},
		Rules: []core.AppRule{{
			AppID:          app,
			Mode:           core.RuleModeLowerPriorityInBackground,
			BackgroundOnly: false,
			NiceValue:      &nice,
		}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     runtime,
		Frontmost:   app,
		Now:         time.Unix(10, 0),
	})

	if len(result.Actions) != 0 {
		t.Fatalf("actions = %#v, want priority left alone", result.Actions)
	}
	if result.Statuses[app.Key()] != core.AppStatusPriorityLowered {
		t.Fatalf("status = %q, want priority lowered", result.Statuses[app.Key()])
	}
}

func schedulerGroup(app core.AppID, pid int) core.AppGroup {
	return core.AppGroup{
		ID:              app,
		Kind:            core.AppKindUserApp,
		Controllability: core.ControllabilityNormal,
		Status:          core.AppStatusObserved,
		Processes: []core.ProcessRef{
			{ID: core.ProcessID{PID: pid, StartTime: time.Unix(int64(pid), 0)}, Name: app.DisplayName(), UID: 501},
		},
	}
}
