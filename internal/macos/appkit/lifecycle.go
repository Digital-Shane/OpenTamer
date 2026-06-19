package appkit

import "github.com/Digital-Shane/open-tamer/internal/core"

type AppKitLifecycleController struct {
	Bridge AppKitController
}

func NewLifecycleController(bridge *AppKitBridge) *AppKitLifecycleController {
	if bridge == nil {
		bridge = NewAppKitBridge()
	}
	return &AppKitLifecycleController{Bridge: bridge}
}

func (controller *AppKitLifecycleController) Hide(process core.ProcessRef) error {
	return controller.Bridge.Hide(process.ID.PID)
}

func (controller *AppKitLifecycleController) Activate(process core.ProcessRef) error {
	return controller.Bridge.Activate(process.ID.PID)
}

func (controller *AppKitLifecycleController) Terminate(process core.ProcessRef) error {
	return controller.Bridge.Terminate(process.ID.PID)
}
