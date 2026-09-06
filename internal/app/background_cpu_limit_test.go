package app

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

func TestBackgroundCPULimitResumesStoppedProcessesOnFocus(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	frontmost := core.AppID{BundleID: "com.example.worker", Path: "/Applications/Worker.app", Name: "Worker"}
	group := core.AppGroup{
		ID:              appID,
		Controllability: core.ControllabilityNormal,
		Processes: []core.ProcessRef{
			{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, ExecutablePath: "/Applications/Worker.app/Contents/MacOS/Worker"},
			{ID: core.ProcessID{PID: 43, StartTime: time.Unix(2, 0)}, ExecutablePath: "/Applications/Worker.app/Contents/MacOS/Worker"},
		},
	}
	signals := &limiterSignals{}
	limiter := NewCPULimiter(signals)
	t.Cleanup(limiter.StopAll)
	target := 0.01
	input := apppolicy.SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: appID, Mode: core.RuleModeLimitCPUInBackground, BackgroundOnly: true, CPUPercent: &target}},
		AppSamples:  []core.AppCPUSample{{AppID: appID, CPUPercent: 100}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true},
		Runtime:     core.NewRuntimeState(),
	}
	scheduler := apppolicy.NewScheduler()
	for cycle := range 3 {
		input.Frontmost = core.AppID{BundleID: "com.example.other"}
		background := scheduler.Evaluate(input)
		limiter.Update(cpuLimitRequests(input.Groups, background.Actions))
		if !signals.waitForStops((cycle+1)*len(group.Processes), time.Second) {
			t.Fatalf("cycle %d: background processes were not limited", cycle)
		}

		input.Runtime = background.Runtime
		input.Frontmost = frontmost
		foreground := scheduler.Evaluate(input)
		continuesBefore := signals.continueCount()
		entry := limiter.entries[appID.Key()]
		limiter.Update(cpuLimitRequests(input.Groups, foreground.Actions))
		if len(limiter.entries) != 0 {
			t.Fatal("foreground app still has an active CPU limiter")
		}
		if signals.continueCount() != continuesBefore+len(group.Processes) {
			t.Fatal("foreground transition did not resume every stopped process before returning")
		}
		select {
		case <-entry.done:
		default:
			t.Fatal("old worker can still signal foreground processes")
		}
		input.Runtime = foreground.Runtime
	}
}
