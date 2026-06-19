package app

import (
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/observe"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
	"github.com/Digital-Shane/open-tamer/internal/ui"
)

func TestRefreshIntervalMinimum(t *testing.T) {
	controller := &Controller{cfg: config.DefaultConfig()}
	controller.cfg.Preferences.StatsInterval = time.Second
	if got := controller.refreshInterval(); got != 3*time.Second {
		t.Fatalf("interval = %s", got)
	}
}

func TestPreferenceMenuCommandsUpdateAndSaveEveryExposedSetting(t *testing.T) {
	type checkFunc func(*testing.T, core.GlobalPreferences)

	durationCommand := func(field ui.PreferenceField, value time.Duration) string {
		return "pref-duration|" + string(field) + "|" + strconv.FormatInt(int64(value), 10)
	}

	floatCommand := func(field ui.PreferenceField, value string) string {
		return "pref-float|" + string(field) + "|" + value
	}

	boolCommand := func(field ui.PreferenceField, value bool) string {
		return "pref-bool|" + string(field) + "|" + strconv.FormatBool(value)
	}

	stringCommand := func(field ui.PreferenceField, value string) string {
		return "pref-string|" + string(field) + "|" + value
	}

	expect := func(check checkFunc) checkFunc {
		return func(t *testing.T, preferences core.GlobalPreferences) {
			t.Helper()
			check(t, preferences)
		}
	}

	cases := []struct {
		name    string
		command string
		prepare func(*Controller)
		check   checkFunc
	}{
		{
			name:    "show menu icon",
			command: boolCommand(ui.PreferenceShowMenuBarIcon, false),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.ShowMenuBarIcon {
					t.Fatal("show menu icon should be disabled")
				}
			}),
		},
		{
			name:    "aggregate by name",
			command: boolCommand(ui.PreferenceAggregateByName, false),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.AggregateByName {
					t.Fatal("aggregate by name should be disabled")
				}
			}),
		},
		{
			name:    "management enabled",
			command: boolCommand(ui.PreferenceManagementEnabled, false),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.ManagementEnabled {
					t.Fatal("management should be disabled")
				}
			}),
		},
		{
			name:    "CPU limiter enabled",
			command: boolCommand(ui.PreferenceCPULimiterEnabled, true),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if !preferences.CPULimiterEnabled {
					t.Fatal("CPU limiter should be enabled")
				}
			}),
		},
		{
			name:    "wake grace",
			command: durationCommand(ui.PreferenceWakeGrace, time.Minute),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.WakeGrace != time.Minute {
					t.Fatalf("wake grace = %s, want 1m", preferences.WakeGrace)
				}
			}),
		},
		{
			name:    "stats interval",
			command: durationCommand(ui.PreferenceStatsInterval, 5*time.Second),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.StatsInterval != 5*time.Second {
					t.Fatalf("stats interval = %s, want 5s", preferences.StatsInterval)
				}
			}),
		},
		{
			name:    "averaging window",
			command: durationCommand(ui.PreferenceAveragingWindow, 2*time.Minute),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.AveragingWindow != 2*time.Minute {
					t.Fatalf("averaging window = %s, want 2m", preferences.AveragingWindow)
				}
			}),
		},
		{
			name:    "CPU graph window",
			command: durationCommand(ui.PreferenceCPUGraphWindow, 10*time.Minute),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.CPUGraphWindow != 10*time.Minute {
					t.Fatalf("CPU graph window = %s, want 10m", preferences.CPUGraphWindow)
				}
			}),
		},
		{
			name:    "CPU display mode",
			command: stringCommand(ui.PreferenceCPUDisplayMode, core.CPUDisplayModeSystemNormalized),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.CPUDisplayMode != core.CPUDisplayModeSystemNormalized {
					t.Fatalf("CPU display mode = %q, want %q", preferences.CPUDisplayMode, core.CPUDisplayModeSystemNormalized)
				}
			}),
		},
		{
			name:    "high CPU detection enabled",
			command: boolCommand(ui.PreferenceHighCPUDetectionEnabled, false),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.HighCPUDetectionEnabled {
					t.Fatal("high CPU detection should be disabled")
				}
			}),
		},
		{
			name:    "high CPU threshold",
			command: floatCommand(ui.PreferenceHighCPUThreshold, "90"),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.HighCPUThreshold != 90 {
					t.Fatalf("high CPU threshold = %v, want 90", preferences.HighCPUThreshold)
				}
			}),
		},
		{
			name:    "high CPU duration",
			command: durationCommand(ui.PreferenceHighCPUDuration, time.Minute),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.HighCPUDuration != time.Minute {
					t.Fatalf("high CPU duration = %s, want 1m", preferences.HighCPUDuration)
				}
			}),
		},
		{
			name:    "high CPU cooldown",
			command: durationCommand(ui.PreferenceHighCPUCooldown, 30*time.Minute),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.HighCPUCooldown != 30*time.Minute {
					t.Fatalf("high CPU cooldown = %s, want 30m", preferences.HighCPUCooldown)
				}
			}),
		},
		{
			name:    "disable on AC battery above",
			command: floatCommand(ui.PreferenceDisableWhenACBatteryAbove, "75"),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.DisableWhenACBatteryAbove == nil || *preferences.DisableWhenACBatteryAbove != 75 {
					t.Fatalf("AC battery threshold = %#v, want 75", preferences.DisableWhenACBatteryAbove)
				}
			}),
		},
		{
			name:    "clear disable on AC battery above",
			command: "pref-clear|" + string(ui.PreferenceDisableWhenACBatteryAbove),
			prepare: func(controller *Controller) {
				threshold := 75.0
				controller.cfg.Preferences.DisableWhenACBatteryAbove = &threshold
			},
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.DisableWhenACBatteryAbove != nil {
					t.Fatalf("AC battery threshold = %#v, want nil", preferences.DisableWhenACBatteryAbove)
				}
			}),
		},
		{
			name:    "disable when user idle longer than",
			command: durationCommand(ui.PreferenceDisableWhenUserIdleLongerThan, 15*time.Minute),
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				if preferences.DisableWhenUserIdleLongerThan != 15*time.Minute {
					t.Fatalf("idle guard = %s, want 15m", preferences.DisableWhenUserIdleLongerThan)
				}
			}),
		},
		{
			name:    "reset defaults",
			command: commandResetDefaults,
			prepare: func(controller *Controller) {
				controller.cfg.Preferences.ManagementEnabled = false
				controller.cfg.Preferences.ShowMenuBarIcon = false
				controller.cfg.Preferences.CPULimiterEnabled = true
				controller.cfg.Preferences.StatsInterval = time.Minute
			},
			check: expect(func(t *testing.T, preferences core.GlobalPreferences) {
				defaults := config.DefaultConfig().Preferences
				if preferences != defaults {
					t.Fatalf("preferences = %#v, want defaults %#v", preferences, defaults)
				}
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := config.NewStore(t.TempDir())
			controller := &Controller{cfg: config.DefaultConfig(), store: store}
			if tc.prepare != nil {
				tc.prepare(controller)
			}

			controller.handleMenuCommandLocked(tc.command)
			tc.check(t, controller.cfg.Preferences)

			loaded, err := store.LoadConfig()
			if err != nil {
				t.Fatalf("load saved config: %v", err)
			}
			tc.check(t, loaded.Preferences)
		})
	}
}

func TestRefreshSensorGatesMatchRuleNeeds(t *testing.T) {
	observe := core.AppRule{Mode: core.RuleModeObserveOnly}
	priorityAlways := core.AppRule{Mode: core.RuleModeLowerPriorityInBackground, BackgroundOnly: false}
	priorityBackground := core.AppRule{Mode: core.RuleModeLowerPriorityInBackground, BackgroundOnly: true}
	pause := core.AppRule{Mode: core.RuleModePauseInBackground}

	if needsFrontmostApp([]core.AppRule{observe, priorityAlways}) {
		t.Fatal("observe-only and always-priority rules should not require frontmost app sampling")
	}
	if !needsFrontmostApp([]core.AppRule{priorityBackground}) {
		t.Fatal("background-priority rules should require frontmost app sampling")
	}
	if !needsFrontmostApp([]core.AppRule{pause}) {
		t.Fatal("pause-in-background rules should require frontmost app sampling")
	}
}

func TestPolicyAndPersistenceGatesSkipIdleObservationWork(t *testing.T) {
	cfg := config.DefaultConfig()
	threshold := 80.0
	cfg.Preferences.DisableWhenACBatteryAbove = &threshold
	cfg.Rules = []core.AppRule{{Mode: core.RuleModeObserveOnly}}
	if needsSystemPolicy(cfg) {
		t.Fatal("observe-only tracking should not require system policy sampling")
	}

	cfg.Rules = []core.AppRule{{Mode: core.RuleModePauseInBackground}}
	if !needsSystemPolicy(cfg) {
		t.Fatal("control rules should sample configured system policy")
	}

	if shouldSaveRuntimeImmediately([]core.ControlAction{{Type: core.ControlActionNoop}}) {
		t.Fatal("noop actions should not force an immediate runtime save")
	}
	if shouldSaveRuntimeImmediately([]core.ControlAction{{Type: core.ControlActionLimitCPU}}) {
		t.Fatal("CPU limit planning actions should use periodic runtime saves")
	}
	if !shouldSaveRuntimeImmediately([]core.ControlAction{{Type: core.ControlActionPause}}) {
		t.Fatal("process-control actions should force an immediate runtime save")
	}
}

func TestBuildMenuBarStateIncludesLiveRows(t *testing.T) {
	appID := core.AppID{Name: "Editor"}
	logicalCPUCount := runtime.NumCPU()
	controller := &Controller{
		cfg:          config.DefaultConfig(),
		stats:        core.NewStatsDocument(),
		lastTotalCPU: 22,
		lastStatuses: map[string]core.AppStatus{appID.Key(): core.AppStatusObserved},
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Editor"}},
		}},
		lastAppSamples: []core.AppCPUSample{{AppID: appID, CPUPercent: 12, SampledAt: time.Unix(1, 0)}},
	}

	state := controller.buildMenuBarStateLocked(time.Unix(2, 0), "")
	if len(state.TopProcesses) != 2 {
		t.Fatalf("top rows = %d, want app plus system bucket", len(state.TopProcesses))
	}
	if len(state.AllProcesses) != 1 {
		t.Fatalf("all rows = %d", len(state.AllProcesses))
	}
	if state.TopProcesses[0].AppKey == "" {
		t.Fatal("expected app key")
	}
	if state.TopProcesses[0].SystemCPUPercent != 12/float64(logicalCPUCount) {
		t.Fatalf("top row system CPU = %v, want normalized current CPU", state.TopProcesses[0].SystemCPUPercent)
	}
	if state.TotalCPU != 22 {
		t.Fatalf("total CPU = %v", state.TotalCPU)
	}
	if !state.ShowMenuBarIcon {
		t.Fatal("expected menu bar icon enabled")
	}
	if state.Preferences.StatsInterval != controller.cfg.Preferences.StatsInterval {
		t.Fatalf("state preferences stats interval = %s, want %s", state.Preferences.StatsInterval, controller.cfg.Preferences.StatsInterval)
	}
}

func TestBuildMenuBarStateScalesGraphForCPUDisplayMode(t *testing.T) {
	appID := core.AppID{Name: "Renderer"}
	logicalCPUCount := runtime.NumCPU()
	controller := &Controller{
		cfg:          config.DefaultConfig(),
		stats:        core.NewStatsDocument(),
		lastTotalCPU: 25,
		lastStatuses: map[string]core.AppStatus{appID.Key(): core.AppStatusObserved},
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Renderer"}},
		}},
		lastAppSamples: []core.AppCPUSample{{AppID: appID, CPUPercent: 80, SampledAt: time.Unix(1, 0)}},
	}

	perCore := controller.buildMenuBarStateLocked(time.Unix(2, 0), "")
	if len(perCore.CPUGraph.Lines) != 1 || len(perCore.CPUGraph.Lines[0].Points) != 1 {
		t.Fatalf("per-core graph = %#v, want one line with one point", perCore.CPUGraph)
	}
	if perCore.CPUGraph.Lines[0].Points[0].AppCPU != 80 {
		t.Fatalf("per-core graph CPU = %v, want raw 80", perCore.CPUGraph.Lines[0].Points[0].AppCPU)
	}

	controller.cfg.Preferences.CPUDisplayMode = core.CPUDisplayModeSystemNormalized
	systemNormalized := controller.buildMenuBarStateLocked(time.Unix(3, 0), "")
	if len(systemNormalized.CPUGraph.Lines) != 1 || len(systemNormalized.CPUGraph.Lines[0].Points) != 1 {
		t.Fatalf("system-normalized graph = %#v, want one line with one point", systemNormalized.CPUGraph)
	}
	want := 80 / float64(logicalCPUCount)
	if systemNormalized.CPUGraph.Lines[0].Points[0].AppCPU != want {
		t.Fatalf("system-normalized graph CPU = %v, want %v", systemNormalized.CPUGraph.Lines[0].Points[0].AppCPU, want)
	}
}

func TestBuildMenuBarStateWaitsForHighCPUDurationBeforeAlert(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Preferences.HighCPUThreshold = 70
	cfg.Preferences.HighCPUDuration = 30 * time.Second
	controller := &Controller{
		cfg:          cfg,
		stats:        core.NewStatsDocument(),
		lastTotalCPU: 80,
	}

	first := controller.buildMenuBarStateLocked(time.Unix(10, 0), "")
	if first.AlertLevel != core.AlertLevelNormal {
		t.Fatalf("first alert = %q, want normal", first.AlertLevel)
	}

	early := controller.buildMenuBarStateLocked(time.Unix(39, 0), "")
	if early.AlertLevel != core.AlertLevelNormal {
		t.Fatalf("early alert = %q, want normal", early.AlertLevel)
	}

	afterDuration := controller.buildMenuBarStateLocked(time.Unix(40, 0), "")
	if afterDuration.AlertLevel != core.AlertLevelHigh {
		t.Fatalf("alert after duration = %q, want high", afterDuration.AlertLevel)
	}
}

func TestBuildMenuBarStateResetsHighCPUAlertTimerBelowThreshold(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Preferences.HighCPUThreshold = 70
	cfg.Preferences.HighCPUDuration = 30 * time.Second
	controller := &Controller{
		cfg:          cfg,
		stats:        core.NewStatsDocument(),
		lastTotalCPU: 80,
	}

	controller.buildMenuBarStateLocked(time.Unix(10, 0), "")
	if alert := controller.buildMenuBarStateLocked(time.Unix(40, 0), "").AlertLevel; alert != core.AlertLevelHigh {
		t.Fatalf("alert after duration = %q, want high", alert)
	}

	controller.lastTotalCPU = 20
	if alert := controller.buildMenuBarStateLocked(time.Unix(41, 0), "").AlertLevel; alert != core.AlertLevelNormal {
		t.Fatalf("alert after reset = %q, want normal", alert)
	}

	controller.lastTotalCPU = 80
	if alert := controller.buildMenuBarStateLocked(time.Unix(42, 0), "").AlertLevel; alert != core.AlertLevelNormal {
		t.Fatalf("alert after new spike = %q, want normal while duration restarts", alert)
	}
}

func TestBuildMenuBarStateAddsSystemOtherBucketForCurrentTopRows(t *testing.T) {
	appID := core.AppID{Name: "Editor"}
	logicalCPUCount := runtime.NumCPU()
	controller := &Controller{
		cfg:          config.DefaultConfig(),
		stats:        core.NewStatsDocument(),
		lastTotalCPU: 22,
		lastStatuses: map[string]core.AppStatus{appID.Key(): core.AppStatusObserved},
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Editor"}},
		}},
		lastAppSamples: []core.AppCPUSample{{AppID: appID, CPUPercent: 16, SampledAt: time.Unix(1, 0)}},
	}

	state := controller.buildMenuBarStateLocked(time.Unix(2, 0), "")
	if len(state.TopProcesses) != 2 {
		t.Fatalf("top rows = %#v, want app row plus system bucket", state.TopProcesses)
	}
	bucket := state.TopProcesses[1]
	if !bucket.SystemBucket || bucket.Name != "System / Other" {
		t.Fatalf("bucket = %#v, want system bucket", bucket)
	}
	wantAppSystemCPU := 16 / float64(logicalCPUCount)
	wantBucketSystemCPU := 22 - wantAppSystemCPU
	if bucket.SystemCPUPercent != wantBucketSystemCPU {
		t.Fatalf("bucket system CPU = %v, want %v", bucket.SystemCPUPercent, wantBucketSystemCPU)
	}
	if bucket.CanPause || bucket.CanSlow {
		t.Fatalf("bucket capabilities = pause %v slow %v, want both disabled", bucket.CanPause, bucket.CanSlow)
	}
}

func TestBuildMenuBarStateIncludesBlockedRowsInVisibleProcessLists(t *testing.T) {
	selfID := core.AppID{Name: "OpenTamer"}
	editorID := core.AppID{Name: "Editor"}
	controller := &Controller{
		cfg:   config.DefaultConfig(),
		stats: core.NewStatsDocument(),
		lastStatuses: map[string]core.AppStatus{
			selfID.Key():   core.AppStatusBlockedBySafety,
			editorID.Key(): core.AppStatusObserved,
		},
		lastGroups: []core.AppGroup{
			{
				ID:              selfID,
				Controllability: core.ControllabilityBlocked,
				SafetyReason:    core.SafetyReasonOpenTamerSelf,
				Status:          core.AppStatusBlockedBySafety,
				Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 10}, Name: "OpenTamer"}},
			},
			{
				ID:              editorID,
				Controllability: core.ControllabilityNormal,
				Status:          core.AppStatusObserved,
				Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Editor"}},
			},
		},
		lastAppSamples: []core.AppCPUSample{
			{AppID: selfID, CPUPercent: 40, SampledAt: time.Unix(1, 0)},
			{AppID: editorID, CPUPercent: 12, SampledAt: time.Unix(1, 0)},
		},
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   selfID,
		Mode:    core.RuleModeObserveOnly,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar, core.RuleTrackInManagedApps},
	}}

	state := controller.buildMenuBarStateLocked(time.Unix(2, 0), "")
	var foundAll bool
	for _, row := range state.AllProcesses {
		if row.AppID != selfID {
			continue
		}
		foundAll = true
		if row.BlockerReason != string(core.SafetyReasonOpenTamerSelf) {
			t.Fatalf("self blocker = %q, want %q", row.BlockerReason, core.SafetyReasonOpenTamerSelf)
		}
		if row.CanPause || row.CanSlow {
			t.Fatalf("self capabilities = pause %v slow %v, want both disabled", row.CanPause, row.CanSlow)
		}
		if !row.TrackedInMenuBar || !row.TrackedInManagedApps {
			t.Fatalf("self tracking flags = menu bar %v tracked apps %v, want both", row.TrackedInMenuBar, row.TrackedInManagedApps)
		}
	}
	if !foundAll {
		t.Fatalf("All Processes omitted blocked self row: %#v", state.AllProcesses)
	}
	if len(state.MenuBarApps) != 1 || state.MenuBarApps[0].AppID != selfID {
		t.Fatalf("menu bar rows = %#v, want blocked self row", state.MenuBarApps)
	}
	if len(state.TrackedApps) != 1 || state.TrackedApps[0].AppID != selfID {
		t.Fatalf("tracked rows = %#v, want blocked self row", state.TrackedApps)
	}
	if len(state.TopProcesses) == 0 || state.TopProcesses[0].AppID != selfID {
		t.Fatalf("Top Processes first row = %#v, want blocked self row ranked by CPU", state.TopProcesses)
	}
	var foundTop bool
	for _, row := range state.TopProcesses {
		if row.AppID != selfID {
			continue
		}
		foundTop = true
		if row.BlockerReason != string(core.SafetyReasonOpenTamerSelf) {
			t.Fatalf("top self blocker = %q, want %q", row.BlockerReason, core.SafetyReasonOpenTamerSelf)
		}
		if row.CanPause || row.CanSlow {
			t.Fatalf("top self capabilities = pause %v slow %v, want both disabled", row.CanPause, row.CanSlow)
		}
	}
	if !foundTop {
		t.Fatalf("Top Processes omitted blocked self row: %#v", state.TopProcesses)
	}
}

func TestBuildMenuBarStateGraphsEveryTopProcess(t *testing.T) {
	apps := make([]core.AppID, topProcessesLimit)
	groups := make([]core.AppGroup, 0, topProcessesLimit)
	samples := make([]core.AppCPUSample, 0, topProcessesLimit)
	statuses := make(map[string]core.AppStatus, topProcessesLimit)
	for i := range apps {
		apps[i] = core.AppID{Name: "App " + strconv.Itoa(i+1)}
		groups = append(groups, core.AppGroup{
			ID:              apps[i],
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 100 + i}, Name: apps[i].Name}},
		})
		samples = append(samples, core.AppCPUSample{
			AppID:      apps[i],
			CPUPercent: float64(topProcessesLimit - i),
			SampledAt:  time.Unix(1, 0),
		})
		statuses[apps[i].Key()] = core.AppStatusObserved
	}

	controller := &Controller{
		cfg:            config.DefaultConfig(),
		stats:          core.NewStatsDocument(),
		lastGroups:     groups,
		lastStatuses:   statuses,
		lastAppSamples: samples,
	}

	state := controller.buildMenuBarStateLocked(time.Unix(2, 0), "")
	if len(state.TopProcesses) != topProcessesLimit {
		t.Fatalf("top rows = %d, want %d", len(state.TopProcesses), topProcessesLimit)
	}
	if len(state.CPUGraph.Lines) != len(state.TopProcesses) {
		t.Fatalf("graph lines = %d, want one per top row %d", len(state.CPUGraph.Lines), len(state.TopProcesses))
	}
}

func TestBuildMenuBarStateIncludesMenuBarRows(t *testing.T) {
	appID := core.AppID{Name: "Editor"}
	controller := &Controller{
		cfg:          config.DefaultConfig(),
		stats:        core.NewStatsDocument(),
		lastStatuses: map[string]core.AppStatus{appID.Key(): core.AppStatusObserved},
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Editor"}},
		}},
		lastAppSamples: []core.AppCPUSample{{AppID: appID, CPUPercent: 12, SampledAt: time.Unix(1, 0)}},
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModeObserveOnly,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar},
	}}

	state := controller.buildMenuBarStateLocked(time.Unix(2, 0), "")
	if len(state.MenuBarApps) != 1 {
		t.Fatalf("menu bar rows = %d, want 1", len(state.MenuBarApps))
	}
	if state.MenuBarApps[0].AppID != appID || state.MenuBarApps[0].CPUPercent != 12 {
		t.Fatalf("menu bar row = %#v", state.MenuBarApps[0])
	}
	if len(state.TrackedApps) != 0 {
		t.Fatalf("tracked rows = %d, want 0 for menu bar-only tracking", len(state.TrackedApps))
	}
	if len(state.ManagedApps) != 0 {
		t.Fatalf("managed rows = %d, want 0 for menu bar-only tracking", len(state.ManagedApps))
	}
}

func TestBuildMenuBarStateSharesRollingAverageWithTrackedAndManagedRows(t *testing.T) {
	trackedID := core.AppID{Name: "Tracked"}
	managedID := core.AppID{Name: "Managed"}
	history := observe.NewCPUHistory(time.Minute, 100)
	history.AddBatch([]core.AppCPUSample{
		{
			AppID:        trackedID,
			CPUSeconds:   3,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 6 * time.Second,
		},
		{
			AppID:        managedID,
			CPUSeconds:   1.5,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 6 * time.Second,
		},
	})

	controller := &Controller{
		cfg:     config.DefaultConfig(),
		stats:   core.NewStatsDocument(),
		history: history,
		lastStatuses: map[string]core.AppStatus{
			trackedID.Key(): core.AppStatusObserved,
			managedID.Key(): core.AppStatusObserved,
		},
		lastGroups: testGroups(trackedID, managedID),
		lastAppSamples: []core.AppCPUSample{
			{AppID: trackedID, CPUPercent: 80, SampledAt: time.Unix(6, 0)},
			{AppID: managedID, CPUPercent: 40, SampledAt: time.Unix(6, 0)},
		},
	}
	controller.cfg.Rules = []core.AppRule{
		{
			AppID: trackedID,
			Mode:  core.RuleModeObserveOnly,
			TrackIn: []core.RuleTrackingLocation{
				core.RuleTrackInMenuBar,
				core.RuleTrackInManagedApps,
			},
		},
		{
			AppID:   managedID,
			Mode:    core.RuleModePauseInBackground,
			TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps},
		},
	}

	state := controller.buildMenuBarStateLocked(time.Unix(6, 0), "")
	topByApp := map[string]ui.ProcessRow{}
	for _, row := range state.TopProcesses {
		topByApp[row.AppID.Key()] = row
	}
	trackedTop := topByApp[trackedID.Key()]
	managedTop := topByApp[managedID.Key()]
	if trackedTop.AverageCPUPercent == 0 || managedTop.AverageCPUPercent == 0 {
		t.Fatalf("top rows missing rolling averages: %#v", state.TopProcesses)
	}
	if len(state.MenuBarApps) != 1 || state.MenuBarApps[0].AppID != trackedID {
		t.Fatalf("menu bar rows = %#v, want tracked app", state.MenuBarApps)
	}
	if len(state.TrackedApps) != 1 || state.TrackedApps[0].AppID != trackedID {
		t.Fatalf("tracked rows = %#v, want tracked app", state.TrackedApps)
	}
	if len(state.ManagedApps) != 1 || state.ManagedApps[0].AppID != managedID {
		t.Fatalf("managed rows = %#v, want managed app", state.ManagedApps)
	}
	if state.MenuBarApps[0].CPUPercent != trackedTop.CPUPercent || state.TrackedApps[0].CPUPercent != trackedTop.CPUPercent {
		t.Fatalf("tracked current CPU = menu bar %v tracked %v top %v, want same", state.MenuBarApps[0].CPUPercent, state.TrackedApps[0].CPUPercent, trackedTop.CPUPercent)
	}
	if state.MenuBarApps[0].AverageCPUPercent != trackedTop.AverageCPUPercent || state.TrackedApps[0].AverageCPUPercent != trackedTop.AverageCPUPercent {
		t.Fatalf("tracked average CPU = menu bar %v tracked %v top %v, want same", state.MenuBarApps[0].AverageCPUPercent, state.TrackedApps[0].AverageCPUPercent, trackedTop.AverageCPUPercent)
	}
	if state.ManagedApps[0].CPUPercent != managedTop.CPUPercent {
		t.Fatalf("managed current CPU = %v, top %v, want same", state.ManagedApps[0].CPUPercent, managedTop.CPUPercent)
	}
	if state.ManagedApps[0].AverageCPUPercent != managedTop.AverageCPUPercent {
		t.Fatalf("managed average CPU = %v, top %v, want same", state.ManagedApps[0].AverageCPUPercent, managedTop.AverageCPUPercent)
	}
}

func TestBuildMenuBarStateSplitsTrackedAndManagedSections(t *testing.T) {
	trackedID := core.AppID{Name: "Tracked"}
	managedID := core.AppID{Name: "Managed"}
	controller := &Controller{
		cfg:   config.DefaultConfig(),
		stats: core.NewStatsDocument(),
		lastStatuses: map[string]core.AppStatus{
			trackedID.Key(): core.AppStatusObserved,
			managedID.Key(): core.AppStatusPaused,
		},
		lastGroups: []core.AppGroup{
			{
				ID:              trackedID,
				Controllability: core.ControllabilityNormal,
				Status:          core.AppStatusObserved,
				Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Tracked"}},
			},
			{
				ID:              managedID,
				Controllability: core.ControllabilityNormal,
				Status:          core.AppStatusPaused,
				Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 43}, Name: "Managed"}},
			},
		},
		lastAppSamples: []core.AppCPUSample{
			{AppID: trackedID, CPUPercent: 2, SampledAt: time.Unix(1, 0)},
			{AppID: managedID, CPUPercent: 5, SampledAt: time.Unix(1, 0)},
		},
	}
	controller.cfg.Rules = []core.AppRule{
		{AppID: trackedID, Mode: core.RuleModeObserveOnly, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps}},
		{AppID: managedID, Mode: core.RuleModePauseInBackground, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps}},
	}

	state := controller.buildMenuBarStateLocked(time.Unix(2, 0), "")
	if len(state.TrackedApps) != 1 || state.TrackedApps[0].AppID != trackedID {
		t.Fatalf("tracked rows = %#v, want tracked app only", state.TrackedApps)
	}
	if len(state.ManagedApps) != 1 || state.ManagedApps[0].AppID != managedID {
		t.Fatalf("managed rows = %#v, want managed app only", state.ManagedApps)
	}
}

func TestBuildMenuBarStateUsesCurrentCPUForTopRowsAndShowsRollingAverage(t *testing.T) {
	appA := core.AppID{Name: "A"}
	appB := core.AppID{Name: "B"}
	history := observe.NewCPUHistory(time.Minute, 100)
	history.AddBatch([]core.AppCPUSample{
		{
			AppID:        appA,
			CPUSeconds:   3,
			SampledAt:    time.Unix(3, 0),
			SampleWindow: 3 * time.Second,
		},
		{
			AppID:        appA,
			CPUSeconds:   0,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 3 * time.Second,
		},
		{
			AppID:        appB,
			CPUSeconds:   1.5,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 3 * time.Second,
		},
	})

	controller := &Controller{
		cfg:        config.DefaultConfig(),
		stats:      core.NewStatsDocument(),
		history:    history,
		lastGroups: testGroups(appA, appB),
		lastStatuses: map[string]core.AppStatus{
			appA.Key(): core.AppStatusObserved,
			appB.Key(): core.AppStatusObserved,
		},
		lastAppSamples: []core.AppCPUSample{
			{AppID: appA, CPUPercent: 0, SampledAt: time.Unix(6, 0)},
			{AppID: appB, CPUPercent: 100, SampledAt: time.Unix(6, 0)},
		},
	}

	state := controller.buildMenuBarStateLocked(time.Unix(6, 0), "")
	if len(state.TopProcesses) != 1 {
		t.Fatalf("top rows = %d, want only current CPU app", len(state.TopProcesses))
	}
	if state.TopProcesses[0].AppID != appB {
		t.Fatalf("top row = %#v, want current app B", state.TopProcesses[0])
	}
	if state.TopProcesses[0].CPUPercent != 100 {
		t.Fatalf("top row current CPU = %v, want live 100%%", state.TopProcesses[0].CPUPercent)
	}
	if state.TopProcesses[0].AverageCPUPercent != 25 {
		t.Fatalf("top row average CPU = %v, want rolling 25%%", state.TopProcesses[0].AverageCPUPercent)
	}
	if state.AllProcesses[1].AppID != appB || state.AllProcesses[1].CPUPercent != 100 {
		t.Fatalf("all process row = %#v, want live app B at 100%%", state.AllProcesses[1])
	}
	if len(state.CPUGraph.Lines) != 1 {
		t.Fatalf("graph lines = %d, want one current top-process line", len(state.CPUGraph.Lines))
	}
}

func TestBuildMenuBarStateCanSortTopRowsByRollingAverage(t *testing.T) {
	appA := core.AppID{Name: "A"}
	appB := core.AppID{Name: "B"}
	history := observe.NewCPUHistory(time.Minute, 100)
	history.AddBatch([]core.AppCPUSample{
		{
			AppID:        appA,
			CPUSeconds:   6,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 6 * time.Second,
		},
		{
			AppID:        appB,
			CPUSeconds:   1.5,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 6 * time.Second,
		},
	})

	cfg := config.DefaultConfig()
	cfg.Preferences.TopProcessesSort = core.TopProcessesSortAverage
	controller := &Controller{
		cfg:        cfg,
		stats:      core.NewStatsDocument(),
		history:    history,
		lastGroups: testGroups(appA, appB),
		lastStatuses: map[string]core.AppStatus{
			appA.Key(): core.AppStatusObserved,
			appB.Key(): core.AppStatusObserved,
		},
		lastAppSamples: []core.AppCPUSample{
			{AppID: appA, CPUPercent: 0, SampledAt: time.Unix(6, 0)},
			{AppID: appB, CPUPercent: 100, SampledAt: time.Unix(6, 0)},
		},
	}

	state := controller.buildMenuBarStateLocked(time.Unix(6, 0), "")
	if len(state.TopProcesses) != 2 {
		t.Fatalf("top rows = %d, want 2 average-ranked rows", len(state.TopProcesses))
	}
	if state.TopProcesses[0].AppID != appA || state.TopProcesses[1].AppID != appB {
		t.Fatalf("top rows = %#v, want average order A then B", state.TopProcesses)
	}
	if state.TopProcesses[0].CPUPercent != 0 || state.TopProcesses[0].AverageCPUPercent != 100 {
		t.Fatalf("app A CPU = current %v average %v, want current 0 average 100", state.TopProcesses[0].CPUPercent, state.TopProcesses[0].AverageCPUPercent)
	}
	if state.TopProcesses[1].CPUPercent != 100 || state.TopProcesses[1].AverageCPUPercent != 25 {
		t.Fatalf("app B CPU = current %v average %v, want current 100 average 25", state.TopProcesses[1].CPUPercent, state.TopProcesses[1].AverageCPUPercent)
	}
}

func TestBuildMenuBarStateIncludesBlockedSelfInCurrentTopRows(t *testing.T) {
	selfID := core.AppID{Name: "OpenTamer"}
	editorID := core.AppID{Name: "Editor"}
	history := observe.NewCPUHistory(time.Minute, 100)
	history.AddBatch([]core.AppCPUSample{
		{
			AppID:        selfID,
			CPUSeconds:   6,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 3 * time.Second,
		},
		{
			AppID:        editorID,
			CPUSeconds:   1.5,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 3 * time.Second,
		},
	})

	controller := &Controller{
		cfg:     config.DefaultConfig(),
		stats:   core.NewStatsDocument(),
		history: history,
		lastGroups: []core.AppGroup{
			{
				ID:              selfID,
				Controllability: core.ControllabilityBlocked,
				SafetyReason:    core.SafetyReasonOpenTamerSelf,
				Status:          core.AppStatusBlockedBySafety,
				Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 10}, Name: "OpenTamer"}},
			},
			{
				ID:              editorID,
				Controllability: core.ControllabilityNormal,
				Status:          core.AppStatusObserved,
				Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Editor"}},
			},
		},
		lastStatuses: map[string]core.AppStatus{
			selfID.Key():   core.AppStatusBlockedBySafety,
			editorID.Key(): core.AppStatusObserved,
		},
		lastAppSamples: []core.AppCPUSample{
			{AppID: selfID, CPUPercent: 20, SampledAt: time.Unix(6, 0)},
			{AppID: editorID, CPUPercent: 10, SampledAt: time.Unix(6, 0)},
		},
	}

	state := controller.buildMenuBarStateLocked(time.Unix(6, 0), "")
	if len(state.TopProcesses) < 2 || state.TopProcesses[0].AppID != selfID {
		t.Fatalf("top rows = %#v, want blocked self row ranked first by current CPU", state.TopProcesses)
	}
	if state.TopProcesses[0].AverageCPUPercent <= state.TopProcesses[1].AverageCPUPercent {
		t.Fatalf("top rows did not keep rolling average context: %#v", state.TopProcesses)
	}
	if state.TopProcesses[0].CPUPercent != 20 {
		t.Fatalf("top self current CPU = %v, want live 20", state.TopProcesses[0].CPUPercent)
	}
	if state.TopProcesses[0].BlockerReason != string(core.SafetyReasonOpenTamerSelf) {
		t.Fatalf("top self blocker = %q, want %q", state.TopProcesses[0].BlockerReason, core.SafetyReasonOpenTamerSelf)
	}
	if state.TopProcesses[0].CanPause || state.TopProcesses[0].CanSlow {
		t.Fatalf("top self capabilities = pause %v slow %v, want both disabled", state.TopProcesses[0].CanPause, state.TopProcesses[0].CanSlow)
	}
}

func TestBuildMenuBarStateUsesConfiguredCPUGraphWindow(t *testing.T) {
	appA := core.AppID{Name: "A"}
	now := time.Unix(900, 0)
	history := observe.NewCPUHistory(core.MaxCPUGraphWindow, 0)
	history.AddBatch([]core.AppCPUSample{
		{AppID: appA, CPUPercent: 10, SampledAt: now.Add(-9 * time.Minute)},
		{AppID: appA, CPUPercent: 20, SampledAt: now},
	})

	controller := &Controller{
		cfg:        config.DefaultConfig(),
		stats:      core.NewStatsDocument(),
		history:    history,
		lastGroups: testGroups(appA),
		lastStatuses: map[string]core.AppStatus{
			appA.Key(): core.AppStatusObserved,
		},
		lastAppSamples: []core.AppCPUSample{
			{AppID: appA, CPUPercent: 20, SampledAt: now},
		},
	}
	controller.cfg.Preferences.CPUGraphWindow = 10 * time.Minute

	state := controller.buildMenuBarStateLocked(now, "")
	if got := state.CPUGraph.WindowEndUnix - state.CPUGraph.WindowStartUnix; got != 600 {
		t.Fatalf("graph window seconds = %v, want 600", got)
	}
	if len(state.CPUGraph.Lines) != 1 {
		t.Fatalf("graph lines = %d, want 1", len(state.CPUGraph.Lines))
	}
	if len(state.CPUGraph.Lines[0].Points) != 2 {
		t.Fatalf("graph points = %d, want 2 from configured history window", len(state.CPUGraph.Lines[0].Points))
	}
}

func TestBuildMenuBarStateRefreshesTopRowsFromCurrentCPU(t *testing.T) {
	appA := core.AppID{Name: "A"}
	appB := core.AppID{Name: "B"}
	history := observe.NewCPUHistory(2*time.Minute, 100)
	history.AddBatch([]core.AppCPUSample{
		{
			AppID:        appA,
			CPUSeconds:   3,
			SampledAt:    time.Unix(3, 0),
			SampleWindow: 3 * time.Second,
		},
	})

	controller := &Controller{
		cfg:        config.DefaultConfig(),
		stats:      core.NewStatsDocument(),
		history:    history,
		lastGroups: testGroups(appA, appB),
		lastStatuses: map[string]core.AppStatus{
			appA.Key(): core.AppStatusObserved,
			appB.Key(): core.AppStatusObserved,
		},
		lastAppSamples: []core.AppCPUSample{
			{AppID: appA, CPUPercent: 100, SampledAt: time.Unix(3, 0)},
		},
	}

	first := controller.buildMenuBarStateLocked(time.Unix(6, 0), "")
	if len(first.TopProcesses) != 1 || first.TopProcesses[0].AppID != appA {
		t.Fatalf("first top rows = %#v, want app A", first.TopProcesses)
	}
	if first.TopProcesses[0].CPUPercent != 100 {
		t.Fatalf("first top percent = %v, want 100", first.TopProcesses[0].CPUPercent)
	}

	history.AddBatch([]core.AppCPUSample{
		{
			AppID:        appB,
			CPUSeconds:   6,
			SampledAt:    time.Unix(9, 0),
			SampleWindow: 3 * time.Second,
		},
	})
	controller.lastAppSamples = []core.AppCPUSample{
		{AppID: appB, CPUPercent: 100, SampledAt: time.Unix(9, 0)},
	}

	second := controller.buildMenuBarStateLocked(time.Unix(9, 0), "")
	if len(second.TopProcesses) != 1 || second.TopProcesses[0].AppID != appB {
		t.Fatalf("second top rows = %#v, want app B first after current CPU update", second.TopProcesses)
	}
	if second.TopProcesses[0].CPUPercent != 100 {
		t.Fatalf("second top current percent = %v, want live 100", second.TopProcesses[0].CPUPercent)
	}
	if second.TopProcesses[0].AverageCPUPercent <= 0 {
		t.Fatalf("top row missing rolling average context: %#v", second.TopProcesses)
	}
}

func TestHandleRuleCommandCreatesCPULimitRule(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Worker"}},
		}},
	}
	controller.cfg.Preferences.CPULimiterEnabled = false

	controller.handleRuleCommandLocked([]string{"rule", "limit", "0.01", appID.Key()})

	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if rule.Mode != core.RuleModeLimitCPUInBackground {
		t.Fatalf("mode = %q, want CPU limit", rule.Mode)
	}
	if rule.BackgroundOnly {
		t.Fatal("CPU limit rules from the menu should apply while foreground")
	}
	if rule.CPUPercent == nil || *rule.CPUPercent != apppolicy.MinCPULimitPercent {
		t.Fatalf("cpu percent = %#v, want %.2f", rule.CPUPercent, apppolicy.MinCPULimitPercent)
	}
	if !controller.cfg.Preferences.CPULimiterEnabled {
		t.Fatal("CPU limiter should be enabled after creating a limit rule")
	}
}

func TestCPULimitRequestsIncludeAllProcessesInAggregateGroup(t *testing.T) {
	appID := core.AppID{Name: "worker"}
	target := 1.0
	requests := cpuLimitRequests(
		[]core.AppGroup{{
			ID: appID,
			Processes: []core.ProcessRef{
				{ID: core.ProcessID{PID: 10, StartTime: time.Unix(10, 0)}, Name: "worker"},
				{ID: core.ProcessID{PID: 11, StartTime: time.Unix(11, 0)}, Name: "worker"},
			},
		}},
		[]core.ControlAction{{
			Type:       core.ControlActionLimitCPU,
			AppID:      appID,
			CPUPercent: &target,
			RunFor:     time.Millisecond,
			StopFor:    time.Second,
		}},
	)

	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	if len(requests[0].Processes) != 2 {
		t.Fatalf("limited processes = %#v, want both aggregate processes", requests[0].Processes)
	}
}

func TestHandleRuleCommandCreatesPriorityRuleScopes(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Worker"}},
		}},
	}

	controller.handleRuleCommandLocked([]string{"rule", "priority-background", appID.Key()})
	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	if rule := controller.cfg.Rules[0]; rule.Mode != core.RuleModeLowerPriorityInBackground || !rule.BackgroundOnly {
		t.Fatalf("background priority rule = %#v", rule)
	}

	controller.handleRuleCommandLocked([]string{"rule", "priority-always", appID.Key()})
	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	if rule := controller.cfg.Rules[0]; rule.Mode != core.RuleModeLowerPriorityInBackground || rule.BackgroundOnly {
		t.Fatalf("always priority rule = %#v", rule)
	}
}

func TestHandleRuleCommandAddsTrackingLocations(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Worker"}},
		}},
	}

	controller.handleRuleCommandLocked([]string{"rule", "track-menu-bar", appID.Key()})
	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if rule.Mode != core.RuleModeObserveOnly {
		t.Fatalf("mode = %q, want observe-only tracking rule", rule.Mode)
	}
	if !rule.TracksIn(core.RuleTrackInMenuBar) || rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in after menu bar = %#v", rule.TrackIn)
	}

	controller.handleRuleCommandLocked([]string{"rule", "track-managed", appID.Key()})
	rule = controller.cfg.Rules[0]
	if !rule.TracksIn(core.RuleTrackInMenuBar) || !rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in after managed = %#v, want both", rule.TrackIn)
	}
}

func TestHandleRuleCommandAcceptsLegacyTrayTrackingCommand(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Worker"}},
		}},
	}

	controller.handleRuleCommandLocked([]string{"rule", "track-tray", appID.Key()})
	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	if rule := controller.cfg.Rules[0]; !rule.TracksIn(core.RuleTrackInMenuBar) {
		t.Fatalf("track in = %#v, want menu bar", rule.TrackIn)
	}
}

func TestHandleRuleCommandAllowsTrackingBlockedApps(t *testing.T) {
	appID := core.AppID{Name: "OpenTamer"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityBlocked,
			SafetyReason:    core.SafetyReasonOpenTamerSelf,
			Status:          core.AppStatusBlockedBySafety,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "OpenTamer"}},
		}},
	}

	controller.handleRuleCommandLocked([]string{"rule", "track-menu-bar", appID.Key()})
	controller.handleRuleCommandLocked([]string{"rule", "track-managed", appID.Key()})

	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if rule.Mode != core.RuleModeObserveOnly {
		t.Fatalf("mode = %q, want observe-only tracking rule", rule.Mode)
	}
	if !rule.TracksIn(core.RuleTrackInMenuBar) || !rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want menu bar and tracked apps", rule.TrackIn)
	}
}

func TestHandleRuleCommandDowngradesUnsafeExistingRuleWhenTrackingBlockedApp(t *testing.T) {
	appID := core.AppID{Name: "OpenTamer"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityBlocked,
			SafetyReason:    core.SafetyReasonOpenTamerSelf,
			Status:          core.AppStatusBlockedBySafety,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "OpenTamer"}},
		}},
	}
	nice := core.DefaultBackgroundNice
	controller.cfg.Rules = []core.AppRule{{
		AppID:      appID,
		Mode:       core.RuleModeLowerPriorityInBackground,
		NiceValue:  &nice,
		CPUPercent: new(float64(1)),
	}}

	controller.handleRuleCommandLocked([]string{"rule", "track-menu-bar", appID.Key()})

	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if rule.Mode != core.RuleModeObserveOnly {
		t.Fatalf("mode = %q, want unsafe rule downgraded to observe-only", rule.Mode)
	}
	if rule.NiceValue != nil || rule.CPUPercent != nil {
		t.Fatalf("control fields were not cleared: %#v", rule)
	}
	if !rule.TracksIn(core.RuleTrackInMenuBar) {
		t.Fatalf("track in = %#v, want menu bar", rule.TrackIn)
	}
}

func TestHandleRuleCommandRejectsBlockedManagementRules(t *testing.T) {
	appID := core.AppID{Name: "OpenTamer"}
	for _, command := range [][]string{
		{"rule", "pause", appID.Key()},
		{"rule", "priority-background", appID.Key()},
		{"rule", "limit", "1", appID.Key()},
	} {
		controller := &Controller{
			store: config.NewStore(t.TempDir()),
			cfg:   config.DefaultConfig(),
			lastGroups: []core.AppGroup{{
				ID:              appID,
				Controllability: core.ControllabilityBlocked,
				SafetyReason:    core.SafetyReasonOpenTamerSelf,
				Status:          core.AppStatusBlockedBySafety,
				Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "OpenTamer"}},
			}},
		}
		controller.cfg.Preferences.CPULimiterEnabled = false

		controller.handleRuleCommandLocked(command)

		if len(controller.cfg.Rules) != 0 {
			t.Fatalf("%v created rules = %#v, want none", command, controller.cfg.Rules)
		}
		if controller.cfg.Preferences.CPULimiterEnabled {
			t.Fatalf("%v enabled CPU limiter despite rejected rule", command)
		}
	}
}

func TestHandleRuleCommandAddsMenuBarFromTrackedAppsRule(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Worker"}},
		}},
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModeObserveOnly,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "track-menu-bar", appID.Key()})

	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if !rule.TracksIn(core.RuleTrackInMenuBar) || !rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want menu bar and tracked apps", rule.TrackIn)
	}
}

func TestHandleRuleCommandPreservesTrackingWhenChangingControlMode(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
		lastGroups: []core.AppGroup{{
			ID:              appID,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "Worker"}},
		}},
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModeObserveOnly,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "priority-background", appID.Key()})
	rule := controller.cfg.Rules[0]
	if rule.Mode != core.RuleModeLowerPriorityInBackground {
		t.Fatalf("mode = %q, want priority", rule.Mode)
	}
	if !rule.TracksIn(core.RuleTrackInMenuBar) || rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want menu bar only preserved", rule.TrackIn)
	}
}

func TestHandleRuleCommandTogglesPauseRuleOff(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store:      config.NewStore(t.TempDir()),
		cfg:        config.DefaultConfig(),
		lastGroups: testGroups(appID),
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModePauseInBackground,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "pause", appID.Key()})

	if len(controller.cfg.Rules) != 0 {
		t.Fatalf("rules = %#v, want pause rule removed", controller.cfg.Rules)
	}
}

func TestHandleRuleCommandTogglesPauseRuleToMenuBarTracking(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store:      config.NewStore(t.TempDir()),
		cfg:        config.DefaultConfig(),
		lastGroups: testGroups(appID),
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModePauseInBackground,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "pause", appID.Key()})

	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want menu bar tracking rule preserved", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if rule.Mode != core.RuleModeObserveOnly {
		t.Fatalf("mode = %q, want observe-only tracking rule", rule.Mode)
	}
	if !rule.TracksIn(core.RuleTrackInMenuBar) || rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want menu bar only", rule.TrackIn)
	}
}

func TestHandleRuleCommandUntracksMenuBarOnlyObserveRule(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModeObserveOnly,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "untrack-menu-bar", appID.Key()})
	if len(controller.cfg.Rules) != 0 {
		t.Fatalf("rules = %#v, want observe-only menu bar rule removed", controller.cfg.Rules)
	}
}

func TestHandleRuleCommandUntracksMenuBarAndPreservesControlRule(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModePauseInBackground,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "untrack-menu-bar", appID.Key()})
	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want control rule preserved", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if rule.Mode != core.RuleModePauseInBackground {
		t.Fatalf("mode = %q, want pause rule", rule.Mode)
	}
	if rule.TracksIn(core.RuleTrackInMenuBar) || rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want no tracking locations", rule.TrackIn)
	}
}

func TestHandleRuleCommandUntracksManagedLocation(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModeObserveOnly,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar, core.RuleTrackInManagedApps},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "untrack-managed", appID.Key()})
	if len(controller.cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want observe rule preserved in menu bar", len(controller.cfg.Rules))
	}
	rule := controller.cfg.Rules[0]
	if !rule.TracksIn(core.RuleTrackInMenuBar) || rule.TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want menu bar only", rule.TrackIn)
	}
}

func TestHandleRuleCommandUntracksManagedOnlyObserveRule(t *testing.T) {
	appID := core.AppID{Name: "Worker"}
	controller := &Controller{
		store: config.NewStore(t.TempDir()),
		cfg:   config.DefaultConfig(),
	}
	controller.cfg.Rules = []core.AppRule{{
		AppID:   appID,
		Mode:    core.RuleModeObserveOnly,
		TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps},
	}}

	controller.handleRuleCommandLocked([]string{"rule", "untrack-managed", appID.Key()})
	if len(controller.cfg.Rules) != 0 {
		t.Fatalf("rules = %#v, want observe-only managed rule removed", controller.cfg.Rules)
	}
}

func testGroups(ids ...core.AppID) []core.AppGroup {
	groups := make([]core.AppGroup, 0, len(ids))
	for index, id := range ids {
		groups = append(groups, core.AppGroup{
			ID:              id,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes: []core.ProcessRef{{
				ID:   core.ProcessID{PID: index + 1},
				Name: id.DisplayName(),
			}},
		})
	}
	return groups
}

func floatPtr(value float64) *float64 {
	return new(value)
}
