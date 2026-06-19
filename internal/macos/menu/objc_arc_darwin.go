//go:build darwin && cgo

package menu

/*
#cgo darwin CFLAGS: -fobjc-arc
*/
import "C"
