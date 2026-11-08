package app

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/observe"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
	"github.com/Digital-Shane/open-tamer/internal/ui"
)

const runtimeSaveInterval = 30 * time.Second

type MenuBarUpdater interface {
	UpdateMenuBarState(ui.MenuBarViewState) error
}

type Controller struct {
	store        config.Store
	updater      MenuBarUpdater
	applications ApplicationObserver
	processes    ProcessEnumerator
	metrics      MetricsSampler
	audio        AudioActivityObserver
	system       core.SystemPolicyObserver
	notifier     Notifier
	scheduler    *apppolicy.Scheduler
	cpu          *observe.CPUUsageCalculator
	systemCPU    *observe.SystemCPUCalculator
	history      *observe.CPUHistory
	highCPU      *HighCPUDetector
	groupCtl     *AppGroupController
	limiter      *CPULimiter

	mu                 sync.Mutex
	refreshMu          sync.Mutex
	stopped            bool
	shutdownRequested  bool
	controlError       string
	cfg                config.Config
	runtime            core.RuntimeState
	stats              core.StatsDocument
	lastGroups         []core.AppGroup
	lastAppSamples     []core.AppCPUSample
	lastStatuses       map[string]core.AppStatus
	lastTotalCPU       float64
	systemHighCPUSince time.Time
	lastRuntimeSaveAt  time.Time
	lastStatsSaveAt    time.Time
	launchedAt         time.Time
}

func NewController(store config.Store, updater MenuBarUpdater, adapters Adapters) (*Controller, error) {
	if err := adapters.Validate(); err != nil {
		return nil, err
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		log.Printf("OpenTamer config load warning: %v", err)
	}
	runtimeState, runtimeErr := store.LoadRuntimeState()
	if runtimeErr != nil {
		log.Printf("OpenTamer runtime-state load warning: %v", runtimeErr)
	}
	stats, statsErr := store.LoadStats()
	if statsErr != nil {
		log.Printf("OpenTamer stats load warning: %v", statsErr)
	}

	var processValidator ProcessGenerationValidator
	if lookup, ok := adapters.Processes.(ProcessGenerationLookup); ok {
		processValidator = ProcessGenerationLookupValidator{Lookup: lookup}
	}
	limiterValidator := processValidator
	if lookup, ok := adapters.Processes.(ProcessIdentityLookup); ok && processValidator != nil {
		limiterValidator = automaticCPUValidator{ProcessGenerationValidator: processValidator, lookup: lookup}
	}
	now := time.Now()
	return &Controller{
		store:        store,
		updater:      updater,
		applications: adapters.Applications,
		processes:    adapters.Processes,
		metrics:      adapters.Metrics,
		audio:        adapters.Audio,
		system:       adapters.SystemPolicy,
		notifier:     adapters.Notifier,
		scheduler:    apppolicy.NewScheduler(),
		cpu:          observe.NewCPUUsageCalculator(),
		systemCPU:    &observe.SystemCPUCalculator{},
		history:      observe.NewCPUHistory(core.MaxCPUGraphWindow, 0),
		highCPU:      NewHighCPUDetector(),
		groupCtl: &AppGroupController{
			Signals:                  adapters.Signals,
			Priority:                 adapters.Priority,
			Lifecycle:                adapters.Lifecycle,
			ProcessValidator:         processValidator,
			AllowPriorityImprovement: true,
		},
		limiter:           NewCPULimiter(adapters.Signals, limiterValidator),
		cfg:               cfg,
		runtime:           runtimeState,
		stats:             stats,
		lastStatuses:      make(map[string]core.AppStatus),
		lastRuntimeSaveAt: now,
		lastStatsSaveAt:   now,
		launchedAt:        now,
	}, nil
}

func (controller *Controller) Start(ctx context.Context) {
	controller.Refresh()

	go func() {
		ticker := time.NewTicker(controller.refreshInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				controller.Refresh()
			}
		}
	}()
}

func (controller *Controller) InitialState() ui.MenuBarViewState {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	return controller.buildMenuBarStateLocked(time.Now(), "Starting")
}

func (controller *Controller) saveConfigLocked() {
	if err := controller.store.SaveConfig(controller.cfg); err != nil {
		log.Printf("OpenTamer config save warning: %v", err)
	}
}

func (controller *Controller) Shutdown(reason core.ControlReason) bool {
	controller.refreshMu.Lock()
	defer controller.refreshMu.Unlock()
	controller.mu.Lock()
	if controller.stopped {
		controller.mu.Unlock()
		return true
	}
	controller.shutdownRequested = true
	controller.stopped = controller.shutdownLocked(reason)
	state := controller.buildMenuBarStateLocked(time.Now(), "")
	stopped := controller.stopped
	controller.mu.Unlock()
	if controller.updater != nil {
		if err := controller.updater.UpdateMenuBarState(state); err != nil {
			log.Printf("OpenTamer UI update warning: %v", err)
		}
	}
	return stopped
}

func (controller *Controller) shutdownLocked(reason core.ControlReason) bool {
	var failures []ControlFailure
	if controller.limiter != nil {
		failures = controller.limiter.StopAll()
	}
	actions := apppolicy.NewScheduler().Evaluate(apppolicy.SchedulerInput{
		Groups:      controller.lastGroups,
		Preferences: core.GlobalPreferences{ManagementEnabled: false},
		Runtime:     controller.runtime,
		Now:         time.Now(),
	}).Actions
	runtimeState, actionFailures := controller.executeActions(controller.lastGroups, actions, controller.runtime)
	failures = append(failures, actionFailures...)
	for _, action := range actions {
		runtimeState.AppendAction(action, 100)
	}
	controller.runtime = runtimeState
	for _, failure := range failures {
		log.Printf("OpenTamer shutdown action failure: %s", failure.Error())
	}
	controller.saveRuntimeLocked()
	controller.saveStatsLocked()
	controller.controlError = recoveryStatus(failures)
	// Other reversible changes remain persisted; stopped processes must not be abandoned.
	return len(runtimeState.PausedProcesses) == 0 && (controller.limiter == nil || len(controller.limiter.PendingRecovery()) == 0)
}

func recoveryStatus(failures []ControlFailure) string {
	for _, failure := range failures {
		if failure.Action == core.ControlActionResume {
			return "Recovery pending: " + failure.Error()
		}
	}
	if len(failures) > 0 {
		return "Control failed: " + failures[0].Error()
	}
	return ""
}

func (controller *Controller) saveRuntimeLocked() {
	if err := controller.store.SaveRuntimeState(controller.runtime); err != nil {
		log.Printf("OpenTamer runtime save warning: %v", err)
		return
	}
	controller.lastRuntimeSaveAt = time.Now()
}

func (controller *Controller) saveStatsLocked() {
	if err := controller.store.SaveStats(controller.stats); err != nil {
		log.Printf("OpenTamer stats save warning: %v", err)
		return
	}
	controller.lastStatsSaveAt = time.Now()
}

func (controller *Controller) saveRuntimeIfDueLocked(now time.Time, immediate bool, periodic bool) {
	if immediate || (periodic && shouldSaveAt(now, controller.lastRuntimeSaveAt, runtimeSaveInterval)) {
		controller.saveRuntimeLocked()
	}
}

func (controller *Controller) saveStatsIfDueLocked(now time.Time, immediate bool) {
	if immediate || shouldSaveAt(now, controller.lastStatsSaveAt, runtimeSaveInterval) {
		controller.saveStatsLocked()
	}
}

func shouldSaveAt(now, last time.Time, interval time.Duration) bool {
	return last.IsZero() || now.Sub(last) >= interval
}
