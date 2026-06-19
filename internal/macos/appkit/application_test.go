package appkit

import (
	"errors"
	"testing"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestApplicationObserverBuildsProcessHints(t *testing.T) {
	observer := NewApplicationObserver(&fakeAppKitObserver{
		apps: []RunningApp{{
			PID:            101,
			BundleID:       "com.example.Preview",
			LocalizedName:  "Preview",
			ExecutablePath: "/Applications/Preview.app/Contents/MacOS/Preview",
			BundlePath:     "/Applications/Preview.app",
		}},
	})

	hints, err := observer.AppProcessHints()
	if err != nil {
		t.Fatalf("app process hints: %v", err)
	}
	if len(hints) != 1 {
		t.Fatalf("hints len = %d, want 1", len(hints))
	}
	if hints[0].PrimaryPID.PID != 101 {
		t.Fatalf("primary pid = %d, want 101", hints[0].PrimaryPID.PID)
	}
	if hints[0].AppID.BundleID != "com.example.Preview" {
		t.Fatalf("bundle id = %q", hints[0].AppID.BundleID)
	}
	if hints[0].BundlePath != "/Applications/Preview.app" {
		t.Fatalf("bundle path = %q", hints[0].BundlePath)
	}
}

func TestApplicationObserverReturnsFrontmostAppID(t *testing.T) {
	observer := NewApplicationObserver(&fakeAppKitObserver{
		frontmost: RunningApp{
			PID:           202,
			BundleID:      "com.example.Terminal",
			LocalizedName: "Terminal",
			BundlePath:    "/Applications/Utilities/Terminal.app",
		},
	})

	appID, err := observer.FrontmostAppID()
	if err != nil {
		t.Fatalf("frontmost app id: %v", err)
	}
	if appID != (core.AppID{BundleID: "com.example.Terminal", Path: "/Applications/Utilities/Terminal.app", Name: "Terminal"}) {
		t.Fatalf("app id = %#v", appID)
	}
}

func TestApplicationObserverPropagatesBridgeErrors(t *testing.T) {
	want := errors.New("appkit unavailable")
	observer := NewApplicationObserver(&fakeAppKitObserver{
		runningErr:   want,
		frontmostErr: want,
	})

	if _, err := observer.AppProcessHints(); !errors.Is(err, want) {
		t.Fatalf("running apps error = %v, want %v", err, want)
	}
	if _, err := observer.FrontmostAppID(); !errors.Is(err, want) {
		t.Fatalf("frontmost app error = %v, want %v", err, want)
	}
}

type fakeAppKitObserver struct {
	apps         []RunningApp
	runningErr   error
	frontmost    RunningApp
	frontmostErr error
	events       chan AppEvent
}

func (observer *fakeAppKitObserver) RunningApps() ([]RunningApp, error) {
	return observer.apps, observer.runningErr
}

func (observer *fakeAppKitObserver) FrontmostApp() (RunningApp, error) {
	return observer.frontmost, observer.frontmostErr
}

func (observer *fakeAppKitObserver) Events() <-chan AppEvent {
	return observer.events
}
