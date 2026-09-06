package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestSchedulerBackgroundCPULimitFollowsFocus(t *testing.T) {
	frontmost := core.AppID{BundleID: "com.example.app", Path: "/Applications/Example.app", Name: "Example"}
	for _, tc := range []struct {
		name  string
		appID core.AppID
		path  string
	}{
		{"bundle grouping", frontmost, "/Applications/Example.app/Contents/MacOS/Example"},
		{"name grouping", core.AppID{Name: "ExampleProcess"}, "/Applications/Example.app/Contents/MacOS/ExampleProcess"},
		{"helper name grouping", core.AppID{Name: "Example Helper"}, "/Applications/Example.app/Contents/Frameworks/Helper.app/Contents/MacOS/Helper"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := schedulerGroup(tc.appID, 10)
			group.Processes[0].ExecutablePath = tc.path
			target := 25.0
			input := SchedulerInput{
				Groups: []core.AppGroup{group},
				Rules: []core.AppRule{{
					AppID: tc.appID, Mode: core.RuleModeLimitCPUInBackground,
					BackgroundOnly: true, CPUPercent: &target,
				}},
				AppSamples:  []core.AppCPUSample{{AppID: tc.appID, CPUPercent: 100}},
				Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
				Runtime:     core.NewRuntimeState(),
				Frontmost:   core.AppID{BundleID: "com.example.other"},
				Now:         time.Unix(10, 0),
			}
			scheduler := NewScheduler()
			for _, step := range []struct {
				name      string
				frontmost core.AppID
				cpu       float64
				limited   bool
			}{
				{"background above target", input.Frontmost, 100, true},
				{"foreground releases limit", frontmost, 100, false},
				{"foreground stays unlimited", frontmost, 200, false},
				{"background below target", input.Frontmost, 10, false},
				{"background above target again", input.Frontmost, 100, true},
			} {
				input.Frontmost = step.frontmost
				input.AppSamples[0].CPUPercent = step.cpu
				input.Now = input.Now.Add(3 * time.Second)
				result := scheduler.Evaluate(input)
				input.Runtime = result.Runtime
				state := result.Runtime.AppStates[tc.appID.Key()]
				if step.limited {
					if len(result.Actions) != 1 || result.Actions[0].Type != core.ControlActionLimitCPU {
						t.Fatalf("%s: actions = %#v, want CPU limit", step.name, result.Actions)
					}
					if result.Statuses[tc.appID.Key()] != core.AppStatusCPULimited {
						t.Fatalf("%s: status = %q, want CPU limited", step.name, state.Status)
					}
				} else {
					if len(result.Actions) != 0 || result.Statuses[tc.appID.Key()] != core.AppStatusEligible {
						t.Fatalf("%s: actions = %#v, status = %q, want eligible without controls", step.name, result.Actions, state.Status)
					}
					if state.CPULimitTarget != 0 || state.CPULimitRunFor != 0 || state.CPULimitStopFor != 0 {
						t.Fatalf("%s: stale CPU duty cycle: %#v", step.name, state)
					}
				}
				if step.frontmost == frontmost && !state.BackgroundSince.IsZero() {
					t.Fatalf("%s: background timer was not reset", step.name)
				}
			}
		})
	}
}

func TestGroupIsForegroundMatchesProcessMetadataForNameGroups(t *testing.T) {
	frontmost := core.AppID{BundleID: "com.example.app", Path: "/Applications/Example.app", Name: "Example"}
	for _, tc := range []struct {
		name      string
		appID     core.AppID
		process   core.ProcessRef
		frontmost core.AppID
		want      bool
	}{
		{"bundle ID", core.AppID{Name: "Example"}, core.ProcessRef{BundleID: "COM.EXAMPLE.APP"}, frontmost, true},
		{"unrelated executable", core.AppID{Name: "Example"}, core.ProcessRef{ExecutablePath: "/usr/bin/Example"}, frontmost, false},
		{"similar bundle prefix", core.AppID{Name: "Example"}, core.ProcessRef{ExecutablePath: "/Applications/Example.app.other/Contents/MacOS/Example"}, frontmost, false},
		{"path outside bundle", core.AppID{Name: "Example"}, core.ProcessRef{ExecutablePath: "/Applications/Example.app/../Other.app/Contents/MacOS/Example"}, frontmost, false},
		{"different bundle same name", core.AppID{BundleID: "com.example.other", Name: "Example"}, core.ProcessRef{}, frontmost, false},
		{"no frontmost app", core.AppID{Name: "Example"}, core.ProcessRef{}, core.AppID{}, false},
		{"name-only identities", core.AppID{Name: "Example"}, core.ProcessRef{}, core.AppID{Name: "Example"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := core.AppGroup{ID: tc.appID, Processes: []core.ProcessRef{tc.process}}
			if got := groupIsForeground(group, tc.frontmost); got != tc.want {
				t.Fatalf("foreground = %v, want %v", got, tc.want)
			}
		})
	}
}
