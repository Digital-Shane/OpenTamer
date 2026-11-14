//go:build darwin && cgo

package app_test

import (
	"os/exec"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/macos/control"
	"github.com/Digital-Shane/open-tamer/internal/macos/process"
)

type transientResumeFailure struct {
	*control.DarwinSignalController
	attempts atomic.Int64
}

func (signals *transientResumeFailure) Continue(ref core.ProcessRef) error {
	if signals.attempts.Add(1) == 1 {
		return syscall.EAGAIN
	}
	return signals.DarwinSignalController.Continue(ref)
}

func TestCPULimiterRecoversRealChildAfterTransientResumeFailure(t *testing.T) {
	child := exec.Command("/bin/sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	enumerator := process.NewProcessEnumerator()
	ref, err := enumerator.ProcessIdentity(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	signals := &transientResumeFailure{DarwinSignalController: control.NewSignalController()}
	limiter := app.NewCPULimiter(signals, app.ProcessGenerationLookupValidator{Lookup: enumerator})
	t.Cleanup(func() { limiter.StopAll() })
	limiter.Update([]app.CPULimitRequest{{
		AppID: core.AppID{Name: "Recovery Test"}, Processes: []core.ProcessRef{ref},
		RunFor: time.Millisecond, StopFor: time.Hour,
	}})
	deadline := time.Now().Add(time.Second)
	for {
		stopped, err := signals.IsStopped(ref)
		if err != nil {
			t.Fatal(err)
		}
		if stopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("test child was not stopped")
		}
		time.Sleep(time.Millisecond)
	}
	if failures := limiter.StopAll(); len(failures) != 1 || len(limiter.PendingRecovery()) != 1 {
		t.Fatalf("failed resume did not retain ownership: %v", failures)
	}
	if stopped, err := signals.IsStopped(ref); err != nil || !stopped {
		t.Fatalf("child should remain stopped after injected failure: stopped=%t, error=%v", stopped, err)
	}
	if failures := limiter.Update(nil); len(failures) != 0 || len(limiter.PendingRecovery()) != 0 {
		t.Fatalf("retry did not complete recovery: %v", failures)
	}
	if stopped, err := signals.IsStopped(ref); err != nil || stopped {
		t.Fatalf("child was not resumed: stopped=%t, error=%v", stopped, err)
	}
}
