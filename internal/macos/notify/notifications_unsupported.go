//go:build !darwin || !cgo

package notify

import "github.com/Digital-Shane/open-tamer/internal/app"

type UserNotifier struct{}

func NewUserNotifier() *UserNotifier {
	return &UserNotifier{}
}

func (notifier *UserNotifier) NotifyHighCPU(notice app.HighCPUNotice) error {
	return ErrUnsupported
}
