package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestProtectionStateBlocksAudioProtectedRule(t *testing.T) {
	app := core.AppID{BundleID: "com.example.player", Name: "Player"}
	group := schedulerGroup(app, 42)
	result := NewScheduler().Evaluate(SchedulerInput{
		Groups: []core.AppGroup{group},
		Rules: []core.AppRule{{
			AppID:        app,
			Mode:         core.RuleModeLowerPriorityInBackground,
			ProtectAudio: true,
		}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Protections: ProtectionState{AudioActive: true},
		Now:         time.Unix(1, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Reason != core.ControlReasonAudioProtection {
		t.Fatalf("actions = %#v, want audio protection noop", result.Actions)
	}
	if result.Statuses[app.Key()] != core.AppStatusBlockedByPolicy {
		t.Fatalf("status = %q", result.Statuses[app.Key()])
	}
}

func TestBrowserPauseBlockedUnlessExplicitlyAllowed(t *testing.T) {
	app := core.AppID{BundleID: "com.google.Chrome", Name: "Google Chrome"}
	group := schedulerGroup(app, 42)
	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(1, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Reason != core.ControlReasonBrowserProtection {
		t.Fatalf("actions = %#v, want browser protection noop", result.Actions)
	}
}
