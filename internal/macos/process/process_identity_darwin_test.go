//go:build darwin && cgo

package process

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	appcontrol "github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
)

var _ appcontrol.ProcessIdentityLookup = (*ProcessEnumerator)(nil)
var _ appcontrol.ProcessGenerationLookup = (*ProcessEnumerator)(nil)

func TestProcessIdentityChild(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	before := time.Now().Add(-time.Second)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	enumerator := NewProcessEnumerator()
	ref, err := enumerator.ProcessIdentity(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ID.PID != cmd.Process.Pid || ref.ParentPID != os.Getpid() ||
		ref.UID != os.Getuid() || ref.GID != os.Getgid() {
		t.Fatalf("unexpected child identity: %+v", ref)
	}
	if ref.ID.StartTime.Before(before) || ref.ID.StartTime.After(time.Now()) {
		t.Fatalf("unexpected start time: %v", ref.ID.StartTime)
	}
	if ref.Name != "sleep" || filepath.Base(ref.ExecutablePath) != "sleep" {
		t.Fatalf("unexpected child metadata: %+v", ref)
	}
	id, err := enumerator.ProcessGeneration(cmd.Process.Pid)
	if err != nil || !id.SameGeneration(ref.ID) {
		t.Fatalf("generation = %+v, %v; identity = %+v", id, err, ref.ID)
	}
	refs, err := enumerator.Processes()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, listed := range refs {
		if listed.ID.SameGeneration(ref.ID) {
			found = true
			if listed != ref {
				t.Fatalf("listed metadata = %+v; identity = %+v", listed, ref)
			}
		}
	}
	if !found {
		t.Fatal("child missing from process list")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	got, err := enumerator.ProcessIdentity(cmd.Process.Pid)
	if got != (core.ProcessRef{}) || !errors.Is(err, appcontrol.ErrProcessExited) {
		t.Fatalf("exited identity = %+v, %v", got, err)
	}
	id, err = enumerator.ProcessGeneration(cmd.Process.Pid)
	if id != (core.ProcessID{}) || !errors.Is(err, appcontrol.ErrProcessExited) {
		t.Fatalf("exited generation = %+v, %v", id, err)
	}
}

func TestProcessIdentityInvalidPID(t *testing.T) {
	enumerator := NewProcessEnumerator()
	for _, pid := range []int{0, -1, -int(^uint(0)>>1) - 1} {
		t.Run(strconv.Itoa(pid), func(t *testing.T) {
			ref, err := enumerator.ProcessIdentity(pid)
			if ref != (core.ProcessRef{}) || !errors.Is(err, appcontrol.ErrProcessExited) {
				t.Fatalf("identity = %+v, %v", ref, err)
			}
			id, err := enumerator.ProcessGeneration(pid)
			if id != (core.ProcessID{}) || !errors.Is(err, appcontrol.ErrProcessExited) {
				t.Fatalf("generation = %+v, %v", id, err)
			}
		})
	}
	// These values would truncate to invalid or live PIDs in a native int.
	for _, pid := range []int{1 << 31, 1 << 32, 1<<32 + os.Getpid(), int(^uint(0) >> 1)} {
		t.Run(strconv.Itoa(pid), func(t *testing.T) {
			ref, err := enumerator.ProcessIdentity(pid)
			if ref != (core.ProcessRef{}) || !errors.Is(err, syscall.EINVAL) || errors.Is(err, appcontrol.ErrProcessExited) {
				t.Fatalf("overflow identity = %+v, %v", ref, err)
			}
			id, err := enumerator.ProcessGeneration(pid)
			if id != (core.ProcessID{}) || !errors.Is(err, syscall.EINVAL) || errors.Is(err, appcontrol.ErrProcessExited) {
				t.Fatalf("overflow generation = %+v, %v", id, err)
			}
		})
	}
}

func TestProcessIdentityLookupErrors(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.ESRCH, syscall.ENOENT, syscall.EPERM, syscall.EACCES, syscall.EIO, syscall.EINTR, syscall.EAGAIN} {
		t.Run(errno.Error(), func(t *testing.T) {
			err := processLookupError(42, -int(errno))
			if !errors.Is(err, errno) {
				t.Fatalf("lost errno: %v", err)
			}
			wantExited := errno == syscall.ESRCH || errno == syscall.ENOENT
			if errors.Is(err, appcontrol.ErrProcessExited) != wantExited {
				t.Fatalf("incorrect exit classification: %v", err)
			}
		})
	}
	for _, code := range []int{0, 2} {
		if err := processLookupError(42, code); !errors.Is(err, syscall.EIO) || errors.Is(err, appcontrol.ErrProcessExited) {
			t.Fatalf("unexpected bridge status %d: %v", code, err)
		}
	}
}

func TestProcessIdentityNativeBridge(t *testing.T) {
	compiler, err := exec.LookPath("clang")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "identity-bridge-test")
	cmd := exec.Command(compiler, "-Wall", "-Wextra", "-Werror", "testdata/identity_bridge_test.c", "-lproc", "-o", binary)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile native bridge tests: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native bridge tests: %v\n%s", err, output)
	}
}
