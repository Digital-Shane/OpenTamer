//go:build darwin && cgo

package notify

/*
#cgo darwin CFLAGS: -fobjc-arc
*/
import "C"
