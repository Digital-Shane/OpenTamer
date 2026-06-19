package macos

import (
	"github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/macos/appkit"
	"github.com/Digital-Shane/open-tamer/internal/macos/audio"
	"github.com/Digital-Shane/open-tamer/internal/macos/control"
	"github.com/Digital-Shane/open-tamer/internal/macos/metrics"
	"github.com/Digital-Shane/open-tamer/internal/macos/notify"
	"github.com/Digital-Shane/open-tamer/internal/macos/policy"
	"github.com/Digital-Shane/open-tamer/internal/macos/process"
)

func NewAdapters() app.Adapters {
	appKit := appkit.NewAppKitBridge()
	signals := control.NewSignalController()
	return app.Adapters{
		Applications: appkit.NewApplicationObserver(appKit),
		Processes:    process.NewProcessEnumerator(),
		Metrics:      metrics.NewMetricsSampler(),
		Audio:        audio.NewAudioObserver(),
		SystemPolicy: policy.NewSystemPolicyObserver(),
		Signals:      signals,
		Priority:     control.NewPriorityController(),
		Lifecycle:    appkit.NewLifecycleController(appKit),
		Notifier:     notify.NewUserNotifier(),
	}
}
