//go:build !darwin || !cgo

package menu

import "github.com/Digital-Shane/open-tamer/internal/ui"

func RunMenuBarApp(initialState ui.MenuBarViewState) error {
	return ErrUnsupported
}

func UpdateMenuBarState(state ui.MenuBarViewState) error {
	return ErrUnsupported
}

func TerminateMenuBarApp() {}

func SetMenuCommandHandler(handler func(string)) {}

type MenuBarUpdater struct{}

func (updater MenuBarUpdater) UpdateMenuBarState(state ui.MenuBarViewState) error {
	return ErrUnsupported
}
