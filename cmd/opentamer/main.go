//go:build darwin && cgo

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/macos"
	"github.com/Digital-Shane/open-tamer/internal/macos/menu"
)

func main() {
	runtime.LockOSThread()

	if !launchedByLaunchServices() && os.Getenv("OPENTAMER_ALLOW_DIRECT_LAUNCH") == "" {
		log.Print("OpenTamer must be launched as a macOS app bundle. Use `open build/OpenTamer.app` or set OPENTAMER_ALLOW_DIRECT_LAUNCH=1 for unsupported debugging.")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dir, err := config.DefaultDir()
	if err != nil {
		log.Fatal(err)
	}
	controller, err := app.NewController(config.NewStore(dir), menu.MenuBarUpdater{}, macos.NewAdapters())
	if err != nil {
		log.Fatal(err)
	}
	menu.SetMenuCommandHandler(controller.HandleMenuCommand)
	controller.Start(ctx)
	go func() {
		<-ctx.Done()
		controller.Shutdown(core.ControlReasonGlobalDisabled)
		menu.TerminateMenuBarApp()
	}()

	if err := menu.RunMenuBarApp(controller.InitialState()); err != nil {
		log.Fatal(err)
	}
}

func launchedByLaunchServices() bool {
	return os.Getenv("__CFBundleIdentifier") == "org.opentamer.OpenTamer"
}
