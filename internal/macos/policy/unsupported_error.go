package policy

import "errors"

var ErrUnsupported = errors.New("macOS system policy observer requires macOS with cgo")
