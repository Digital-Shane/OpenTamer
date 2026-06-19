package process

import "errors"

var ErrUnsupported = errors.New("macOS process enumeration requires macOS with cgo")
