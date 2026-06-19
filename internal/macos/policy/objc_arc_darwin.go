//go:build darwin && cgo

package policy

/*
#cgo darwin CFLAGS: -fobjc-arc
*/
import "C"
