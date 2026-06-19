package notify

import (
	"os"
	"strings"
	"testing"

	"github.com/Digital-Shane/open-tamer/internal/app"
)

func TestUserNotifierSatisfiesNotifierInterface(t *testing.T) {
	var _ app.Notifier = (*UserNotifier)(nil)
}

func TestNotificationBridgePresentsForegroundNotifications(t *testing.T) {
	source, err := os.ReadFile("notifications_bridge.m")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"UNUserNotificationCenterDelegate",
		"willPresentNotification",
		"UNNotificationPresentationOptionBanner",
		"center.delegate",
		"dispatch_semaphore_wait",
		"char **error_out",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("notifications_bridge.m missing %q", want)
		}
	}
	if strings.Contains(text, "NSLog") {
		t.Fatal("notifications_bridge.m should route diagnostics through Go instead of NSLog")
	}
}
