package app

import (
	"sync"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestCPULimiterStopsAndContinuesProcesses(t *testing.T) {
	signals := &limiterSignals{}
	limiter := NewCPULimiter(signals)
	process := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, Name: "worker"}

	limiter.Update([]CPULimitRequest{{
		AppID:     core.AppID{Name: "Worker"},
		Processes: []core.ProcessRef{process},
		RunFor:    time.Millisecond,
		StopFor:   time.Millisecond,
	}})
	t.Cleanup(limiter.StopAll)

	if !signals.waitForStops(1, 100*time.Millisecond) {
		t.Fatalf("stops = %d, want at least 1", signals.stopCount())
	}
	if !signals.waitForContinues(1, 100*time.Millisecond) {
		t.Fatalf("continues = %d, want at least 1", signals.continueCount())
	}
}

func TestCPULimiterStopAllResumesStoppedProcesses(t *testing.T) {
	signals := &limiterSignals{}
	limiter := NewCPULimiter(signals)
	process := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, Name: "worker"}

	limiter.Update([]CPULimitRequest{{
		AppID:     core.AppID{Name: "Worker"},
		Processes: []core.ProcessRef{process},
		RunFor:    time.Millisecond,
		StopFor:   time.Hour,
	}})
	if !signals.waitForStops(1, 100*time.Millisecond) {
		t.Fatalf("stops = %d, want at least 1", signals.stopCount())
	}
	limiter.StopAll()
	if !signals.waitForContinues(1, 100*time.Millisecond) {
		t.Fatalf("continues = %d, want at least 1", signals.continueCount())
	}
}

func TestCPULimiterUpdatesDutyCycleWithoutRestartingStoppedEntry(t *testing.T) {
	signals := &limiterSignals{}
	limiter := NewCPULimiter(signals)
	process := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, Name: "worker"}

	limiter.Update([]CPULimitRequest{{
		AppID:     core.AppID{Name: "Worker"},
		Processes: []core.ProcessRef{process},
		RunFor:    time.Millisecond,
		StopFor:   time.Hour,
	}})
	t.Cleanup(limiter.StopAll)

	if !signals.waitForStops(1, 100*time.Millisecond) {
		t.Fatalf("stops = %d, want at least 1", signals.stopCount())
	}
	if !signals.waitForContinues(1, 100*time.Millisecond) {
		t.Fatalf("continues = %d, want initial continue", signals.continueCount())
	}
	continuesBeforeUpdate := signals.continueCount()

	limiter.Update([]CPULimitRequest{{
		AppID:     core.AppID{Name: "Worker"},
		Processes: []core.ProcessRef{process},
		RunFor:    25 * time.Millisecond,
		StopFor:   time.Hour,
	}})
	time.Sleep(10 * time.Millisecond)

	if continues := signals.continueCount(); continues != continuesBeforeUpdate {
		t.Fatalf("continues after in-place update = %d, want %d", continues, continuesBeforeUpdate)
	}
}

func TestCPULimiterShortensCurrentRunPhaseOnDutyCycleUpdate(t *testing.T) {
	signals := &limiterSignals{}
	limiter := NewCPULimiter(signals)
	process := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, Name: "worker"}

	limiter.Update([]CPULimitRequest{{
		AppID:     core.AppID{Name: "Worker"},
		Processes: []core.ProcessRef{process},
		RunFor:    time.Hour,
		StopFor:   time.Hour,
	}})
	t.Cleanup(limiter.StopAll)

	if !signals.waitForContinues(1, 100*time.Millisecond) {
		t.Fatalf("continues = %d, want initial continue", signals.continueCount())
	}

	limiter.Update([]CPULimitRequest{{
		AppID:     core.AppID{Name: "Worker"},
		Processes: []core.ProcessRef{process},
		RunFor:    time.Millisecond,
		StopFor:   time.Hour,
	}})

	if !signals.waitForStops(1, 100*time.Millisecond) {
		t.Fatalf("stops = %d, want updated run phase to stop promptly", signals.stopCount())
	}
}

type limiterSignals struct {
	mu        sync.Mutex
	stopped   []int
	continued []int
}

func (signals *limiterSignals) Stop(process core.ProcessRef) error {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	signals.stopped = append(signals.stopped, process.ID.PID)
	return nil
}

func (signals *limiterSignals) Continue(process core.ProcessRef) error {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	signals.continued = append(signals.continued, process.ID.PID)
	return nil
}

func (signals *limiterSignals) waitForStops(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if signals.stopCount() >= count {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return signals.stopCount() >= count
}

func (signals *limiterSignals) waitForContinues(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if signals.continueCount() >= count {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return signals.continueCount() >= count
}

func (signals *limiterSignals) stopCount() int {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	return len(signals.stopped)
}

func (signals *limiterSignals) continueCount() int {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	return len(signals.continued)
}
