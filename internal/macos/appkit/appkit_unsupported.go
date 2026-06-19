//go:build !darwin || !cgo

package appkit

type AppKitBridge struct{}

func NewAppKitBridge() *AppKitBridge {
	return &AppKitBridge{}
}

func (bridge *AppKitBridge) RunningApps() ([]RunningApp, error) {
	return nil, ErrUnsupported
}

func (bridge *AppKitBridge) FrontmostApp() (RunningApp, error) {
	return RunningApp{}, ErrUnsupported
}

func (bridge *AppKitBridge) Events() <-chan AppEvent {
	ch := make(chan AppEvent)
	close(ch)
	return ch
}

func (bridge *AppKitBridge) Hide(pid int) error {
	return ErrUnsupported
}

func (bridge *AppKitBridge) Activate(pid int) error {
	return ErrUnsupported
}

func (bridge *AppKitBridge) Terminate(pid int) error {
	return ErrUnsupported
}
