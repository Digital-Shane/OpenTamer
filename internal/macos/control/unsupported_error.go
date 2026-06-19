package control

import "errors"

var ErrUnsupported = errors.New("macOS process control is unsupported on this platform")
