package appkit

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

var ErrUnsupported = errors.New("macOS AppKit bridge is unsupported on this platform")

type AppEventKind string

const (
	AppEventLaunched   AppEventKind = "launched"
	AppEventTerminated AppEventKind = "terminated"
	AppEventActivated  AppEventKind = "activated"
	AppEventHidden     AppEventKind = "hidden"
	AppEventUnhidden   AppEventKind = "unhidden"
)

type RunningApp struct {
	PID              int       `json:"pid"`
	BundleID         string    `json:"bundleID,omitempty"`
	LocalizedName    string    `json:"localizedName,omitempty"`
	ExecutablePath   string    `json:"executablePath,omitempty"`
	BundlePath       string    `json:"bundlePath,omitempty"`
	ActivationPolicy int       `json:"activationPolicy"`
	Active           bool      `json:"active"`
	Hidden           bool      `json:"hidden"`
	Terminated       bool      `json:"terminated"`
	ObservedAt       time.Time `json:"observedAt"`
}

func (app RunningApp) AppID() core.AppID {
	return core.AppID{
		BundleID: app.BundleID,
		Path:     app.BundlePath,
		Name:     app.LocalizedName,
	}
}

func (app RunningApp) ProcessRef() core.ProcessRef {
	return core.ProcessRef{
		ID:             core.ProcessID{PID: app.PID},
		Name:           app.LocalizedName,
		ExecutablePath: app.ExecutablePath,
		BundleID:       app.BundleID,
	}
}

func (app RunningApp) AppProcessHint() core.AppProcessHint {
	return core.AppProcessHint{
		AppID:          app.AppID(),
		PrimaryPID:     core.ProcessID{PID: app.PID},
		ExecutablePath: app.ExecutablePath,
		BundlePath:     app.BundlePath,
	}
}

func AppProcessHints(apps []RunningApp) []core.AppProcessHint {
	hints := make([]core.AppProcessHint, 0, len(apps))
	for _, app := range apps {
		hints = append(hints, app.AppProcessHint())
	}
	return hints
}

type AppEvent struct {
	Kind AppEventKind `json:"kind"`
	App  RunningApp   `json:"app"`
	At   time.Time    `json:"at"`
}

type AppKitObserver interface {
	RunningApps() ([]RunningApp, error)
	FrontmostApp() (RunningApp, error)
	Events() <-chan AppEvent
}

type AppKitController interface {
	Hide(pid int) error
	Activate(pid int) error
	Terminate(pid int) error
}

func decodeRunningAppsJSON(payload string) ([]RunningApp, error) {
	var apps []RunningApp
	if err := json.Unmarshal([]byte(payload), &apps); err != nil {
		return nil, err
	}
	observedAt := time.Now()
	for i := range apps {
		if apps[i].ObservedAt.IsZero() {
			apps[i].ObservedAt = observedAt
		}
	}
	return apps, nil
}

func decodeRunningAppJSON(payload string) (RunningApp, error) {
	var app RunningApp
	if err := json.Unmarshal([]byte(payload), &app); err != nil {
		return RunningApp{}, err
	}
	if app.ObservedAt.IsZero() {
		app.ObservedAt = time.Now()
	}
	return app, nil
}

func decodeAppEventJSON(payload string) (AppEvent, error) {
	var event AppEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return AppEvent{}, err
	}
	if event.At.IsZero() {
		event.At = time.Now()
	}
	return event, nil
}
