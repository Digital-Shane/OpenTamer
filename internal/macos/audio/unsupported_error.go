package audio

import "errors"

var ErrUnsupported = errors.New("macOS audio observer requires macOS with cgo")
