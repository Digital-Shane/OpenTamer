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

	state := ui.MenuBarViewState{
		Enabled:    true,
		TotalCPU:   9.5,
		AlertLevel: core.AlertLevelNormal,
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

	dispatchMenuCommand("pref-bool|showMenuBarIcon|false")
	if got != "pref-bool|showMenuBarIcon|false" {
		t.Fatalf("dispatched command = %q", got)
	}

	SetMenuCommandHandler(nil)
	dispatchMenuCommand("quit")
}
