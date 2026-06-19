package metrics

import "errors"

var ErrUnsupported = errors.New("macOS metrics sampler requires macOS with cgo")
