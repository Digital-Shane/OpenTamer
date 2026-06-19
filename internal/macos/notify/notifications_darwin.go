//go:build darwin && cgo

package notify

/*
#cgo darwin LDFLAGS: -framework UserNotifications -framework Foundation
#include <stdlib.h>
#include "notifications_bridge.h"
*/
import "C"

import (
	"fmt"
	"log"
	"unsafe"

	"github.com/Digital-Shane/open-tamer/internal/app"
)

const notificationQueueSize = 16

var deliverNotification = deliverNativeNotification

type UserNotifier struct {
	queue chan app.HighCPUNotice
}

func NewUserNotifier() *UserNotifier {
	notifier := &UserNotifier{queue: make(chan app.HighCPUNotice, notificationQueueSize)}
	go notifier.run()
	return notifier
}

func (notifier *UserNotifier) NotifyHighCPU(notice app.HighCPUNotice) error {
	if notifier == nil {
		return fmt.Errorf("notifier is not configured")
	}
	select {
	case notifier.queue <- notice:
		return nil
	default:
		return fmt.Errorf("notification queue full")
	}
}

func (notifier *UserNotifier) run() {
	for notice := range notifier.queue {
		if err := deliverHighCPUNotification(notice); err != nil {
			log.Printf("OpenTamer notification warning: %v", err)
		}
	}
}

func deliverHighCPUNotification(notice app.HighCPUNotice) error {
	return deliverNotification("High CPU usage", fmt.Sprintf("%s is using %.0f%% CPU.", notice.AppName, notice.CPUPercent))
}

func deliverNativeNotification(titleText, bodyText string) error {
	title := C.CString(titleText)
	body := C.CString(bodyText)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(body))

	var nativeError *C.char
	if C.OpenTamerDeliverNotification(title, body, &nativeError) != 1 {
		if nativeError != nil {
			defer C.free(unsafe.Pointer(nativeError))
			message := C.GoString(nativeError)
			if message != "" {
				return fmt.Errorf("%s", message)
			}
		}
		return fmt.Errorf("deliver notification")
	}
	return nil
}
