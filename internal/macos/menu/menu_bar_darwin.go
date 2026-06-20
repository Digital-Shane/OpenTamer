//go:build darwin && cgo

package menu

/*
#cgo darwin LDFLAGS: -framework Cocoa -framework ServiceManagement
#include <stdlib.h>
#include "menu_bar_bridge.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"sync"
	"unsafe"

	"github.com/Digital-Shane/open-tamer/internal/ui"
)

var menuCommandHub struct {
	sync.Mutex
	handler func(string)
}

type menuBarNativeBridge struct {
	run       func(string)
	update    func(string)
	terminate func()
}

var menuBarNative = menuBarNativeBridge{
	run:       runMenuBarAppNative,
	update:    updateMenuBarStateNative,
	terminate: terminateMenuBarAppNative,
}

func RunMenuBarApp(initialState ui.MenuBarViewState) error {
	payload, err := json.Marshal(initialState)
	if err != nil {
		return fmt.Errorf("encode menu bar state: %w", err)
	}

	menuBarNative.run(string(payload))
	return nil
}

func UpdateMenuBarState(state ui.MenuBarViewState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode menu bar state: %w", err)
	}
	menuBarNative.update(string(payload))
	return nil
}

func TerminateMenuBarApp() {
	menuBarNative.terminate()
}

func SetMenuCommandHandler(handler func(string)) {
	menuCommandHub.Lock()
	defer menuCommandHub.Unlock()
	menuCommandHub.handler = handler
}

type MenuBarUpdater struct{}

func (updater MenuBarUpdater) UpdateMenuBarState(state ui.MenuBarViewState) error {
	return UpdateMenuBarState(state)
}

func init() {
	opentamer_menu_command(nil)
}

func runMenuBarAppNative(payload string) {
	cPayload := C.CString(payload)
	defer C.free(unsafe.Pointer(cPayload))
	C.OpenTamerRunMenuBarApp(cPayload)
}

func updateMenuBarStateNative(payload string) {
	cPayload := C.CString(payload)
	defer C.free(unsafe.Pointer(cPayload))
	C.OpenTamerUpdateMenuBarJSON(cPayload)
}

func terminateMenuBarAppNative() {
	C.OpenTamerTerminateMenuBarApp()
}

func dispatchMenuCommand(command string) {
	menuCommandHub.Lock()
	handler := menuCommandHub.handler
	menuCommandHub.Unlock()
	if handler != nil {
		handler(command)
	}
}

//export opentamer_menu_command
func opentamer_menu_command(command *C.char) {
	if command == nil {
		return
	}
	dispatchMenuCommand(C.GoString(command))
}
