//go:build darwin && cgo

package notify

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/app"
)

func TestNotifyHighCPURejectsNilNotifier(t *testing.T) {
	var notifier *UserNotifier
	err := notifier.NotifyHighCPU(app.HighCPUNotice{AppName: "Preview"})
	if err == nil || !strings.Contains(err.Error(), "notifier is not configured") {
		t.Fatalf("nil notifier error = %v", err)
	}
}

func TestNotifyHighCPUReturnsQueueFullWhenNativeDeliveryBacksUp(t *testing.T) {
	notifier := &UserNotifier{queue: make(chan app.HighCPUNotice)}

	err := notifier.NotifyHighCPU(app.HighCPUNotice{AppName: "Preview"})
	if err == nil || !strings.Contains(err.Error(), "notification queue full") {
		t.Fatalf("queue full error = %v", err)
	}
}

func TestDeliverHighCPUNotificationFormatsNativePayload(t *testing.T) {
	old := deliverNotification
	t.Cleanup(func() {
		deliverNotification = old
	})

	var gotTitle, gotBody string
	deliverNotification = func(title, body string) error {
		gotTitle = title
		gotBody = body
		return nil
	}

	if err := deliverHighCPUNotification(app.HighCPUNotice{AppName: "Preview", CPUPercent: 87.6}); err != nil {
		t.Fatalf("deliver high CPU notification: %v", err)
	}
	if gotTitle != "High CPU usage" {
		t.Fatalf("title = %q, want High CPU usage", gotTitle)
	}
	if gotBody != "Preview is using 88% CPU." {
		t.Fatalf("body = %q, want rounded CPU body", gotBody)
	}
}

func TestDeliverHighCPUNotificationPropagatesNativeError(t *testing.T) {
	old := deliverNotification
	t.Cleanup(func() {
		deliverNotification = old
	})

	want := errors.New("authorization denied")
	deliverNotification = func(title, body string) error {
		return want
	}

	err := deliverHighCPUNotification(app.HighCPUNotice{AppName: "Preview", CPUPercent: 87.6})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestUserNotifierDeliversQueuedHighCPUNotice(t *testing.T) {
	old := deliverNotification
	t.Cleanup(func() {
		deliverNotification = old
	})

	delivered := make(chan string, 1)
	deliverNotification = func(title, body string) error {
		delivered <- title + "\n" + body
		return nil
	}

	notifier := NewUserNotifier()
	t.Cleanup(func() {
		close(notifier.queue)
	})

	if err := notifier.NotifyHighCPU(app.HighCPUNotice{AppName: "Preview", CPUPercent: 87.6}); err != nil {
		t.Fatalf("notify high CPU: %v", err)
	}

	select {
	case payload := <-delivered:
		if payload != "High CPU usage\nPreview is using 88% CPU." {
			t.Fatalf("payload = %q", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("queued notification was not delivered")
	}
}
