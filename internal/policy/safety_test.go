package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestSchedulerCannotPauseBlockedProcessEvenWithExplicitRule(t *testing.T) {
	app := core.AppID{BundleID: "com.apple.WindowServer", Name: "WindowServer"}
	group := core.AppGroup{
		ID:              app,
		Kind:            core.AppKindEssential,
		Controllability: core.ControllabilityBlocked,
		SafetyReason:    core.SafetyReasonEssentialSystem,
		Processes: []core.ProcessRef{
			{ID: core.ProcessID{PID: 88}, UID: 0, Name: "WindowServer"},
		},
	}

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
	if result.Actions[0].Message != string(core.SafetyReasonEssentialSystem) {
		t.Fatalf("message = %q", result.Actions[0].Message)
	}
}
