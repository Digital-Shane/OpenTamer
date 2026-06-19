package menu

import "errors"

var ErrUnsupported = errors.New("macOS menu-bar UI requires macOS with cgo")
