package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestSchedulerEmitsUnsupportedForEfficiencyCorePreference(t *testing.T) {
	app := core.AppID{Name: "Worker"}
	group := schedulerGroup(app, 42)
	result := NewScheduler().Evaluate(SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModeObserveOnly, PreferEfficiencyCores: true}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     core.NewRuntimeState(),
		Now:         time.Unix(1, 0),
	})

	if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionUnsupported {
		t.Fatalf("actions = %#v, want unsupported", result.Actions)
	}
}
