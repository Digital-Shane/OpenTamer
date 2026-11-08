//go:build darwin && cgo

package process

/*
#include <stdlib.h>
#include <limits.h>
#include "process_bridge.h"
*/
import "C"

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	appcontrol "github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
)

type ProcessEnumerator struct{}

func NewProcessEnumerator() *ProcessEnumerator {
	return &ProcessEnumerator{}
}

func (enumerator *ProcessEnumerator) Processes() ([]core.ProcessRef, error) {
	requiredBytes := int(C.OpenTamerListPIDs(nil, 0))
	if requiredBytes <= 0 {
		return nil, fmt.Errorf("list pids: no processes returned")
	}

	capacity := requiredBytes/int(C.sizeof_int) + 256
	bufferBytes := C.size_t(capacity * int(C.sizeof_int))
	pids := (*C.int)(C.malloc(bufferBytes))
	if pids == nil {
		return nil, fmt.Errorf("allocate pid buffer")
	}
	defer C.free(unsafe.Pointer(pids))

	usedBytes := int(C.OpenTamerListPIDs(pids, C.int(capacity)))
	if usedBytes < 0 {
		return nil, fmt.Errorf("list pids failed")
	}

	count := usedBytes / int(C.sizeof_int)
	pidSlice := unsafe.Slice(pids, count)
	processes := make([]core.ProcessRef, 0, count)

	for _, pid := range pidSlice {
		if pid <= 0 {
			continue
		}
		process, err := copyProcessRef(pid)
		if err != nil {
			continue
		}
		processes = append(processes, process)
	}

	return processes, nil
}

func (enumerator *ProcessEnumerator) ProcessGeneration(pid int) (core.ProcessID, error) {
	process, err := enumerator.ProcessIdentity(pid)
	if err != nil {
		return core.ProcessID{}, err
	}
	return process.ID, nil
}

func (enumerator *ProcessEnumerator) ProcessIdentity(pid int) (core.ProcessRef, error) {
	if pid <= 0 {
		return core.ProcessRef{}, appcontrol.ErrProcessExited
	}
	if pid > C.INT_MAX {
		return core.ProcessRef{}, fmt.Errorf("read process identity for pid %d: %w", pid, syscall.EINVAL)
	}
	return copyProcessRef(C.int(pid))
}

func copyProcessRef(pid C.int) (core.ProcessRef, error) {
	if pid <= 0 {
		return core.ProcessRef{}, appcontrol.ErrProcessExited
	}
	var info C.OpenTamerProcessInfo
	if code := C.OpenTamerCopyProcessInfo(pid, &info); code != 1 {
		return core.ProcessRef{}, processLookupError(int(pid), int(code))
	}
	return core.ProcessRef{
		ID: core.ProcessID{
			PID:       int(info.pid),
			StartTime: time.Unix(int64(info.start_sec), int64(info.start_usec)*1000),
		},
		ParentPID:      int(info.ppid),
		UID:            int(info.uid),
		GID:            int(info.gid),
		Nice:           int(info.nice),
		Name:           C.GoString(&info.name[0]),
		ExecutablePath: C.GoString(&info.path[0]),
	}, nil
}

func processLookupError(pid, code int) error {
	err := syscall.EIO
	if code < 0 {
		err = syscall.Errno(-code)
	}
	if err == syscall.ESRCH || err == syscall.ENOENT {
		return fmt.Errorf("read process identity for pid %d: %w: %w", pid, appcontrol.ErrProcessExited, err)
	}
	return fmt.Errorf("read process identity for pid %d: %w", pid, err)
}
