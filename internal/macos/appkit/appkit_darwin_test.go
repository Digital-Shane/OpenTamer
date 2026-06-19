//go:build darwin && cgo

package appkit

import (
	"errors"
	"testing"
	"time"
)

func TestAppKitBridgeRunningAppsDecodesNativePayload(t *testing.T) {
	old := appKitNative
	t.Cleanup(func() {
		appKitNative = old
	})

	appKitNative.runningApps = func() (string, error) {
		return `[{"pid":42,"bundleID":"com.example.Preview","localizedName":"Preview","executablePath":"/Applications/Preview.app/Contents/MacOS/Preview","bundlePath":"/Applications/Preview.app","activationPolicy":0,"active":true}]`, nil
	}

	apps, err := NewAppKitBridge().RunningApps()
	if err != nil {
		t.Fatalf("running apps: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("apps len = %d, want 1", len(apps))
	}
	if apps[0].PID != 42 || apps[0].BundleID != "com.example.Preview" || apps[0].ObservedAt.IsZero() {
		t.Fatalf("decoded app = %#v", apps[0])
	}
}

func TestAppKitBridgeFrontmostAppPropagatesNativeError(t *testing.T) {
	old := appKitNative
	t.Cleanup(func() {
		appKitNative = old
	})

	want := errors.New("frontmost application unavailable")
	appKitNative.frontmostApp = func() (string, error) {
		return "", want
	}

	_, err := NewAppKitBridge().FrontmostApp()
	if !errors.Is(err, want) {
		t.Fatalf("frontmost app error = %v, want %v", err, want)
	}
}

func TestAppKitBridgeActionsForwardPIDsAndErrors(t *testing.T) {
	old := appKitNative
	t.Cleanup(func() {
		appKitNative = old
	})

	want := errors.New("activate failed")
	var calls []struct {
		action string
		pid    int
	}
	appKitNative.hide = func(pid int) error {
		calls = append(calls, struct {
			action string
			pid    int
		}{"hide", pid})
		return nil
	}
	appKitNative.activate = func(pid int) error {
		calls = append(calls, struct {
			action string
			pid    int
		}{"activate", pid})
		return want
	}
	appKitNative.terminate = func(pid int) error {
		calls = append(calls, struct {
			action string
			pid    int
		}{"terminate", pid})
		return nil
	}

	bridge := NewAppKitBridge()
	if err := bridge.Hide(11); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if err := bridge.Activate(22); !errors.Is(err, want) {
		t.Fatalf("activate error = %v, want %v", err, want)
	}
	if err := bridge.Terminate(33); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	wantCalls := []struct {
		action string
		pid    int
	}{
		{"hide", 11},
		{"activate", 22},
		{"terminate", 33},
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
		}
	}
}

func TestAppKitBridgeEventsStartsObserverAndPublishesDecodedEvents(t *testing.T) {
	old := appKitNative
	t.Cleanup(func() {
		appKitNative = old
	})
	clearAppKitEventSubscribers(t)

	starts := 0
	appKitNative.startWorkspaceObserver = func() {
		starts++
	}

	events := NewAppKitBridge().Events()
	if starts != 1 {
		t.Fatalf("workspace observer starts = %d, want 1", starts)
	}

	if !publishAppEventPayload(`{"kind":"activated","app":{"pid":44,"localizedName":"Preview"}}`) {
		t.Fatal("valid event payload was rejected")
	}
	select {
	case event := <-events:
		if event.Kind != AppEventActivated || event.App.PID != 44 || event.At.IsZero() {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("published event was not delivered")
	}

	if publishAppEventPayload(`{`) {
		t.Fatal("invalid event payload was accepted")
	}
}

func TestEventHubPublishDoesNotBlockWhenSubscriberIsFull(t *testing.T) {
	hub := &eventHub{}
	ch := make(chan AppEvent, 1)
	hub.add(ch)
	hub.publish(AppEvent{Kind: AppEventActivated})

	done := make(chan struct{})
	go func() {
		hub.publish(AppEvent{Kind: AppEventTerminated})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on a full subscriber channel")
	}
}

func TestCopyCStringRejectsNilAndBoolResultMapsNativeStatus(t *testing.T) {
	if _, err := copyCString(nil); err == nil {
		t.Fatal("copyCString(nil) error = nil")
	}
	if err := appKitBoolResult(1); err != nil {
		t.Fatalf("success result error = %v", err)
	}
	if err := appKitBoolResult(0); err == nil {
		t.Fatal("failure result error = nil")
	}
}

func clearAppKitEventSubscribers(t *testing.T) {
	t.Helper()

	appKitEventHub.mu.Lock()
	oldSubscribers := appKitEventHub.subscribers
	appKitEventHub.subscribers = nil
	appKitEventHub.mu.Unlock()

	t.Cleanup(func() {
		appKitEventHub.mu.Lock()
		appKitEventHub.subscribers = oldSubscribers
		appKitEventHub.mu.Unlock()
	})
}
