package policy

import (
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type SchedulerInput struct {
	Groups      []core.AppGroup
	Rules       []core.AppRule
	AppSamples  []core.AppCPUSample
	Preferences core.GlobalPreferences
	Runtime     core.RuntimeState
	Frontmost   core.AppID
	Policy      core.SystemPolicyState
	Protections ProtectionState
	Now         time.Time
}

type SchedulerResult struct {
	Actions  []core.ControlAction
	Runtime  core.RuntimeState
	Statuses map[string]core.AppStatus
}

type Scheduler struct{}

func NewScheduler() *Scheduler {
	return &Scheduler{}
}

func (scheduler *Scheduler) Evaluate(input SchedulerInput) SchedulerResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	runtime := input.Runtime.Clone()
	runtime.EnsureMaps()
	result := SchedulerResult{
		Runtime:  runtime,
		Statuses: make(map[string]core.AppStatus),
	}

	if !input.Preferences.ManagementEnabled {
		result.Actions = append(result.Actions, restoreAllActions(runtime, now, core.ControlReasonGlobalDisabled)...)
		for _, group := range input.Groups {
			result.Statuses[group.ID.Key()] = core.AppStatusRestoring
		}
		result.Runtime = runtime
		return result
	}
	if restoreReason, shouldRestore := policyRestoresManagement(input.Policy); shouldRestore {
		result.Actions = append(result.Actions, restoreAllActions(runtime, now, restoreReason)...)
		for _, group := range input.Groups {
			result.Statuses[group.ID.Key()] = core.AppStatusRestoring
		}
		result.Runtime = runtime
		return result
	}

	rules := rulesByApp(input.Rules)
	samples := samplesByApp(input.AppSamples)
	for _, group := range input.Groups {
		key := group.ID.Key()
		appRuntime := runtime.AppStates[key]
		if appRuntime.FirstSeenAt.IsZero() {
			appRuntime.FirstSeenAt = now
		}
		appRuntime.AppID = group.ID
		appRuntime.LastEvaluatedAt = now

		rule, ok := matchingRule(group.ID, rules)
		foreground := !group.ID.IsEmpty() && group.ID.Matches(input.Frontmost)
		if foreground {
			appRuntime.BackgroundSince = time.Time{}
			if !ok || !ruleAppliesInForeground(rule) {
				result.Actions = append(result.Actions, scheduler.restoreForegroundActions(group, runtime, now, true)...)
				appRuntime.Status = statusForForeground(group, runtime)
				runtime.AppStates[key] = appRuntime
				result.Statuses[key] = appRuntime.Status
				continue
			}
			result.Actions = append(result.Actions, scheduler.restoreForegroundActions(group, runtime, now, shouldRestorePriorityInForeground(rule))...)
		} else if appRuntime.BackgroundSince.IsZero() {
			appRuntime.BackgroundSince = now
		}

		if !ok {
			result.Actions = append(result.Actions, scheduler.restoreGroupActions(group, runtime, now, core.ControlReasonUserRule, true)...)
			clearCPULimitRuntime(&appRuntime)
			appRuntime.Status = core.AppStatusObserved
			runtime.AppStates[key] = appRuntime
			result.Statuses[key] = appRuntime.Status
			continue
		}

		if rule.PreferEfficiencyCores {
			result.Actions = append(result.Actions, core.ControlAction{
				Type:      core.ControlActionUnsupported,
				AppID:     group.ID,
				Processes: group.ProcessIDs(),
				Reason:    core.ControlReasonUnsupportedRule,
				Message:   "efficiency-core preference is a deferred research feature",
				At:        now,
			})
		}

		if rule.Mode == core.RuleModeObserveOnly {
			result.Actions = append(result.Actions, scheduler.restoreGroupActions(group, runtime, now, core.ControlReasonUserRule, true)...)
			clearCPULimitRuntime(&appRuntime)
			appRuntime.Status = core.AppStatusObserved
			runtime.AppStates[key] = appRuntime
			result.Statuses[key] = appRuntime.Status
			continue
		}

		safety := core.SafetyDecisionFromGroup(group)
		if !safety.AllowsRule(rule.Mode) {
			appRuntime.Status = core.AppStatusBlockedBySafety
			result.Actions = append(result.Actions, core.ControlAction{
				Type:      core.ControlActionNoop,
				AppID:     group.ID,
				Processes: group.ProcessIDs(),
				Reason:    core.ControlReasonSafetyBlocked,
				Message:   string(safety.Reason),
				At:        now,
			})
			runtime.AppStates[key] = appRuntime
			result.Statuses[key] = appRuntime.Status
			continue
		}

		if reason, message, blocked := input.Protections.Blocks(group, rule); blocked {
			appRuntime.Status = core.AppStatusBlockedByPolicy
			result.Actions = append(result.Actions, core.ControlAction{
				Type:      core.ControlActionNoop,
				AppID:     group.ID,
				Processes: group.ProcessIDs(),
				Reason:    reason,
				Message:   message,
				At:        now,
			})
			runtime.AppStates[key] = appRuntime
			result.Statuses[key] = appRuntime.Status
			continue
		}

		if blockReason, blocked := policyBlocksNewControls(input.Policy); blocked {
			appRuntime.Status = core.AppStatusBlockedByPolicy
			result.Actions = append(result.Actions, core.ControlAction{
				Type:      core.ControlActionNoop,
				AppID:     group.ID,
				Processes: group.ProcessIDs(),
				Reason:    blockReason,
				At:        now,
			})
			runtime.AppStates[key] = appRuntime
			result.Statuses[key] = appRuntime.Status
			continue
		}

		if rule.LaunchGrace > 0 && now.Sub(appRuntime.FirstSeenAt) < rule.LaunchGrace {
			appRuntime.Status = core.AppStatusBlockedByPolicy
			runtime.AppStates[key] = appRuntime
			result.Statuses[key] = appRuntime.Status
			continue
		}

		if rule.WaitBeforeApply > 0 && now.Sub(appRuntime.BackgroundSince) < rule.WaitBeforeApply {
			appRuntime.Status = core.AppStatusWaiting
			runtime.AppStates[key] = appRuntime
			result.Statuses[key] = appRuntime.Status
			continue
		}

		switch rule.Mode {
		case core.RuleModePauseInBackground:
			result.Actions = append(result.Actions, scheduler.restoreGroupPriorityActions(group, runtime, now, core.ControlReasonUserRule)...)
			clearCPULimitRuntime(&appRuntime)
			if allPaused(group, runtime) {
				if rule.PeriodicWakeEvery > 0 &&
					(appRuntime.LastPeriodicWakeAt.IsZero() || now.Sub(appRuntime.LastPeriodicWakeAt) >= rule.PeriodicWakeEvery) {
					result.Actions = append(result.Actions, core.ControlAction{
						Type:      core.ControlActionWakeBriefly,
						AppID:     group.ID,
						Processes: group.ProcessIDs(),
						Reason:    core.ControlReasonPeriodicWake,
						At:        now,
					})
					appRuntime.LastPeriodicWakeAt = now
					appRuntime.Status = core.AppStatusTemporarilyAwake
				} else {
					appRuntime.Status = core.AppStatusPaused
				}
			} else {
				result.Actions = append(result.Actions, core.ControlAction{
					Type:      core.ControlActionPause,
					AppID:     group.ID,
					Processes: group.ProcessIDs(),
					Reason:    core.ControlReasonUserRule,
					At:        now,
				})
				if rule.HideWhenStopped {
					result.Actions = append(result.Actions, core.ControlAction{
						Type:      core.ControlActionHide,
						AppID:     group.ID,
						Processes: group.ProcessIDs(),
						Reason:    core.ControlReasonUserRule,
						Message:   "hide_when_stopped",
						At:        now,
					})
				}
				appRuntime.Status = core.AppStatusPaused
			}
		case core.RuleModeLowerPriorityInBackground:
			result.Actions = append(result.Actions, scheduler.restoreGroupPauses(group, runtime, now, core.ControlReasonUserRule)...)
			result.Actions = append(result.Actions, scheduler.restoreGroupHiddenActions(group, runtime, now, core.ControlReasonUserRule)...)
			clearCPULimitRuntime(&appRuntime)
			nice := core.DefaultBackgroundNice
			if rule.NiceValue != nil {
				nice = *rule.NiceValue
			}
			if !allPriorityChanged(group, runtime, nice) {
				result.Actions = append(result.Actions, core.ControlAction{
					Type:      core.ControlActionSetPriority,
					AppID:     group.ID,
					Processes: group.ProcessIDs(),
					Reason:    core.ControlReasonUserRule,
					NiceValue: &nice,
					At:        now,
				})
			}
			appRuntime.Status = core.AppStatusPriorityLowered
		case core.RuleModeLimitCPUInBackground:
			result.Actions = append(result.Actions, scheduler.restoreGroupActions(group, runtime, now, core.ControlReasonUserRule, true)...)
			target := 0.0
			if rule.CPUPercent != nil {
				target = *rule.CPUPercent
			}
			sample := samples[group.ID.Key()]
			plan := DefaultCPULimitPlanner(input.Preferences.CPULimiterEnabled).Plan(CPULimitInput{
				Group:       group,
				ObservedCPU: sample.CPUPercent,
				TargetCPU:   target,
				BrowserLike: input.Protections.IsBrowserLike(group.ID),
			})
			runFor, stopFor, message, active := cpuLimitDutyCycle(plan, appRuntime, target)
			if active {
				result.Actions = append(result.Actions, cpuLimitAction(group, target, runFor, stopFor, message, now))
				appRuntime.CPULimitTarget = target
				appRuntime.CPULimitRunFor = runFor
				appRuntime.CPULimitStopFor = stopFor
				appRuntime.Status = core.AppStatusCPULimited
			} else {
				appRuntime.CPULimitTarget = 0
				appRuntime.CPULimitRunFor = 0
				appRuntime.CPULimitStopFor = 0
				appRuntime.Status = core.AppStatusEligible
			}
		case core.RuleModeHideAfterIdle:
			result.Actions = append(result.Actions, scheduler.restoreGroupPauses(group, runtime, now, core.ControlReasonUserRule)...)
			result.Actions = append(result.Actions, scheduler.restoreGroupPriorityActions(group, runtime, now, core.ControlReasonUserRule)...)
			clearCPULimitRuntime(&appRuntime)
			result.Actions = append(result.Actions, core.ControlAction{
				Type:      core.ControlActionHide,
				AppID:     group.ID,
				Processes: group.ProcessIDs(),
				Reason:    core.ControlReasonUserRule,
				Message:   "hide_after_idle",
				At:        now,
			})
			appRuntime.Status = core.AppStatusEligible
		case core.RuleModeQuitAfterIdle:
			result.Actions = append(result.Actions, scheduler.restoreGroupActions(group, runtime, now, core.ControlReasonUserRule, true)...)
			clearCPULimitRuntime(&appRuntime)
			result.Actions = append(result.Actions, core.ControlAction{
				Type:      core.ControlActionQuit,
				AppID:     group.ID,
				Processes: group.ProcessIDs(),
				Reason:    core.ControlReasonUserRule,
				At:        now,
			})
			appRuntime.Status = core.AppStatusEligible
		default:
			result.Actions = append(result.Actions, scheduler.restoreGroupActions(group, runtime, now, core.ControlReasonUnsupportedRule, true)...)
			clearCPULimitRuntime(&appRuntime)
			result.Actions = append(result.Actions, core.ControlAction{
				Type:      core.ControlActionUnsupported,
				AppID:     group.ID,
				Processes: group.ProcessIDs(),
				Reason:    core.ControlReasonUnsupportedRule,
				At:        now,
			})
			appRuntime.Status = core.AppStatusUnsupported
		}

		runtime.AppStates[key] = appRuntime
		result.Statuses[key] = appRuntime.Status
	}

	result.Runtime = runtime
	return result
}

func cpuLimitAction(group core.AppGroup, target float64, runFor, stopFor time.Duration, message string, at time.Time) core.ControlAction {
	return core.ControlAction{
		Type:       core.ControlActionLimitCPU,
		AppID:      group.ID,
		Processes:  group.ProcessIDs(),
		Reason:     core.ControlReasonUserRule,
		CPUPercent: &target,
		RunFor:     runFor,
		StopFor:    stopFor,
		Message:    message,
		At:         at,
	}
}

func cpuLimitDutyCycle(plan CPULimitPlan, runtime core.AppControlRuntime, target float64) (time.Duration, time.Duration, string, bool) {
	previousActive := runtime.Status == core.AppStatusCPULimited &&
		runtime.CPULimitTarget == target &&
		runtime.CPULimitRunFor > 0 &&
		runtime.CPULimitStopFor > 0
	if plan.Active {
		if previousActive && runtime.CPULimitStopFor >= plan.StopFor {
			return runtime.CPULimitRunFor, runtime.CPULimitStopFor, "continuing previous CPU limit duty cycle", true
		}
		return plan.RunFor, plan.StopFor, plan.Explanation, true
	}
	if previousActive && plan.Reason == CPULimitReasonUnderLimit {
		return runtime.CPULimitRunFor, runtime.CPULimitStopFor, "continuing previous CPU limit duty cycle", true
	}
	return 0, 0, "", false
}

func samplesByApp(samples []core.AppCPUSample) map[string]core.AppCPUSample {
	byApp := make(map[string]core.AppCPUSample, len(samples))
	for _, sample := range samples {
		byApp[sample.AppID.Key()] = sample
	}
	return byApp
}

func rulesByApp(rules []core.AppRule) []core.AppRule {
	return append([]core.AppRule(nil), rules...)
}

func matchingRule(app core.AppID, rules []core.AppRule) (core.AppRule, bool) {
	for _, rule := range rules {
		if rule.Matches(app) {
			return rule, true
		}
	}
	return core.AppRule{}, false
}

func ruleAppliesInForeground(rule core.AppRule) bool {
	switch rule.Mode {
	case core.RuleModeLimitCPUInBackground:
		return true
	case core.RuleModeLowerPriorityInBackground:
		return !rule.BackgroundOnly
	default:
		return false
	}
}

func shouldRestorePriorityInForeground(rule core.AppRule) bool {
	return rule.Mode != core.RuleModeLowerPriorityInBackground || rule.BackgroundOnly
}

func policyRestoresManagement(policy core.SystemPolicyState) (core.ControlReason, bool) {
	if !policy.ManagementBlocked {
		return "", false
	}
	switch policy.BlockReason {
	case core.ControlReasonPowerPolicy, core.ControlReasonIdlePolicy:
		return policy.BlockReason, true
	default:
		return "", false
	}
}

func policyBlocksNewControls(policy core.SystemPolicyState) (core.ControlReason, bool) {
	if policy.InStartupGrace {
		return core.ControlReasonLaunchGrace, true
	}
	if policy.InWakeGrace {
		return core.ControlReasonWakeGrace, true
	}
	if policy.ManagementBlocked {
		if policy.BlockReason != "" {
			return policy.BlockReason, true
		}
		return core.ControlReasonPolicyBlocked, true
	}
	return "", false
}

func EvaluateSystemPolicy(preferences core.GlobalPreferences, observed core.SystemPolicyState) core.SystemPolicyState {
	state := observed
	if state.InStartupGrace {
		state.ManagementBlocked = true
		state.BlockReason = core.ControlReasonLaunchGrace
		return state
	}
	if state.InWakeGrace {
		state.ManagementBlocked = true
		state.BlockReason = core.ControlReasonWakeGrace
		return state
	}
	if preferences.DisableWhenACBatteryAbove != nil &&
		state.OnACPower &&
		state.BatteryPercent != nil &&
		*state.BatteryPercent >= *preferences.DisableWhenACBatteryAbove {
		state.ManagementBlocked = true
		state.BlockReason = core.ControlReasonPowerPolicy
		return state
	}
	if preferences.DisableWhenUserIdleLongerThan > 0 &&
		state.UserIdleFor >= preferences.DisableWhenUserIdleLongerThan {
		state.ManagementBlocked = true
		state.BlockReason = core.ControlReasonIdlePolicy
		return state
	}
	return state
}

func ApplyWakeGrace(preferences core.GlobalPreferences, observed core.SystemPolicyState, launchedAt, now time.Time) core.SystemPolicyState {
	state := observed
	if preferences.WakeGrace <= 0 {
		return state
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !launchedAt.IsZero() && !now.Before(launchedAt) && now.Sub(launchedAt) < preferences.WakeGrace {
		state.InStartupGrace = true
	}
	if !state.LastWakeAt.IsZero() && !now.Before(state.LastWakeAt) && now.Sub(state.LastWakeAt) < preferences.WakeGrace {
		state.InWakeGrace = true
	}
	return state
}

func restoreAllActions(runtime core.RuntimeState, now time.Time, reason core.ControlReason) []core.ControlAction {
	actions := make([]core.ControlAction, 0, len(runtime.PausedProcesses)+len(runtime.PriorityChanges)+len(runtime.HiddenApps))
	for _, paused := range runtime.PausedProcesses {
		actions = append(actions, core.ControlAction{
			Type:      core.ControlActionResume,
			AppID:     paused.AppID,
			Processes: []core.ProcessID{paused.Process.ID},
			Reason:    reason,
			At:        now,
		})
	}
	for _, changed := range runtime.PriorityChanges {
		actions = append(actions, core.ControlAction{
			Type:      core.ControlActionRestorePriority,
			AppID:     changed.AppID,
			Processes: []core.ProcessID{changed.Process.ID},
			Reason:    reason,
			At:        now,
		})
	}
	for _, hidden := range runtime.HiddenApps {
		processes := make([]core.ProcessID, 0, 1)
		if !hidden.Process.ID.IsEmpty() {
			processes = append(processes, hidden.Process.ID)
		}
		actions = append(actions, core.ControlAction{
			Type:      core.ControlActionActivate,
			AppID:     hidden.AppID,
			Processes: processes,
			Reason:    reason,
			At:        now,
		})
	}
	return actions
}

func (scheduler *Scheduler) restoreForegroundActions(group core.AppGroup, runtime core.RuntimeState, now time.Time, restorePriority bool) []core.ControlAction {
	actions := make([]core.ControlAction, 0)
	paused := make([]core.ProcessID, 0)
	priority := make([]core.ProcessID, 0)
	for _, process := range group.Processes {
		if runtime.IsPausedByOpenTamer(process.ID) {
			paused = append(paused, process.ID)
		}
		if restorePriority {
			if _, ok := runtime.PriorityChanges[process.ID.Key()]; ok {
				priority = append(priority, process.ID)
			}
		}
	}
	if len(paused) > 0 {
		actions = append(actions, core.ControlAction{
			Type:      core.ControlActionResume,
			AppID:     group.ID,
			Processes: paused,
			Reason:    core.ControlReasonForeground,
			At:        now,
		})
	}
	if len(priority) > 0 {
		actions = append(actions, core.ControlAction{
			Type:      core.ControlActionRestorePriority,
			AppID:     group.ID,
			Processes: priority,
			Reason:    core.ControlReasonForeground,
			At:        now,
		})
	}
	return actions
}

func (scheduler *Scheduler) restoreGroupActions(group core.AppGroup, runtime core.RuntimeState, now time.Time, reason core.ControlReason, restorePriority bool) []core.ControlAction {
	actions := scheduler.restoreGroupPauses(group, runtime, now, reason)
	if restorePriority {
		actions = append(actions, scheduler.restoreGroupPriorityActions(group, runtime, now, reason)...)
	}
	actions = append(actions, scheduler.restoreGroupHiddenActions(group, runtime, now, reason)...)
	return actions
}

func (scheduler *Scheduler) restoreGroupPriorityActions(group core.AppGroup, runtime core.RuntimeState, now time.Time, reason core.ControlReason) []core.ControlAction {
	priority := make([]core.ProcessID, 0)
	for _, process := range group.Processes {
		if _, ok := runtime.PriorityChanges[process.ID.Key()]; ok {
			priority = append(priority, process.ID)
		}
	}
	if len(priority) == 0 {
		return nil
	}
	return []core.ControlAction{{
		Type:      core.ControlActionRestorePriority,
		AppID:     group.ID,
		Processes: priority,
		Reason:    reason,
		At:        now,
	}}
}

func (scheduler *Scheduler) restoreGroupHiddenActions(group core.AppGroup, runtime core.RuntimeState, now time.Time, reason core.ControlReason) []core.ControlAction {
	hidden, ok := runtime.HiddenApps[group.ID.Key()]
	if ok {
		processes := make([]core.ProcessID, 0, 1)
		if !hidden.Process.ID.IsEmpty() {
			processes = append(processes, hidden.Process.ID)
		}
		return []core.ControlAction{{
			Type:      core.ControlActionActivate,
			AppID:     group.ID,
			Processes: processes,
			Reason:    reason,
			At:        now,
		}}
	}
	return nil
}

func (scheduler *Scheduler) restoreGroupPauses(group core.AppGroup, runtime core.RuntimeState, now time.Time, reason core.ControlReason) []core.ControlAction {
	paused := make([]core.ProcessID, 0)
	for _, process := range group.Processes {
		if runtime.IsPausedByOpenTamer(process.ID) {
			paused = append(paused, process.ID)
		}
	}
	if len(paused) == 0 {
		return nil
	}
	return []core.ControlAction{{
		Type:      core.ControlActionResume,
		AppID:     group.ID,
		Processes: paused,
		Reason:    reason,
		At:        now,
	}}
}

func clearCPULimitRuntime(runtime *core.AppControlRuntime) {
	runtime.CPULimitTarget = 0
	runtime.CPULimitRunFor = 0
	runtime.CPULimitStopFor = 0
}

func statusForForeground(group core.AppGroup, runtime core.RuntimeState) core.AppStatus {
	for _, process := range group.Processes {
		if runtime.IsPausedByOpenTamer(process.ID) {
			return core.AppStatusRestoring
		}
		if _, ok := runtime.PriorityChanges[process.ID.Key()]; ok {
			return core.AppStatusRestoring
		}
	}
	return core.AppStatusEligible
}

func allPaused(group core.AppGroup, runtime core.RuntimeState) bool {
	if len(group.Processes) == 0 {
		return true
	}
	for _, process := range group.Processes {
		if !runtime.IsPausedByOpenTamer(process.ID) {
			return false
		}
	}
	return true
}

func allPriorityChanged(group core.AppGroup, runtime core.RuntimeState, nice int) bool {
	if len(group.Processes) == 0 {
		return true
	}
	for _, process := range group.Processes {
		change, ok := runtime.PriorityChanges[process.ID.Key()]
		if !ok || change.TargetNice != nice {
			return false
		}
	}
	return true
}
