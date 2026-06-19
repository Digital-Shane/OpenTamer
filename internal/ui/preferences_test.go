package ui

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

func TestApplyPreferencesCommandUpdatesTypedControl(t *testing.T) {
	cfg := config.DefaultConfig()
	updated, err := ApplyPreferencesCommand(cfg, PreferencesCommand{Kind: CommandSetManagementEnabled, BoolValue: false})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if updated.Preferences.ManagementEnabled {
		t.Fatal("management should be disabled")
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{Kind: CommandSetStatsInterval, DurationValue: 5 * time.Second})
	if err != nil {
		t.Fatalf("apply interval: %v", err)
	}
	if updated.Preferences.StatsInterval != 5*time.Second {
		t.Fatalf("interval = %s", updated.Preferences.StatsInterval)
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{Kind: CommandSetCPUGraphWindow, DurationValue: 10 * time.Minute})
	if err != nil {
		t.Fatalf("apply graph window: %v", err)
	}
	if updated.Preferences.CPUGraphWindow != 10*time.Minute {
		t.Fatalf("CPU graph window = %s", updated.Preferences.CPUGraphWindow)
	}
}

func TestApplyPreferencesCommandUpdatesGenericGlobalPreferences(t *testing.T) {
	cfg := config.DefaultConfig()

	updated, err := ApplyPreferencesCommand(cfg, PreferencesCommand{
		Kind:      CommandSetBoolPreference,
		Field:     PreferenceAggregateByName,
		BoolValue: false,
	})
	if err != nil {
		t.Fatalf("apply aggregate by name: %v", err)
	}
	if updated.Preferences.AggregateByName {
		t.Fatal("aggregate by name should be disabled")
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{
		Kind:          CommandSetDurationPreference,
		Field:         PreferenceWakeGrace,
		DurationValue: 0,
	})
	if err != nil {
		t.Fatalf("apply wake grace: %v", err)
	}
	if updated.Preferences.WakeGrace != 0 {
		t.Fatalf("wake grace = %s, want off", updated.Preferences.WakeGrace)
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{
		Kind:          CommandSetDurationPreference,
		Field:         PreferenceCPUGraphWindow,
		DurationValue: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("apply graph window: %v", err)
	}
	if updated.Preferences.CPUGraphWindow != core.MinCPUGraphWindow {
		t.Fatalf("CPU graph window = %s, want normalized minimum %s", updated.Preferences.CPUGraphWindow, core.MinCPUGraphWindow)
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{
		Kind:       CommandSetFloatPreference,
		Field:      PreferenceHighCPUThreshold,
		FloatValue: 90,
	})
	if err != nil {
		t.Fatalf("apply high CPU threshold: %v", err)
	}
	if updated.Preferences.HighCPUThreshold != 90 {
		t.Fatalf("high CPU threshold = %v, want 90", updated.Preferences.HighCPUThreshold)
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{
		Kind:       CommandSetFloatPreference,
		Field:      PreferenceDisableWhenACBatteryAbove,
		FloatValue: 75,
	})
	if err != nil {
		t.Fatalf("apply battery threshold: %v", err)
	}
	if updated.Preferences.DisableWhenACBatteryAbove == nil || *updated.Preferences.DisableWhenACBatteryAbove != 75 {
		t.Fatalf("battery threshold = %#v, want 75", updated.Preferences.DisableWhenACBatteryAbove)
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{
		Kind:  CommandClearPreference,
		Field: PreferenceDisableWhenACBatteryAbove,
	})
	if err != nil {
		t.Fatalf("clear battery threshold: %v", err)
	}
	if updated.Preferences.DisableWhenACBatteryAbove != nil {
		t.Fatalf("battery threshold = %#v, want nil", updated.Preferences.DisableWhenACBatteryAbove)
	}

	updated, err = ApplyPreferencesCommand(updated, PreferencesCommand{
		Kind:        CommandSetStringPreference,
		Field:       PreferenceTopProcessesSort,
		StringValue: core.TopProcessesSortAverage,
	})
	if err != nil {
		t.Fatalf("apply top processes sort: %v", err)
	}
	if updated.Preferences.TopProcessesSort != core.TopProcessesSortAverage {
		t.Fatalf("top processes sort = %q, want %q", updated.Preferences.TopProcessesSort, core.TopProcessesSortAverage)
	}
}

func TestApplyPreferencesCommandRejectsInvalidGenericPreference(t *testing.T) {
	cfg := config.DefaultConfig()
	if _, err := ApplyPreferencesCommand(cfg, PreferencesCommand{
		Kind:          CommandSetDurationPreference,
		Field:         PreferenceStatsInterval,
		DurationValue: 0,
	}); err == nil {
		t.Fatal("expected zero stats interval to fail")
	}
	if _, err := ApplyPreferencesCommand(cfg, PreferencesCommand{
		Kind:  CommandSetBoolPreference,
		Field: PreferenceStatsInterval,
	}); err == nil {
		t.Fatal("expected wrong preference type to fail")
	}
	if _, err := ApplyPreferencesCommand(cfg, PreferencesCommand{
		Kind:        CommandSetStringPreference,
		Field:       PreferenceTopProcessesSort,
		StringValue: "bogus",
	}); err == nil {
		t.Fatal("expected invalid top processes sort mode to fail")
	}
}

func TestApplyPreferencesCommandUpsertsAndResetsRule(t *testing.T) {
	cfg := config.DefaultConfig()
	app := core.AppID{Name: "Editor"}
	cfg, err := ApplyPreferencesCommand(cfg, PreferencesCommand{
		Kind: CommandUpsertRule,
		Rule: core.AppRule{AppID: app, Mode: core.RuleModePauseInBackground},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(cfg.Rules))
	}

	cfg, err = ApplyPreferencesCommand(cfg, PreferencesCommand{Kind: CommandResetRule, AppID: app})
	if err != nil {
		t.Fatalf("reset rule: %v", err)
	}
	if len(cfg.Rules) != 0 {
		t.Fatalf("rules = %d, want 0", len(cfg.Rules))
	}
}

func TestApplyPreferencesRejectsCPULimitBelowMinimum(t *testing.T) {
	cfg := config.DefaultConfig()
	target := apppolicy.MinCPULimitPercent / 2
	_, err := ApplyPreferencesCommand(cfg, PreferencesCommand{
		Kind: CommandUpsertRule,
		Rule: core.AppRule{
			AppID:      core.AppID{Name: "Worker"},
			Mode:       core.RuleModeLimitCPUInBackground,
			CPUPercent: &target,
		},
	})
	if err == nil {
		t.Fatal("expected CPU limit below minimum to fail")
	}
}
