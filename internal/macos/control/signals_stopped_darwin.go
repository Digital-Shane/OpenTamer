//go:build darwin && cgo

package control

/*
#include <errno.h>
#include <libproc.h>
#include <sys/proc.h>
#include <sys/proc_info.h>

static int OpenTamerCopyStoppedInfo(int pid, struct proc_bsdinfo *info) {
    errno = 0;
    int size = proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, info, PROC_PIDTBSDINFO_SIZE);
    if (size == PROC_PIDTBSDINFO_SIZE) {
        return 0;
    }
    return size <= 0 && errno != 0 ? errno : EIO;
}
*/
import "C"

import (
	"fmt"
	"syscall"
	"time"

	appcontrol "github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
)

// IsStopped reads process state without sending signals.
func (controller *DarwinSignalController) IsStopped(process core.ProcessRef) (bool, error) {
	pid := process.ID.PID
	if pid <= 0 || int(C.int(pid)) != pid {
		return false, appcontrol.ErrProcessExited
	}

	var info C.struct_proc_bsdinfo
	if code := C.OpenTamerCopyStoppedInfo(C.int(pid), &info); code != 0 {
		err := syscall.Errno(code)
		if err == syscall.ESRCH || err == syscall.ENOENT {
			return false, appcontrol.ErrProcessExited
		}
		return false, fmt.Errorf("read process state for pid %d: %w", pid, err)
	}
	if int(info.pbi_pid) != pid || info.pbi_status == C.SZOMB {
		return false, appcontrol.ErrProcessExited
	}
	if !process.ID.StartTime.IsZero() {
		started := time.Unix(int64(info.pbi_start_tvsec), int64(info.pbi_start_tvusec)*1000)
		if !process.ID.StartTime.Equal(started) {
			return false, fmt.Errorf("%w: pid %d", appcontrol.ErrProcessGenerationMismatch, pid)
		}
	}
	return info.pbi_status == C.SSTOP, nil
}
