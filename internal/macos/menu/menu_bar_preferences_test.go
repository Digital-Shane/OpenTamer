package menu

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestPreferencesMenuExposesExpectedConfigKeys(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	keyPattern := regexp.MustCompile(`(?s)(?:boolPreferenceItemWithTitle|addDurationPreferenceWithTitle|addFloatPreferenceWithTitle|floatPreferencePresetItemWithTitle|addStringPreferenceWithTitle|addNullableFloatPreferenceWithTitle).*?key:@"([^"]+)"`)
	matches := keyPattern.FindAllStringSubmatch(source, -1)
	keys := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		key := match[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)

	expected := []string{
		"aggregateByName",
		"averagingWindow",
		"cpuDisplayMode",
		"cpuGraphWindow",
		"disableWhenACBatteryAbove",
		"disableWhenUserIdleLongerThan",
		"highCPUDetectionEnabled",
		"highCPUCooldown",
		"highCPUDuration",
		"highCPUThreshold",
		"showMenuBarIcon",
		"statsInterval",
		"topProcessesSort",
		"wakeGrace",
	}
	sort.Strings(expected)

	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("preference keys = %#v, want %#v", keys, expected)
	}
	if !strings.Contains(source, `command:@"reset-defaults"`) {
		t.Fatal("preferences menu should expose reset defaults")
	}
	if strings.Contains(source, `[command isEqualToString:@"reset-defaults"]`) {
		t.Fatal("reset defaults should use a native menu item so it remains clickable and aligned")
	}
	if !strings.Contains(source, `"sort-current"`) || !strings.Contains(source, `"pref-string|topProcessesSort|current"`) {
		t.Fatal("CPU header should select current top-process sorting")
	}
	if !strings.Contains(source, `"sort-average"`) || !strings.Contains(source, `"pref-string|topProcessesSort|average"`) {
		t.Fatal("Avg header should select average top-process sorting")
	}
	if !strings.Contains(source, `key:@"cpuDisplayMode"`) ||
		!strings.Contains(source, `@"per_core_process"`) ||
		!strings.Contains(source, `@"system_normalized"`) {
		t.Fatal("preferences menu should expose CPU display mode choices")
	}
	if strings.Contains(source, "preferencesSummary") {
		t.Fatal("preferences menu should not expose disabled summary rows")
	}
}

func TestManagementPreferencesMovedIntoGeneral(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	if strings.Contains(source, `initWithTitle:@"Management"`) ||
		strings.Contains(source, `"Management Enabled"`) ||
		strings.Contains(source, `"CPU Limiter Enabled"`) {
		t.Fatal("preferences menu should not expose the old Management submenu or toggles")
	}
	generalStart := strings.Index(source, `initWithTitle:@"General"`)
	statsStart := strings.Index(source, `initWithTitle:@"Stats & Graph"`)
	if generalStart < 0 || statsStart < 0 || generalStart >= statsStart {
		t.Fatal("preferences menu should build General before Stats & Graph")
	}
	generalBlock := source[generalStart:statsStart]
	for _, entry := range []string{`launchAtLoginItem`, `key:@"wakeGrace"`, `key:@"cpuDisplayMode"`} {
		if !strings.Contains(generalBlock, entry) {
			t.Fatalf("General preferences should include %s", entry)
		}
	}
	if strings.Contains(generalBlock, `key:@"startupGrace"`) {
		t.Fatal("General preferences should expose one wake grace control, not legacy startupGrace")
	}
}

func TestLaunchAtLoginPreferenceUsesServiceManagement(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	for _, entry := range []string{
		`#import <ServiceManagement/ServiceManagement.h>`,
		`SMAppService.mainAppService`,
		`registerAndReturnError`,
		`unregisterAndReturnError`,
		`Launch at Login`,
	} {
		if !strings.Contains(source, entry) {
			t.Fatalf("launch-at-login preference should include %s", entry)
		}
	}
}

func TestHighCPUAlertThresholdMenuHasCustomEntry(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	if strings.Contains(source, `@"150%"`) || strings.Contains(source, `@150`) {
		t.Fatal("high CPU alert threshold should not expose the old 150% preset")
	}
	for _, preset := range []string{`@"50%"`, `@"75%"`, `@"90%"`, `@"100%"`} {
		if !strings.Contains(source, preset) {
			t.Fatalf("high CPU alert threshold should expose preset %s", preset)
		}
	}
	if !strings.Contains(source, `@selector(promptForHighCPUThreshold:)`) ||
		!strings.Contains(source, `pref-float|highCPUThreshold|%.6g`) {
		t.Fatal("high CPU alert threshold should expose a custom entry that saves through the float preference command")
	}
}

func TestPrimaryPanelExposesQuitShortcut(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	if !strings.Contains(source, `keyEquivalent:@"q"`) {
		t.Fatal("preferences menu should expose Command-Q via the quit menu item")
	}
	if !strings.Contains(source, "performKeyEquivalent:") ||
		!strings.Contains(source, "NSEventModifierFlagCommand") ||
		!strings.Contains(source, "charactersIgnoringModifiers") ||
		!strings.Contains(source, "performPanelQuit") {
		t.Fatal("primary panel should handle Command-Q while the popover is open")
	}
	if !strings.Contains(source, "makeFirstResponder:viewController.view") {
		t.Fatal("primary panel should become first responder when shown or refreshed")
	}
	if !strings.Contains(source, "[self quit:nil]") {
		t.Fatal("primary panel shortcut should route through the normal quit cleanup path")
	}
}

func TestPreferenceCommandSubmenusStayOpenWhileProcessCommandsClose(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	if !strings.Contains(source, "OpenTamerCommandMenuItemView") ||
		!strings.Contains(source, "item.view = view") ||
		!strings.Contains(source, "@selector(sendPersistentCommand:)") ||
		!strings.Contains(source, "view.menuItem") {
		t.Fatal("persistent commands should be custom menu-item views instead of normal closing menu actions")
	}
	if !strings.Contains(source, "self.autoenablesItems = NO") ||
		!strings.Contains(source, "item.enabled = YES") {
		t.Fatal("persistent command menus should preserve explicit enabled state while the menu remains open")
	}
	for _, prefix := range []string{`[command hasPrefix:@"pref-"]`, `[command hasPrefix:@"graph-window|"]`} {
		if !strings.Contains(source, prefix) {
			t.Fatalf("persistent menus should keep %s actions open", prefix)
		}
	}
	for _, prefix := range []string{`[command hasPrefix:@"rule|"]`, `[command hasPrefix:@"disable-rule|"]`} {
		if strings.Contains(source, prefix) {
			t.Fatalf("process command prefix %s should close its submenu after selection", prefix)
		}
	}
	if !strings.Contains(source, `action:keepMenuOpen ? nil : @selector(sendCommand:)`) {
		t.Fatal("non-persistent process commands should use native menu actions so submenus close")
	}
	if !strings.Contains(source, "[NSApp sendAction:self.action to:self.target from:self]") {
		t.Fatal("persistent command views should execute commands directly from mouseDown")
	}
	if !strings.Contains(source, "updateMenuItem:item afterCommand:command") ||
		!strings.Contains(source, "exclusiveForCommandPrefix") {
		t.Fatal("open command menus should update local item state after actions")
	}
}

func TestPreferenceOffRowsUpdateImmediatelyWithSiblingChoices(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	if !strings.Contains(source, "setPreferenceOffItemChecked:NO forKey:key inMenu:item.menu") {
		t.Fatal("selecting a typed preference value should immediately uncheck the sibling Off row")
	}
	if !strings.Contains(source, "clearPreferencePresetItemsForKey:key inMenu:item.menu") {
		t.Fatal("selecting a preference Off row should immediately uncheck sibling preset values")
	}
	if !strings.Contains(source, "[command isEqualToString:clearCommand]") {
		t.Fatal("preference Off row updates should match the exact pref-clear command for the key")
	}
	for _, prefix := range []string{`pref-duration|%@|`, `pref-float|%@|`, `pref-string|%@|`} {
		if !strings.Contains(source, prefix) {
			t.Fatalf("Off row sibling clearing should include %s typed preferences", prefix)
		}
	}
	if strings.Contains(source, "NSString *floatPrefix") {
		t.Fatal("Off row sibling clearing should not be limited to float preference menus")
	}
}

func TestProcessMenusUseSelectedCPUDisplayMode(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)

	for _, expected := range []string{
		"OpenTamerDisplayCPUFromRow",
		"OpenTamerAverageDisplayCPUFromRow",
		`@"Process CPU"`,
		`@"System CPU"`,
		"topProcessesCPULabel",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("process menus should use selected CPU display mode; missing %s", expected)
		}
	}
	if strings.Contains(source, "current system CPU") {
		t.Fatal("process section headers should not hard-code system CPU")
	}
}
