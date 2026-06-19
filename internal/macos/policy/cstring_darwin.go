//go:build darwin && cgo

package policy

/*
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

func copyCString(value *C.char) (string, error) {
	if value == nil {
		return "", errors.New("system policy bridge returned nil")
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value), nil
}
