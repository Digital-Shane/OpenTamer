package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestLoadConfigReturnsDefaultsWhenMissing(t *testing.T) {
	cfg, err := NewStore(t.TempDir()).LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Preferences.ManagementEnabled {
		t.Fatal("management should default enabled")
	}
	if !cfg.Preferences.CPULimiterEnabled {
		t.Fatal("CPU limiter should default enabled")
	}
	if cfg.Preferences.StatsInterval != 3*time.Second {
		t.Fatalf("stats interval = %s", cfg.Preferences.StatsInterval)
	}
	if !cfg.Preferences.AggregateByName {
		t.Fatal("aggregate by name should default enabled")
	}
	if !cfg.Preferences.ShowMenuBarIcon {
		t.Fatal("show menu bar icon should default enabled")
	}
	if cfg.Preferences.CPUGraphWindow != core.DefaultCPUGraphWindow {
		t.Fatalf("CPU graph window = %s, want %s", cfg.Preferences.CPUGraphWindow, core.DefaultCPUGraphWindow)
	}
	if cfg.Preferences.TopProcessesSort != core.TopProcessesSortCurrent {
		t.Fatalf("top processes sort = %q, want %q", cfg.Preferences.TopProcessesSort, core.TopProcessesSortCurrent)
	}
	if cfg.Preferences.CPUDisplayMode != core.CPUDisplayModePerCoreProcess {
		t.Fatalf("CPU display mode = %q, want %q", cfg.Preferences.CPUDisplayMode, core.CPUDisplayModePerCoreProcess)
	}
	if cfg.Preferences.Theme != "system" {
		t.Fatalf("theme = %q, want %q", cfg.Preferences.Theme, "system")
	}
}

func TestSaveConfigWritesAtomicallyReadableConfig(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.Rules = append(cfg.Rules, core.AppRule{
		AppID:        core.AppID{BundleID: "com.apple.Safari", Name: "Safari"},
		Mode:         core.RuleModeLowerPriorityInBackground,
		ProtectAudio: true,
	})

	if err := store.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(loaded.Rules))
	}
	if loaded.Rules[0].Mode == core.RuleModePauseInBackground {
		t.Fatal("browser starter preset should not full pause")
	}
}

func TestSaveConfigOmitsDisabledOptionalPolicyGuards(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.Preferences.DisableWhenACBatteryAbove = nil
	cfg.Preferences.DisableWhenUserIdleLongerThan = 0

	if err := store.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if bytes.Contains(payload, []byte("disableWhenACBatteryAbove")) {
		t.Fatalf("disabled AC battery guard should be omitted: %s", payload)
	}
	if bytes.Contains(payload, []byte("disableWhenUserIdleLongerThan")) {
		t.Fatalf("disabled idle guard should be omitted: %s", payload)
	}
}

func TestInvalidConfigReturnsDefaultsAndError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	cfg, err := NewStore(dir).LoadConfig()
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !cfg.Preferences.ManagementEnabled {
		t.Fatal("defaults should be returned on invalid config")
	}
}

func TestMigrateConfigFillsDefaults(t *testing.T) {
	cfg, err := MigrateConfig(Config{SchemaVersion: 0})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema = %d", cfg.SchemaVersion)
	}
	if cfg.Preferences.StatsInterval == 0 {
		t.Fatal("expected stats interval default")
	}
	if cfg.Preferences.CPUGraphWindow != core.DefaultCPUGraphWindow {
		t.Fatalf("CPU graph window = %s, want %s", cfg.Preferences.CPUGraphWindow, core.DefaultCPUGraphWindow)
	}
	if cfg.Preferences.TopProcessesSort != core.TopProcessesSortCurrent {
		t.Fatalf("top processes sort = %q, want %q", cfg.Preferences.TopProcessesSort, core.TopProcessesSortCurrent)
	}
	if cfg.Preferences.CPUDisplayMode != core.CPUDisplayModePerCoreProcess {
		t.Fatalf("CPU display mode = %q, want %q", cfg.Preferences.CPUDisplayMode, core.CPUDisplayModePerCoreProcess)
	}
}

func TestLoadConfigBackfillsMissingPreferences(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	payload := []byte(`{
  "schemaVersion": 1,
  "preferences": {
    "managementEnabled": true,
    "statsInterval": 3000000000,
    "averagingWindow": 30000000000,
    "startupGrace": 30000000000,
    "wakeGrace": 30000000000,
    "highCPUDetectionEnabled": true,
    "highCPUThreshold": 75,
    "highCPUDuration": 30000000000,
    "highCPUCooldown": 600000000000
  },
  "rules": [],
  "safetyOverrides": {}
}
`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), payload, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Preferences.AggregateByName {
		t.Fatal("aggregate by name should be backfilled to true")
	}
	if !cfg.Preferences.CPULimiterEnabled {
		t.Fatal("CPU limiter should be backfilled to true")
	}
	if !cfg.Preferences.ShowMenuBarIcon {
		t.Fatal("show menu bar icon should be backfilled to true")
	}
	if cfg.Preferences.CPUGraphWindow != core.DefaultCPUGraphWindow {
		t.Fatalf("CPU graph window = %s, want %s", cfg.Preferences.CPUGraphWindow, core.DefaultCPUGraphWindow)
	}
	if cfg.Preferences.TopProcessesSort != core.TopProcessesSortCurrent {
		t.Fatalf("top processes sort = %q, want %q", cfg.Preferences.TopProcessesSort, core.TopProcessesSortCurrent)
	}
	if cfg.Preferences.CPUDisplayMode != core.CPUDisplayModePerCoreProcess {
		t.Fatalf("CPU display mode = %q, want %q", cfg.Preferences.CPUDisplayMode, core.CPUDisplayModePerCoreProcess)
	}
	if cfg.Preferences.Theme != "system" {
		t.Fatal("theme should be backfilled to system")
	}
	updated, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if !bytes.Contains(updated, []byte(`"aggregateByName": true`)) {
		t.Fatalf("updated config missing aggregateByName: %s", updated)
	}
	if !bytes.Contains(updated, []byte(`"cpuLimiterEnabled": true`)) {
		t.Fatalf("updated config missing cpuLimiterEnabled: %s", updated)
	}
	if !bytes.Contains(updated, []byte(`"showMenuBarIcon": true`)) {
		t.Fatalf("updated config missing showMenuBarIcon: %s", updated)
	}
	if !bytes.Contains(updated, []byte(`"cpuGraphWindow": 300000000000`)) {
		t.Fatalf("updated config missing cpuGraphWindow: %s", updated)
	}
	if !bytes.Contains(updated, []byte(`"topProcessesSort": "current"`)) {
		t.Fatalf("updated config missing topProcessesSort: %s", updated)
	}
	if !bytes.Contains(updated, []byte(`"cpuDisplayMode": "per_core_process"`)) {
		t.Fatalf("updated config missing cpuDisplayMode: %s", updated)
	}
	if !bytes.Contains(updated, []byte(`"theme": "system"`)) {
		t.Fatalf("updated config missing theme: %s", updated)
	}
	if bytes.Contains(updated, []byte("startupGrace")) {
		t.Fatalf("updated config should drop legacy startupGrace: %s", updated)
	}
	if bytes.Contains(updated, []byte("safetyOverrides")) {
		t.Fatalf("updated config should drop safetyOverrides: %s", updated)
	}
	if bytes.Contains(updated, []byte("ignoredHighCPUApps")) {
		t.Fatalf("updated config should drop ignoredHighCPUApps: %s", updated)
	}
}

func TestLoadConfigMigratesLegacyTrayTrackingLocation(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	payload := []byte(`{
  "schemaVersion": 1,
  "preferences": {
    "managementEnabled": true,
    "cpuLimiterEnabled": true,
    "aggregateByName": true,
    "showMenuBarIcon": true,
    "topProcessesSort": "current",
    "statsInterval": 3000000000,
    "averagingWindow": 30000000000,
    "cpuGraphWindow": 300000000000,
    "wakeGrace": 30000000000,
    "highCPUDetectionEnabled": true,
    "highCPUThreshold": 75,
    "highCPUDuration": 30000000000,
    "highCPUCooldown": 600000000000
  },
  "rules": [
    {
      "appID": {"name": "Worker"},
      "mode": "observe_only",
      "trackIn": ["tray"]
    }
  ]
}
`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), payload, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Rules) != 1 || !cfg.Rules[0].TracksIn(core.RuleTrackInMenuBar) {
		t.Fatalf("rules = %#v, want menu bar tracking", cfg.Rules)
	}
	updated, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if !bytes.Contains(updated, []byte(`"menu_bar"`)) {
		t.Fatalf("updated config missing menu_bar tracking: %s", updated)
	}
	if bytes.Contains(updated, []byte(`"tray"`)) {
		t.Fatalf("updated config should not retain legacy tray tracking: %s", updated)
	}
}

func TestLoadConfigMigratesLegacyStartupGraceToWakeGrace(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	payload := []byte(`{
  "schemaVersion": 1,
  "preferences": {
    "managementEnabled": true,
    "cpuLimiterEnabled": false,
    "aggregateByName": true,
    "showMenuBarIcon": true,
    "topProcessesSort": "current",
    "statsInterval": 3000000000,
    "averagingWindow": 30000000000,
    "cpuGraphWindow": 300000000000,
    "startupGrace": 10000000000,
    "highCPUDetectionEnabled": true,
    "highCPUThreshold": 75,
    "highCPUDuration": 30000000000,
    "highCPUCooldown": 600000000000
  },
  "rules": []
}
`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), payload, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Preferences.WakeGrace != 10*time.Second {
		t.Fatalf("wake grace = %s, want legacy startup grace", cfg.Preferences.WakeGrace)
	}
	updated, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if bytes.Contains(updated, []byte("startupGrace")) {
		t.Fatalf("updated config should drop legacy startupGrace: %s", updated)
	}
	if !bytes.Contains(updated, []byte(`"wakeGrace": 10000000000`)) {
		t.Fatalf("updated config missing migrated wakeGrace: %s", updated)
	}
}

func TestMigrateConfigNormalizesTopProcessesSort(t *testing.T) {
	cfg, err := MigrateConfig(Config{
		SchemaVersion: CurrentSchemaVersion,
		Preferences: core.GlobalPreferences{
			TopProcessesSort: "bogus",
		},
	})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if cfg.Preferences.TopProcessesSort != core.TopProcessesSortCurrent {
		t.Fatalf("top processes sort = %q, want %q", cfg.Preferences.TopProcessesSort, core.TopProcessesSortCurrent)
	}

	cfg, err = MigrateConfig(Config{
		SchemaVersion: CurrentSchemaVersion,
		Preferences: core.GlobalPreferences{
			TopProcessesSort: core.TopProcessesSortAverage,
		},
	})
	if err != nil {
		t.Fatalf("migrate average: %v", err)
	}
	if cfg.Preferences.TopProcessesSort != core.TopProcessesSortAverage {
		t.Fatalf("top processes sort = %q, want %q", cfg.Preferences.TopProcessesSort, core.TopProcessesSortAverage)
	}
}

func TestMigrateConfigNormalizesCPUDisplayMode(t *testing.T) {
	cfg, err := MigrateConfig(Config{
		SchemaVersion: CurrentSchemaVersion,
		Preferences: core.GlobalPreferences{
			CPUDisplayMode: "bogus",
		},
	})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if cfg.Preferences.CPUDisplayMode != core.CPUDisplayModePerCoreProcess {
		t.Fatalf("CPU display mode = %q, want %q", cfg.Preferences.CPUDisplayMode, core.CPUDisplayModePerCoreProcess)
	}

	cfg, err = MigrateConfig(Config{
		SchemaVersion: CurrentSchemaVersion,
		Preferences: core.GlobalPreferences{
			CPUDisplayMode: core.CPUDisplayModeSystemNormalized,
		},
	})
	if err != nil {
		t.Fatalf("migrate system normalized: %v", err)
	}
	if cfg.Preferences.CPUDisplayMode != core.CPUDisplayModeSystemNormalized {
		t.Fatalf("CPU display mode = %q, want %q", cfg.Preferences.CPUDisplayMode, core.CPUDisplayModeSystemNormalized)
	}
}

func TestLoadConfigPreservesExplicitBooleanPreferenceFalse(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.Preferences.ManagementEnabled = false
	cfg.Preferences.CPULimiterEnabled = false
	cfg.Preferences.AggregateByName = false
	cfg.Preferences.ShowMenuBarIcon = false
	cfg.Preferences.HighCPUDetectionEnabled = false
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Preferences.AggregateByName {
		t.Fatal("explicit aggregate by name false should be preserved")
	}
	if loaded.Preferences.ManagementEnabled {
		t.Fatal("explicit management enabled false should be preserved")
	}
	if loaded.Preferences.CPULimiterEnabled {
		t.Fatal("explicit CPU limiter enabled false should be preserved")
	}
	if loaded.Preferences.ShowMenuBarIcon {
		t.Fatal("explicit show menu bar icon false should be preserved")
	}
	if loaded.Preferences.HighCPUDetectionEnabled {
		t.Fatal("explicit high CPU detection false should be preserved")
	}
}

func TestMigrateConfigBackfillsRuleTrackingLocations(t *testing.T) {
	observe := core.AppID{Name: "Observer"}
	paused := core.AppID{Name: "Paused"}
	cfg, err := MigrateConfig(Config{
		SchemaVersion: 1,
		Rules: []core.AppRule{
			{AppID: observe, Mode: core.RuleModeObserveOnly},
			{AppID: paused, Mode: core.RuleModePauseInBackground},
		},
	})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if !cfg.Rules[0].TracksIn(core.RuleTrackInMenuBar) || !cfg.Rules[0].TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("observe track in = %#v, want menu bar and managed apps", cfg.Rules[0].TrackIn)
	}
	if cfg.Rules[1].TracksIn(core.RuleTrackInMenuBar) || !cfg.Rules[1].TracksIn(core.RuleTrackInManagedApps) {
		t.Fatalf("pause track in = %#v, want managed apps only", cfg.Rules[1].TrackIn)
	}
}

func TestRuntimeStateSurvivesRelaunch(t *testing.T) {
	store := NewStore(t.TempDir())
	state := core.NewRuntimeState()
	process := core.ProcessRef{ID: core.ProcessID{PID: 42}, Name: "sleep"}
	state.RecordPause(process, core.AppID{Name: "Sleep"}, core.ControlReasonUserRule, time.Unix(1, 0))

	if err := store.SaveRuntimeState(state); err != nil {
		t.Fatalf("save runtime: %v", err)
	}
	loaded, err := store.LoadRuntimeState()
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if len(loaded.PausedProcesses) != 1 {
		t.Fatalf("paused = %d, want 1", len(loaded.PausedProcesses))
	}
}

func TestStatsSurviveRelaunch(t *testing.T) {
	store := NewStore(t.TempDir())
	document := core.NewStatsDocument()
	document.RecordAutomaticPause(core.AppID{Name: "App"})

	if err := store.SaveStats(document); err != nil {
		t.Fatalf("save stats: %v", err)
	}
	loaded, err := store.LoadStats()
	if err != nil {
		t.Fatalf("load stats: %v", err)
	}
	if loaded.AutomaticPauses != 1 {
		t.Fatalf("automatic pauses = %d, want 1", loaded.AutomaticPauses)
	}
}
