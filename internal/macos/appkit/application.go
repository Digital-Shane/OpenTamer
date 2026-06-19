package appkit

import "github.com/Digital-Shane/open-tamer/internal/core"

type ApplicationObserver struct {
	appKit AppKitObserver
}

func NewApplicationObserver(appKit AppKitObserver) *ApplicationObserver {
	return &ApplicationObserver{appKit: appKit}
}

func (observer *ApplicationObserver) AppProcessHints() ([]core.AppProcessHint, error) {
	apps, err := observer.appKit.RunningApps()
	if err != nil {
		return nil, err
	}
	return AppProcessHints(apps), nil
}

func (observer *ApplicationObserver) FrontmostAppID() (core.AppID, error) {
	app, err := observer.appKit.FrontmostApp()
	if err != nil {
		return core.AppID{}, err
	}
	return app.AppID(), nil
}
