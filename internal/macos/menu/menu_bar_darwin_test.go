//go:build darwin && cgo

package menu

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/ui"
)

func TestRunMenuBarAppEncodesInitialStateForNativeBridge(t *testing.T) {
	old := menuBarNative
	t.Cleanup(func() {
		menuBarNative = old
	})

	var payload string
	menuBarNative.run = func(value string) {
		payload = value
	}

	state := ui.MenuBarViewState{
		Enabled:         true,
		ShowMenuBarIcon: true,
		TotalCPU:        42.25,
		AlertLevel:      core.AlertLevelHigh,
		StatusMessage:   "High CPU",
		LastUpdated:     time.Unix(1710000000, 0).UTC(),
	}
	if err := RunMenuBarApp(state); err != nil {
		t.Fatalf("run menu bar app: %v", err)
	}

	var decoded ui.MenuBarViewState
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decode native payload: %v", err)
	}
	if !decoded.Enabled || !decoded.ShowMenuBarIcon {
		t.Fatalf("decoded state flags = enabled:%t icon:%t, want both true", decoded.Enabled, decoded.ShowMenuBarIcon)
	}
	if decoded.TotalCPU != 42.25 {
		t.Fatalf("decoded total CPU = %v, want 42.25", decoded.TotalCPU)
	}
	if decoded.AlertLevel != core.AlertLevelHigh {
		t.Fatalf("decoded alert level = %q, want %q", decoded.AlertLevel, core.AlertLevelHigh)
	}
	if decoded.StatusMessage != "High CPU" {
		t.Fatalf("decoded status = %q, want High CPU", decoded.StatusMessage)
	}
}

func TestUpdateMenuBarStateAndUpdaterEncodeStateForNativeBridge(t *testing.T) {
	old := menuBarNative
	t.Cleanup(func() {
		menuBarNative = old
	})

	var payloads []string
	menuBarNative.update = func(value string) {
		payloads = append(payloads, value)
	}

	budget := 62.5
	state := ui.MenuBarViewState{
		Enabled:     true,
		TotalCPU:    9.5,
		AlertLevel:  core.AlertLevelNormal,
		Preferences: core.GlobalPreferences{GlobalCPUPercent: &budget},
	}
	if err := UpdateMenuBarState(state); err != nil {
		t.Fatalf("update menu bar state: %v", err)
	}
	if err := (MenuBarUpdater{}).UpdateMenuBarState(state); err != nil {
		t.Fatalf("updater update menu bar state: %v", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("native update calls = %d, want 2", len(payloads))
	}

	for _, payload := range payloads {
		var decoded ui.MenuBarViewState
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatalf("decode native payload: %v", err)
		}
		if decoded.TotalCPU != 9.5 || decoded.AlertLevel != core.AlertLevelNormal {
			t.Fatalf("decoded state = totalCPU:%v alert:%q, want 9.5/%q", decoded.TotalCPU, decoded.AlertLevel, core.AlertLevelNormal)
		}
		if decoded.Preferences.GlobalCPUPercent == nil || *decoded.Preferences.GlobalCPUPercent != budget {
			t.Fatalf("native bridge lost global CPU budget: %+v", decoded.Preferences)
		}
	}

	for _, managementEnabled := range []bool{false, true} {
		for _, limiterEnabled := range []bool{false, true} {
			state.Enabled = managementEnabled
			state.Preferences.ManagementEnabled = managementEnabled
			state.Preferences.CPULimiterEnabled = limiterEnabled
			if err := UpdateMenuBarState(state); err != nil {
				t.Fatal(err)
			}
			var decoded ui.MenuBarViewState
			if err := json.Unmarshal([]byte(payloads[len(payloads)-1]), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Enabled != managementEnabled || decoded.Preferences.ManagementEnabled != managementEnabled || decoded.Preferences.CPULimiterEnabled != limiterEnabled {
				t.Fatalf("native bridge lost enable flags: %+v", decoded)
			}
			if decoded.Preferences.GlobalCPUPercent == nil || *decoded.Preferences.GlobalCPUPercent != budget {
				t.Fatal("native bridge must preserve saved budget even when inactive")
			}
		}
	}
}

func TestTerminateMenuBarAppCallsNativeBridge(t *testing.T) {
	old := menuBarNative
	t.Cleanup(func() {
		menuBarNative = old
	})

	calls := 0
	menuBarNative.terminate = func() {
		calls++
	}

	TerminateMenuBarApp()
	if calls != 1 {
		t.Fatalf("terminate calls = %d, want 1", calls)
	}
}

func TestMenuCommandHandlerDispatchesCommand(t *testing.T) {
	t.Cleanup(func() {
		SetMenuCommandHandler(nil)
	})

	var got string
	SetMenuCommandHandler(func(command string) {
		got = command
	})

	for _, command := range []string{"pref-bool|showMenuBarIcon|false", "pref-float|globalCPUPercent|62.5", "pref-clear|globalCPUPercent"} {
		dispatchMenuCommand(command)
		if got != command {
			t.Fatalf("dispatched command = %q, want %q", got, command)
		}
	}

	SetMenuCommandHandler(nil)
	dispatchMenuCommand("quit")
}
