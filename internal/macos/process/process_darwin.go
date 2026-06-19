//go:build darwin && cgo

package process

/*
#include <stdlib.h>
#include "process_bridge.h"
*/
import "C"

import (
	"fmt"
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
	process, err := copyProcessRef(C.int(pid))
	if err != nil {
		return core.ProcessID{}, err
	}
	return process.ID, nil
}

func copyProcessRef(pid C.int) (core.ProcessRef, error) {
	if pid <= 0 {
		return core.ProcessRef{}, appcontrol.ErrProcessExited
	}
	var info C.OpenTamerProcessInfo
	if C.OpenTamerCopyProcessInfo(pid, &info) != 1 {
		return core.ProcessRef{}, appcontrol.ErrProcessExited
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
