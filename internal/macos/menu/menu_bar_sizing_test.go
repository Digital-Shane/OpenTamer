package menu

import (
	"os"
	"strings"
	"testing"
)

func TestMainStatusItemUsesIntrinsicContentWidth(t *testing.T) {
	payload, err := os.ReadFile("menu_bar_bridge.m")
	if err != nil {
		t.Fatalf("read menu bridge: %v", err)
	}
	source := string(payload)
	if !strings.Contains(source, "self.statusItem = [NSStatusBar.systemStatusBar statusItemWithLength:NSVariableStatusItemLength]") {
		t.Fatal("main status item should use native variable sizing")
	}
	if strings.Contains(source, "self.statusItem.length =") || strings.Contains(source, "OpenTamerStatusItemLength") {
		t.Fatal("main status item should not override native sizing with fixed widths")
	}

	start := strings.Index(source, "- (void)updateStatusTitle {")
	if start < 0 {
		t.Fatal("missing main status title update")
	}
	end := strings.Index(source[start:], "\n}\n")
	if end < 0 {
		t.Fatal("missing end of main status title update")
	}
	update := source[start : start+end]
	for _, expected := range []string{
		"button.image = showIcon ? [self openTamerStatusIcon] : nil;",
		"button.imagePosition = showIcon ? NSImageLeft : NSNoImage;",
		`title = @"Off";`,
		`title = [NSString stringWithFormat:@"! %@", [self formattedPercent:[self totalCPU]]];`,
		"title = [self formattedPercent:[self totalCPU]];",
		"button.attributedTitle = [self statusTitleWithText:title];",
	} {
		if !strings.Contains(update, expected) {
			t.Fatalf("native content sizing should preserve status content: missing %s", expected)
		}
	}
}
