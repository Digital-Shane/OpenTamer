//go:build darwin && cgo

package appkit

/*
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "appkit_bridge.h"
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

type AppKitBridge struct{}

type appKitNativeBridge struct {
	runningApps            func() (string, error)
	frontmostApp           func() (string, error)
	startWorkspaceObserver func()
	hide                   func(int) error
	activate               func(int) error
	terminate              func(int) error
}

var appKitNative = appKitNativeBridge{
	runningApps: func() (string, error) {
		return copyCString(C.OpenTamerCopyRunningApplicationsJSON())
	},
	frontmostApp: func() (string, error) {
		return copyCString(C.OpenTamerCopyFrontmostApplicationJSON())
	},
	startWorkspaceObserver: func() {
		C.OpenTamerStartWorkspaceObserver()
	},
	hide: func(pid int) error {
		return appKitBoolResult(int(C.OpenTamerHideApplication(C.int(pid))))
	},
	activate: func(pid int) error {
		return appKitBoolResult(int(C.OpenTamerActivateApplication(C.int(pid))))
	},
	terminate: func(pid int) error {
		return appKitBoolResult(int(C.OpenTamerTerminateApplication(C.int(pid))))
	},
}

func NewAppKitBridge() *AppKitBridge {
	return &AppKitBridge{}
}

func (bridge *AppKitBridge) RunningApps() ([]RunningApp, error) {
	payload, err := appKitNative.runningApps()
	if err != nil {
		return nil, err
	}
	return decodeRunningAppsJSON(payload)
}

func (bridge *AppKitBridge) FrontmostApp() (RunningApp, error) {
	payload, err := appKitNative.frontmostApp()
	if err != nil {
		return RunningApp{}, err
	}
	return decodeRunningAppJSON(payload)
}

func (bridge *AppKitBridge) Events() <-chan AppEvent {
	ch := make(chan AppEvent, 64)
	appKitEventHub.add(ch)
	appKitNative.startWorkspaceObserver()
	return ch
}

func (bridge *AppKitBridge) Hide(pid int) error {
	return appKitNative.hide(pid)
}

func (bridge *AppKitBridge) Activate(pid int) error {
	return appKitNative.activate(pid)
}

func (bridge *AppKitBridge) Terminate(pid int) error {
	return appKitNative.terminate(pid)
}

func copyCString(value *C.char) (string, error) {
	if value == nil {
		return "", errors.New("AppKit bridge returned nil")
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value), nil
}

func appKitBoolResult(result int) error {
	if result == 1 {
		return nil
	}
	return errors.New("AppKit action failed")
}

type eventHub struct {
	mu          sync.Mutex
	subscribers []chan AppEvent
}

var appKitEventHub eventHub

func init() {
	opentamer_app_event(nil)
}

func (hub *eventHub) add(ch chan AppEvent) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.subscribers = append(hub.subscribers, ch)
}

func (hub *eventHub) publish(event AppEvent) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for _, ch := range hub.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func publishAppEventPayload(payload string) bool {
	event, err := decodeAppEventJSON(payload)
	if err != nil {
		return false
	}
	appKitEventHub.publish(event)
	return true
}

//export opentamer_app_event
func opentamer_app_event(payload *C.char) {
	if payload == nil {
		return
	}
	publishAppEventPayload(C.GoString(payload))
}
