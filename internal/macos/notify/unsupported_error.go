package notify

import "errors"

var ErrUnsupported = errors.New("macOS notifications require macOS with cgo")
