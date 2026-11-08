package menu

import (
	"os"
	"strings"
	"testing"
)

func TestQuitDelegatesTerminationToRecoveryController(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatal(err)
	}
	source := string(payload)
	start := strings.Index(source, "- (void)quit:(id)sender {")
	if start < 0 {
		t.Fatal("missing quit handler")
	}
	end := strings.Index(source[start:], "\n}")
	if end < 0 {
		t.Fatal("missing end of quit handler")
	}
	handler := source[start : start+end]
	if !strings.Contains(handler, `opentamer_menu_command("quit")`) || strings.Contains(handler, "terminate:") {
		t.Fatal("quit must wait for the controller to recover stopped processes")
	}
}
