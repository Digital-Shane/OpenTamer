package policy

import (
	"math"
	"os"
	"runtime"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

const globalCPUCycle = 100 * time.Millisecond

type cpuDemandWindow struct {
	runFor, stopFor time.Duration
	window          time.Duration
	weightedCPU     float64
}

func (scheduler *Scheduler) demandSample(sample core.AppCPUSample, previous core.AppControlRuntime) core.AppCPUSample {
	key := sample.AppID.Key()
	cycle := previous.CPULimitRunFor + previous.CPULimitStopFor
	if previous.CPULimitRunFor <= 0 || previous.CPULimitStopFor <= 0 || cycle <= globalCPUCycle || sample.SampleWindow <= 0 || sample.SampleWindow >= 5*cycle {
		delete(scheduler.demandWindows, key)
		return sample
	}
	if scheduler.demandWindows == nil {
		scheduler.demandWindows = make(map[string]cpuDemandWindow)
	}
	window := scheduler.demandWindows[key]
	if window.runFor != previous.CPULimitRunFor || window.stopFor != previous.CPULimitStopFor {
		window = cpuDemandWindow{runFor: previous.CPULimitRunFor, stopFor: previous.CPULimitStopFor}
	}
	window.window += sample.SampleWindow
	window.weightedCPU += cleanCPU(sample.CPUPercent) * float64(sample.SampleWindow)
	if window.window >= 5*cycle {
		sample.CPUPercent = window.weightedCPU / float64(window.window)
		sample.SampleWindow = window.window
		delete(scheduler.demandWindows, key)
	} else {
		scheduler.demandWindows[key] = window
	}
	return sample
}

type GlobalCPUAllocation struct {
	Key     string
	Demand  float64
	Ceiling float64
}

// AllocateGlobalCPU uses per-core percentages; totalCPU and percent are machine-wide.
func AllocateGlobalCPU(percent float64, logicalCPUs int, totalCPU, observedEligible float64, apps []GlobalCPUAllocation) map[string]float64 {
	result := make(map[string]float64, len(apps))
	if logicalCPUs < 1 {
		logicalCPUs = 1
	}
	if !finiteCPU(percent) || percent < 1 || percent > 100 {
		return result
	}
	residual := math.Max(0, cleanCPU(totalCPU)*float64(logicalCPUs)-cleanCPU(observedEligible))
	available := math.Max(0, percent*float64(logicalCPUs)-residual)
	remaining := append([]GlobalCPUAllocation(nil), apps...)
	// Redistribute capacity released by individual ceilings proportionally to demand.
	for len(remaining) > 0 {
		demand := 0.0
		for _, app := range remaining {
			demand += cleanCPU(app.Demand)
		}
		if demand == 0 {
			break
		}
		ratio := math.Min(1, available/demand)
		next := make([]GlobalCPUAllocation, 0, len(remaining))
		for _, app := range remaining {
			share := cleanCPU(app.Demand) * ratio
			if app.Ceiling > 0 && app.Ceiling < share {
				result[app.Key] = app.Ceiling
				available = math.Max(0, available-app.Ceiling)
			} else {
				next = append(next, app)
			}
		}
		if len(next) == len(remaining) {
			for _, app := range next {
				result[app.Key] = cleanCPU(app.Demand) * ratio
			}
			break
		}
		remaining = next
	}
	return result
}

func finiteCPU(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func cleanCPU(value float64) float64 {
	if !finiteCPU(value) || value < 0 {
		return 0
	}
	return value
}

func globalCPUDemand(sample core.AppCPUSample, previous core.AppControlRuntime, capacity float64) float64 {
	demand := cleanCPU(sample.CPUPercent)
	if previous.CPULimitRunFor > 0 && previous.CPULimitStopFor > 0 {
		cycle := previous.CPULimitRunFor + previous.CPULimitStopFor
		duty := float64(previous.CPULimitRunFor) / float64(cycle)
		prior := cleanCPU(previous.CPULimitTarget / duty)
		if sample.SampleWindow >= 5*cycle {
			inferred := math.Min(capacity, demand/duty)
			// Ignore sampling jitter; adapt to sustained increases and decreases.
			if math.Abs(inferred-prior) > math.Max(1, prior*0.05) {
				demand = (prior + inferred) / 2
			} else {
				demand = prior
			}
		} else {
			// A short sample may fall entirely inside either phase.
			demand = math.Max(demand, prior)
		}
	}
	return math.Min(demand, capacity)
}

func globalCPUEnabled(p core.GlobalPreferences) bool {
	return p.ManagementEnabled && p.CPULimiterEnabled && p.GlobalCPUPercent != nil &&
		finiteCPU(*p.GlobalCPUPercent) && *p.GlobalCPUPercent >= 1 && *p.GlobalCPUPercent <= 100
}

func globalCPUGroupSafe(group core.AppGroup) bool {
	if group.Controllability != core.ControllabilityNormal || !core.SafetyDecisionFromGroup(group).CanPause || len(group.Processes) == 0 {
		return false
	}
	safety := core.NewSafetyPolicy(os.Getpid())
	for _, process := range group.Processes {
		if process.UID != os.Getuid() || process.UID == 0 || !safety.ClassifyProcess(process).CanPause {
			return false
		}
	}
	return true
}

func globalCPUGroupEligible(input SchedulerInput, result SchedulerResult, group core.AppGroup, now time.Time) bool {
	if !globalCPUGroupSafe(group) {
		return false
	}
	for _, process := range group.Processes {
		if input.Runtime.IsPausedByOpenTamer(process.ID) {
			return false
		}
	}
	rule, hasRule := matchingRule(group.ID, input.Rules)
	if !hasRule {
		return true
	}
	if rule.Mode == core.RuleModeObserveOnly {
		return false
	}
	state := result.Runtime.AppStates[group.ID.Key()]
	if rule.LaunchGrace > 0 && now.Sub(state.FirstSeenAt) < rule.LaunchGrace {
		return false
	}
	if !groupIsForeground(group, input.Frontmost) && rule.WaitBeforeApply > 0 && now.Sub(state.BackgroundSince) < rule.WaitBeforeApply {
		return false
	}
	if rule.Mode == core.RuleModePauseInBackground || rule.Mode == core.RuleModeQuitAfterIdle {
		return false
	}
	return state.Status != core.AppStatusBlockedByPolicy && state.Status != core.AppStatusBlockedBySafety
}

func (scheduler *Scheduler) applyGlobalCPU(input SchedulerInput, result *SchedulerResult, now time.Time) {
	if !globalCPUEnabled(input.Preferences) || input.Protections.AudioActive || !finiteCPU(input.TotalCPUPercent) || input.TotalCPUPercent < 0 || input.TotalCPUPercent > 100 {
		return
	}
	if _, blocked := policyBlocksNewControls(input.Policy); blocked {
		return
	}
	logicalCPUs := input.LogicalCPUCount
	if logicalCPUs < 1 {
		logicalCPUs = runtime.NumCPU()
	}
	samples := samplesByApp(input.AppSamples)
	apps := make([]GlobalCPUAllocation, 0, len(input.Groups))
	eligible := make(map[string]bool)
	observed := 0.0
	for _, group := range input.Groups {
		if !globalCPUGroupEligible(input, *result, group, now) {
			continue
		}
		key := group.ID.Key()
		sample, ok := samples[key]
		if !ok {
			continue
		}
		previous := input.Runtime.AppStates[key]
		demand := globalCPUDemand(scheduler.demandSample(sample, previous), previous, 100*float64(logicalCPUs))
		ceiling := 0.0
		if rule, ok := matchingRule(group.ID, input.Rules); ok && rule.Mode == core.RuleModeLimitCPUInBackground && rule.CPUPercent != nil && (!rule.BackgroundOnly || !groupIsForeground(group, input.Frontmost)) {
			ceiling = *rule.CPUPercent
		}
		apps = append(apps, GlobalCPUAllocation{Key: key, Demand: demand, Ceiling: ceiling})
		eligible[key] = true
		observed += cleanCPU(sample.CPUPercent)
	}
	allocations := AllocateGlobalCPU(*input.Preferences.GlobalCPUPercent, logicalCPUs, input.TotalCPUPercent, observed, apps)
	kept := result.Actions[:0]
	for _, action := range result.Actions {
		if action.Type == core.ControlActionLimitCPU && eligible[action.AppID.Key()] {
			continue
		}
		kept = append(kept, action)
	}
	result.Actions = kept
	demands := make(map[string]float64, len(apps))
	ceilings := make(map[string]float64, len(apps))
	for _, app := range apps {
		demands[app.Key] = app.Demand
		ceilings[app.Key] = app.Ceiling
	}
	for _, group := range input.Groups {
		key := group.ID.Key()
		if !eligible[key] {
			continue
		}
		target, demand := allocations[key], demands[key]
		state := result.Runtime.AppStates[key]
		clearCPULimitRuntime(&state)
		if state.Status == core.AppStatusCPULimited {
			state.Status = core.AppStatusEligible
			result.Statuses[key] = state.Status
		}
		if demand > 0 && target < demand {
			// Always leave a runnable slice even when protected load exhausts budget.
			runFor := max(time.Millisecond, time.Duration(float64(globalCPUCycle)*target/demand))
			stopFor := globalCPUCycle - runFor
			if ceiling := ceilings[key]; ceiling > 0 && demand*float64(runFor)/float64(globalCPUCycle) > ceiling {
				// Keep very low explicit limits even when they need a longer cycle.
				plan := DefaultCPULimitPlanner(true).Plan(CPULimitInput{ObservedCPU: demand, TargetCPU: ceiling, BrowserLike: input.Protections.IsBrowserLike(group.ID)})
				if plan.Active && float64(plan.RunFor)/float64(plan.RunFor+plan.StopFor) < float64(runFor)/float64(globalCPUCycle) {
					runFor, stopFor = plan.RunFor, plan.StopFor
				}
			}
			target = demand * float64(runFor) / float64(runFor+stopFor)
			result.Actions = append(result.Actions, cpuLimitAction(group, target, runFor, stopFor, "global CPU budget", now))
			state.CPULimitTarget, state.CPULimitRunFor, state.CPULimitStopFor = target, runFor, stopFor
			state.Status = core.AppStatusCPULimited
			result.Statuses[key] = state.Status
		}
		result.Runtime.AppStates[key] = state
	}
}

func clearInactiveCPULimits(result *SchedulerResult) {
	active := make(map[string]bool)
	for _, action := range result.Actions {
		if action.Type == core.ControlActionLimitCPU {
			active[action.AppID.Key()] = true
		}
	}
	for key, state := range result.Runtime.AppStates {
		if active[key] {
			continue
		}
		clearCPULimitRuntime(&state)
		if state.Status == core.AppStatusCPULimited {
			state.Status = result.Statuses[key]
			if state.Status == "" || state.Status == core.AppStatusCPULimited {
				state.Status = core.AppStatusEligible
			}
		}
		result.Runtime.AppStates[key] = state
	}
}
