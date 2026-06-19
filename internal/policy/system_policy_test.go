package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestEvaluateSystemPolicyBlocksOnACAboveBatteryThreshold(t *testing.T) {
	threshold := 80.0
	battery := 95.0
	state := EvaluateSystemPolicy(
		core.GlobalPreferences{DisableWhenACBatteryAbove: &threshold},
		core.SystemPolicyState{OnACPower: true, BatteryPercent: &battery},
	)

	if !state.ManagementBlocked {
		t.Fatal("expected policy to block management")
	}
	if state.BlockReason != core.ControlReasonPowerPolicy {
		t.Fatalf("reason = %q", state.BlockReason)
	}
}

func TestEvaluateSystemPolicyBlocksWhenUserIdleTooLong(t *testing.T) {
	state := EvaluateSystemPolicy(
		core.GlobalPreferences{DisableWhenUserIdleLongerThan: 10 * time.Minute},
		core.SystemPolicyState{UserIdleFor: 15 * time.Minute},
	)

	if !state.ManagementBlocked {
		t.Fatal("expected idle policy to block management")
	}
	if state.BlockReason != core.ControlReasonIdlePolicy {
		t.Fatalf("reason = %q", state.BlockReason)
	}
}

func TestApplyWakeGraceCoversStartupWindow(t *testing.T) {
	launchedAt := time.Unix(10, 0)
	state := ApplyWakeGrace(
		core.GlobalPreferences{WakeGrace: 30 * time.Second},
		core.SystemPolicyState{},
		launchedAt,
		launchedAt.Add(10*time.Second),
	)

	if !state.InStartupGrace {
		t.Fatal("expected wake grace to cover app startup")
	}
	if state.InWakeGrace {
		t.Fatal("startup grace should not imply system wake grace")
	}
}

func TestApplyWakeGraceCoversRecentSystemWake(t *testing.T) {
	wokeAt := time.Unix(10, 0)
	state := ApplyWakeGrace(
		core.GlobalPreferences{WakeGrace: 30 * time.Second},
		core.SystemPolicyState{LastWakeAt: wokeAt},
		time.Time{},
		wokeAt.Add(10*time.Second),
	)

	if !state.InWakeGrace {
		t.Fatal("expected wake grace after recent system wake")
	}
}

func TestPolicyGuardRestoresExistingControlsLikeGlobalDisable(t *testing.T) {
	for _, reason := range []core.ControlReason{core.ControlReasonPowerPolicy, core.ControlReasonIdlePolicy} {
		t.Run(string(reason), func(t *testing.T) {
			app := core.AppID{BundleID: "com.example.app", Name: "Example"}
			group := schedulerGroup(app, 42)
			state := core.NewRuntimeState()
			state.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(1, 0))
			state.RecordPriorityChange(group.Processes[0], app, 0, core.DefaultBackgroundNice, core.ControlReasonUserRule, time.Unix(1, 0))

			result := NewScheduler().Evaluate(SchedulerInput{
				Groups:      []core.AppGroup{group},
				Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
				Preferences: core.GlobalPreferences{ManagementEnabled: true},
				Runtime:     state,
				Policy: core.SystemPolicyState{
					ManagementBlocked: true,
					BlockReason:       reason,
				},
				Now: time.Unix(2, 0),
			})

			if len(result.Actions) != 2 {
				t.Fatalf("actions = %#v, want resume and restore priority", result.Actions)
			}
			if result.Actions[0].Type != core.ControlActionResume || result.Actions[0].Reason != reason {
				t.Fatalf("first action = %#v, want %s resume", result.Actions[0], reason)
			}
			if result.Actions[1].Type != core.ControlActionRestorePriority || result.Actions[1].Reason != reason {
				t.Fatalf("second action = %#v, want %s priority restore", result.Actions[1], reason)
			}
			if result.Statuses[app.Key()] != core.AppStatusRestoring {
				t.Fatalf("status = %q, want restoring", result.Statuses[app.Key()])
			}
		})
	}
}

func TestStartupGraceBlocksNewControlsButAllowsForegroundRestore(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 42)
	state := core.NewRuntimeState()
	state.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(1, 0))

	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     state,
		Frontmost:   app,
		Policy:      core.SystemPolicyState{InStartupGrace: true},
		Now:         time.Unix(2, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionResume {
		t.Fatalf("actions = %#v, want foreground resume", result.Actions)
	}
}
