package app

import (
	"log"
	"os"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/observe"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

func (controller *Controller) Refresh() {
	controller.refreshMu.Lock()
	defer controller.refreshMu.Unlock()

	started := time.Now()

	controller.mu.Lock()
	cfg := controller.cfg
	runtimeState := controller.runtime
	stats := controller.stats
	lastTotalCPU := controller.lastTotalCPU
	controller.mu.Unlock()

	var hints []core.AppProcessHint
	if !cfg.Preferences.AggregateByName {
		var appErr error
		hints, appErr = controller.applications.AppProcessHints()
		if appErr != nil {
			log.Printf("OpenTamer running apps warning: %v", appErr)
		}
	}
	var frontmost core.AppID
	if needsFrontmostApp(cfg.Rules) {
		var frontErr error
		frontmost, frontErr = controller.applications.FrontmostAppID()
		if frontErr != nil {
			log.Printf("OpenTamer frontmost app warning: %v", frontErr)
		}
	}
	processes, processErr := controller.processes.Processes()
	if processErr != nil {
		log.Printf("OpenTamer process enumeration warning: %v", processErr)
	}

	grouper := observe.NewProcessGrouper()
	grouper.Safety = core.NewSafetyPolicy(os.Getpid())
	grouper.AggregateByName = cfg.Preferences.AggregateByName
	groups := grouper.Group(processes, hints)
	for i := range groups {
		groups[i] = core.ApplySafetyDecision(groups[i], grouper.Safety.ClassifyGroup(groups[i]))
	}

	metrics, metricsErr := controller.metrics.SampleProcesses(flattenProcesses(groups))
	if metricsErr != nil {
		log.Printf("OpenTamer process metrics warning: %v", metricsErr)
	}
	processUsage := controller.cpu.Update(metrics)
	appSamples := observe.AggregateAppCPU(groups, processUsage)

	systemSample, systemErr := controller.metrics.SampleSystemCPU()
	totalCPU := lastTotalCPU
	if systemErr == nil {
		if usage := controller.systemCPU.Update(systemSample); usage != nil {
			totalCPU = usage.TotalPercent
		}
	} else {
		log.Printf("OpenTamer system CPU warning: %v", systemErr)
	}

	audioActive := false
	if needsAudioProtection(cfg.Rules) {
		var audioErr error
		audioActive, audioErr = controller.audio.AudioActive()
		if audioErr != nil {
			log.Printf("OpenTamer audio warning: %v", audioErr)
		}
	}
	policyNow := time.Now()
	policy := core.SystemPolicyState{}
	if needsSystemPolicy(cfg) {
		var policyErr error
		policy, policyErr = controller.system.Snapshot()
		if policyErr != nil {
			log.Printf("OpenTamer system policy warning: %v", policyErr)
		}
	}
	policy = apppolicy.ApplyWakeGrace(cfg.Preferences, policy, controller.launchedAt, policyNow)
	policy = apppolicy.EvaluateSystemPolicy(cfg.Preferences, policy)

	protections := apppolicy.ProtectionState{
		AudioActive:      audioActive,
		BrowserLikeByApp: browserMap(groups),
	}

	result := controller.scheduler.Evaluate(apppolicy.SchedulerInput{
		Groups:      groups,
		Rules:       cfg.Rules,
		AppSamples:  appSamples,
		Preferences: cfg.Preferences,
		Runtime:     runtimeState,
		Frontmost:   frontmost,
		Policy:      policy,
		Protections: protections,
		Now:         policyNow,
	})

	runtimeState, failures := controller.executeActions(groups, result.Actions, result.Runtime)
	if controller.limiter != nil {
		controller.limiter.Update(cpuLimitRequests(groups, result.Actions))
	}
	for _, failure := range failures {
		log.Printf("OpenTamer action failure: %s", failure.Error())
	}

	statsDirty := false
	for _, action := range result.Actions {
		if action.Type == core.ControlActionPause {
			stats.RecordAutomaticPause(action.AppID)
			statsDirty = true
		}
		runtimeState.AppendAction(action, 100)
	}

	notices := controller.highCPU.Update(
		HighCPUConfigFromPreferences(cfg.Preferences),
		groups,
		appSamples,
		result.Statuses,
		time.Now(),
	)
	for _, notice := range notices {
		if err := controller.notifier.NotifyHighCPU(notice); err != nil {
			log.Printf("OpenTamer notification warning: %v", err)
		}
	}

	controller.mu.Lock()
	controller.runtime = runtimeState
	controller.stats = stats
	controller.lastGroups = groups
	controller.lastAppSamples = appSamples
	controller.lastStatuses = result.Statuses
	controller.lastTotalCPU = totalCPU
	if controller.history != nil {
		controller.history.AddBatch(appSamples)
	}
	state := controller.buildMenuBarStateLocked(time.Now(), "")
	controller.saveRuntimeIfDueLocked(started, shouldSaveRuntimeImmediately(result.Actions), hasControlRules(cfg.Rules))
	if statsDirty {
		controller.saveStatsIfDueLocked(started, true)
	}
	controller.mu.Unlock()

	if controller.updater != nil {
		if err := controller.updater.UpdateMenuBarState(state); err != nil {
			log.Printf("OpenTamer UI update warning: %v", err)
		}
	}
}

func (controller *Controller) refreshInterval() time.Duration {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	interval := controller.cfg.Preferences.StatsInterval
	if interval < 3*time.Second {
		return 3 * time.Second
	}
	return interval
}

func flattenProcesses(groups []core.AppGroup) []core.ProcessRef {
	processes := make([]core.ProcessRef, 0)
	seen := make(map[string]bool)
	for _, group := range groups {
		for _, process := range group.Processes {
			key := process.ID.Key()
			if seen[key] {
				continue
			}
			seen[key] = true
			processes = append(processes, process)
		}
	}
	return processes
}

func browserMap(groups []core.AppGroup) map[string]bool {
	result := make(map[string]bool)
	for _, group := range groups {
		if apppolicy.IsKnownBrowser(group.ID) {
			result[group.ID.Key()] = true
		}
	}
	return result
}

func needsFrontmostApp(rules []core.AppRule) bool {
	for _, rule := range rules {
		switch rule.Mode {
		case core.RuleModePauseInBackground, core.RuleModeHideAfterIdle, core.RuleModeQuitAfterIdle:
			return true
		case core.RuleModeLowerPriorityInBackground:
			if rule.BackgroundOnly {
				return true
			}
		}
	}
	return false
}

func needsAudioProtection(rules []core.AppRule) bool {
	for _, rule := range rules {
		if rule.Mode != core.RuleModeObserveOnly && rule.ProtectAudio {
			return true
		}
	}
	return false
}

func needsSystemPolicy(cfg config.Config) bool {
	if !hasControlRules(cfg.Rules) {
		return false
	}
	preferences := cfg.Preferences
	return preferences.DisableWhenACBatteryAbove != nil ||
		preferences.DisableWhenUserIdleLongerThan > 0 ||
		preferences.WakeGrace > 0
}

func hasControlRules(rules []core.AppRule) bool {
	for _, rule := range rules {
		if rule.Mode != core.RuleModeObserveOnly {
			return true
		}
	}
	return false
}

func shouldSaveRuntimeImmediately(actions []core.ControlAction) bool {
	for _, action := range actions {
		switch action.Type {
		case core.ControlActionPause,
			core.ControlActionResume,
			core.ControlActionSetPriority,
			core.ControlActionRestorePriority,
			core.ControlActionWakeBriefly,
			core.ControlActionHide,
			core.ControlActionActivate,
			core.ControlActionQuit:
			return true
		}
	}
	return false
}
