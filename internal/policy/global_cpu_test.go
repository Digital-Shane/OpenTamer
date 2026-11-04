package policy

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestAllocateGlobalCPU(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		percent, total, observed float64
		cpus                     int
		apps                     []GlobalCPUAllocation
		want                     []float64
	}{
		{"multicore", 25, 100, 800, 8, []GlobalCPUAllocation{{"a", 400, 0}, {"b", 400, 0}}, []float64{100, 100}},
		{"protected residual", 50, 75, 400, 8, []GlobalCPUAllocation{{"a", 400, 0}}, []float64{200}},
		{"exhausted", 10, 75, 100, 8, []GlobalCPUAllocation{{"a", 100, 0}}, []float64{0}},
		{"redistribute individual ceiling", 50, 100, 800, 8, []GlobalCPUAllocation{{"a", 400, 50}, {"b", 400, 0}}, []float64{50, 350}},
		{"headroom", 100, 10, 80, 8, []GlobalCPUAllocation{{"a", 80, 0}}, []float64{80}},
		{"invalid", math.NaN(), 100, 800, 8, []GlobalCPUAllocation{{"a", 800, 0}}, []float64{0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AllocateGlobalCPU(tc.percent, tc.cpus, tc.total, tc.observed, tc.apps)
			for i, app := range tc.apps {
				if math.IsNaN(got[app.Key]) || math.IsInf(got[app.Key], 0) || math.Abs(got[app.Key]-tc.want[i]) > 0.001 {
					t.Fatalf("%s = %v, want %v", app.Key, got[app.Key], tc.want[i])
				}
			}
		})
	}
}

func globalTestInput() SchedulerInput {
	percent := 5.0
	group := schedulerGroup(core.AppID{Name: "Worker"}, 67890)
	group.Processes[0].UID = os.Getuid()
	group.Processes[0].ExecutablePath = "/Applications/Worker.app/Contents/MacOS/Worker"
	return SchedulerInput{
		Groups:      []core.AppGroup{group},
		AppSamples:  []core.AppCPUSample{{AppID: group.ID, CPUPercent: 100, SampleWindow: time.Second}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true, CPULimiterEnabled: true, GlobalCPUPercent: &percent},
		Runtime:     core.NewRuntimeState(), LogicalCPUCount: 8, TotalCPUPercent: 12.5, Now: time.Unix(100, 0),
	}
}

func globalLimitAction(result SchedulerResult) (core.ControlAction, bool) {
	for _, action := range result.Actions {
		if action.Type == core.ControlActionLimitCPU {
			return action, true
		}
	}
	return core.ControlAction{}, false
}

func TestGlobalCPUSustainedDemandChanges(t *testing.T) {
	input := globalTestInput()
	scheduler := NewScheduler()
	duty := 1.0
	for _, demand := range []float64{100, 800, 80, 10} {
		for i := 0; i < 30; i++ {
			observed := demand * duty
			input.AppSamples[0].CPUPercent = observed
			input.TotalCPUPercent = observed / 8
			result := scheduler.Evaluate(input)
			input.Runtime = result.Runtime
			input.Now = input.Now.Add(time.Second)
			if action, ok := globalLimitAction(result); ok {
				if action.RunFor+action.StopFor != 100*time.Millisecond {
					t.Fatal("global cycle is not 100ms")
				}
				duty = float64(action.RunFor) / float64(action.RunFor+action.StopFor)
			} else {
				duty = 1
			}
		}
		want := math.Min(demand, 40)
		if got := demand * duty; math.Abs(got-want) > 3 {
			t.Fatalf("demand %v: delivered %v, want %v", demand, got, want)
		}
		if demand == 10 && input.Runtime.AppStates[input.Groups[0].ID.Key()].Status == core.AppStatusCPULimited {
			t.Fatal("released budget retained limited status")
		}
	}
}

func TestGlobalCPUShortSamplesDoNotOscillate(t *testing.T) {
	input := globalTestInput()
	scheduler := NewScheduler()
	first := scheduler.Evaluate(input)
	input.Runtime = first.Runtime
	input.AppSamples[0].SampleWindow = 10 * time.Millisecond
	for i := 0; i < 20; i++ {
		input.AppSamples[0].CPUPercent = float64(i%2) * 100
		input.TotalCPUPercent = input.AppSamples[0].CPUPercent / 8
		result := scheduler.Evaluate(input)
		action, ok := globalLimitAction(result)
		if !ok || action.RunFor != 40*time.Millisecond {
			t.Fatalf("short sample changed allocation: %#v", result.Actions)
		}
		input.Runtime = result.Runtime
	}
}

func TestGlobalCPUExemptionsAndRelease(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*SchedulerInput)
	}{
		{"disabled", func(i *SchedulerInput) { i.Preferences.GlobalCPUPercent = nil }},
		{"management disabled", func(i *SchedulerInput) { i.Preferences.ManagementEnabled = false }},
		{"limiter disabled", func(i *SchedulerInput) { i.Preferences.CPULimiterEnabled = false }},
		{"audio", func(i *SchedulerInput) { i.Protections.AudioActive = true }},
		{"wake grace", func(i *SchedulerInput) { i.Policy.InWakeGrace = true }},
		{"startup grace", func(i *SchedulerInput) { i.Policy.InStartupGrace = true }},
		{"power restore", func(i *SchedulerInput) {
			i.Policy.ManagementBlocked = true
			i.Policy.BlockReason = core.ControlReasonPowerPolicy
		}},
		{"idle restore", func(i *SchedulerInput) {
			i.Policy.ManagementBlocked = true
			i.Policy.BlockReason = core.ControlReasonIdlePolicy
		}},
		{"root", func(i *SchedulerInput) { i.Groups[0].Processes[0].UID = 0 }},
		{"other user", func(i *SchedulerInput) { i.Groups[0].Processes[0].UID = os.Getuid() + 1 }},
		{"unknown", func(i *SchedulerInput) { i.Groups[0].Processes[0].ExecutablePath = "" }},
		{"self", func(i *SchedulerInput) { i.Groups[0].Processes[0].ID.PID = os.Getpid() }},
		{"system", func(i *SchedulerInput) { i.Groups[0].Processes[0].Name = "WindowServer" }},
		{"slow only", func(i *SchedulerInput) { i.Groups[0].Controllability = core.ControllabilitySlowOnly }},
		{"observe", func(i *SchedulerInput) {
			i.Rules = []core.AppRule{{AppID: i.Groups[0].ID, Mode: core.RuleModeObserveOnly}}
		}},
		{"launch grace", func(i *SchedulerInput) {
			i.Rules = []core.AppRule{{AppID: i.Groups[0].ID, Mode: core.RuleModeLimitCPUInBackground, LaunchGrace: time.Hour}}
		}},
		{"wait", func(i *SchedulerInput) {
			i.Rules = []core.AppRule{{AppID: i.Groups[0].ID, Mode: core.RuleModeLimitCPUInBackground, WaitBeforeApply: time.Hour}}
		}},
		{"owned pause", func(i *SchedulerInput) {
			i.Runtime.RecordPause(i.Groups[0].Processes[0], i.Groups[0].ID, core.ControlReasonUserRule, i.Now)
			i.Rules = []core.AppRule{{AppID: i.Groups[0].ID, Mode: core.RuleModePauseInBackground}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := globalTestInput()
			scheduler := NewScheduler()
			input.Runtime = scheduler.Evaluate(input).Runtime
			tc.change(&input)
			result := scheduler.Evaluate(input)
			if _, ok := globalLimitAction(result); ok {
				t.Fatalf("exempt app was duty-cycled: %#v", result.Actions)
			}
		})
	}
}

func TestGlobalCPUIndividualLimitUsesStricterBudget(t *testing.T) {
	for _, target := range []float64{20, 80} {
		input := globalTestInput()
		input.Rules = []core.AppRule{{AppID: input.Groups[0].ID, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}}
		result := NewScheduler().Evaluate(input)
		if len(result.Actions) != 1 {
			t.Fatalf("overlapping controls: %#v", result.Actions)
		}
		action, ok := globalLimitAction(result)
		if !ok || math.Abs(*action.CPUPercent-math.Min(target, 40)) > 0.01 {
			t.Fatalf("incorrect stricter allocation: %#v", action)
		}
		input.Runtime = result.Runtime
		input.Preferences.GlobalCPUPercent = nil
		result = NewScheduler().Evaluate(input)
		action, ok = globalLimitAction(result)
		if !ok || *action.CPUPercent != target {
			t.Fatalf("individual rule was not restored: %#v", result.Actions)
		}
		if len(input.Rules) != 1 || *input.Rules[0].CPUPercent != target {
			t.Fatal("rules changed")
		}
	}
}

func TestGlobalCPUMinimumSlicePreservesIndividualCeiling(t *testing.T) {
	input := globalTestInput()
	*input.Preferences.GlobalCPUPercent = 100
	input.LogicalCPUCount = 12
	input.TotalCPUPercent = 100
	input.AppSamples[0].CPUPercent = 1200
	target := 1.0
	input.Rules = []core.AppRule{{AppID: input.Groups[0].ID, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}}
	result := NewScheduler().Evaluate(input)
	action, ok := globalLimitAction(result)
	if !ok {
		t.Fatal("individual CPU limit was lost")
	}
	effective := 1200 * float64(action.RunFor) / float64(action.RunFor+action.StopFor)
	if math.IsNaN(effective) || math.IsInf(effective, 0) || effective > target+0.0001 {
		t.Fatalf("individual ceiling weakened: effective %v, target %v", effective, target)
	}
}

func TestGlobalCPULongCycleInfersGrowthFromThreeSecondSamples(t *testing.T) {
	input := globalTestInput()
	*input.Preferences.GlobalCPUPercent = 100
	input.LogicalCPUCount = 12
	input.TotalCPUPercent = 100
	input.AppSamples[0].CPUPercent = 200
	input.AppSamples[0].SampleWindow = 3 * time.Second
	target := 1.0
	input.Rules = []core.AppRule{{AppID: input.Groups[0].ID, Mode: core.RuleModeLimitCPUInBackground, CPUPercent: &target}}
	scheduler := NewScheduler()
	result := scheduler.Evaluate(input)
	action, ok := globalLimitAction(result)
	if !ok {
		t.Fatal("initial individual ceiling lost")
	}
	duty := float64(action.RunFor) / float64(action.RunFor+action.StopFor)
	for i := 0; i < 300; i++ {
		input.Runtime = result.Runtime
		input.AppSamples[0].CPUPercent = 1200 * duty
		input.Now = input.Now.Add(3 * time.Second)
		result = scheduler.Evaluate(input)
		action, ok = globalLimitAction(result)
		if !ok {
			t.Fatal("ceiling lost during sustained growth")
		}
		duty = float64(action.RunFor) / float64(action.RunFor+action.StopFor)
	}
	effective := 1200 * duty
	if math.IsNaN(effective) || math.IsInf(effective, 0) || math.Abs(effective-target) > 0.06 {
		t.Fatalf("long-cycle demand did not adapt: delivered %v, target %v", effective, target)
	}
}

func TestGlobalCPUSuspensionClearsDutyBeforeReenable(t *testing.T) {
	for _, mode := range []string{"management", "audio", "telemetry", "wake", "global"} {
		t.Run(mode, func(t *testing.T) {
			input := globalTestInput()
			scheduler := NewScheduler()
			input.Runtime = scheduler.Evaluate(input).Runtime
			switch mode {
			case "management":
				input.Preferences.ManagementEnabled = false
			case "audio":
				input.Protections.AudioActive = true
			case "telemetry":
				input.TotalCPUPercent = math.NaN()
			case "wake":
				input.Policy.InWakeGrace = true
			case "global":
				input.Preferences.GlobalCPUPercent = nil
			}
			result := scheduler.Evaluate(input)
			if _, ok := globalLimitAction(result); ok {
				t.Fatal("global limiter survived suspension")
			}
			state := result.Runtime.AppStates[input.Groups[0].ID.Key()]
			if state.CPULimitRunFor != 0 || state.CPULimitStopFor != 0 || state.CPULimitTarget != 0 {
				t.Fatalf("stale duty after suspension: %#v", state)
			}
			fresh := globalTestInput()
			fresh.Runtime = result.Runtime
			fresh.AppSamples[0].CPUPercent = 20
			fresh.TotalCPUPercent = 2.5
			if result = scheduler.Evaluate(fresh); len(result.Actions) != 0 {
				t.Fatalf("obsolete duty interpreted unthrottled demand: %#v", result.Actions)
			}
		})
	}
}
