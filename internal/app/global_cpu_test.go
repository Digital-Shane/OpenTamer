package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestCPULimiterDoesNotContinueBeforeItStops(t *testing.T) {
	signals := &limiterSignals{}
	limiter := NewCPULimiter(signals)
	limiter.Update([]CPULimitRequest{{
		AppID:     core.AppID{Name: "Worker"},
		Processes: []core.ProcessRef{{ID: core.ProcessID{PID: 42}}},
		RunFor:    time.Hour, StopFor: time.Hour,
	}})
	limiter.StopAll()
	if signals.stopCount() != 0 || signals.continueCount() != 0 {
		t.Fatal("limiter signaled a process it never stopped")
	}
}

type failedStopSignals struct {
	limiterSignals
	attempted chan struct{}
}

func (signals *failedStopSignals) Stop(core.ProcessRef) error {
	close(signals.attempted)
	return errors.New("stop denied")
}

func TestCPULimiterDoesNotContinueFailedStops(t *testing.T) {
	signals := &failedStopSignals{attempted: make(chan struct{})}
	limiter := NewCPULimiter(signals)
	limiter.Update([]CPULimitRequest{{
		AppID: core.AppID{Name: "Worker"}, Processes: []core.ProcessRef{{ID: core.ProcessID{PID: 42}}},
		RunFor: time.Millisecond, StopFor: time.Hour,
	}})
	select {
	case <-signals.attempted:
	case <-time.After(time.Second):
		t.Fatal("stop was not attempted")
	}
	limiter.StopAll()
	if signals.continueCount() != 0 {
		t.Fatal("limiter continued a process whose stop failed")
	}
}

type observedStopSignals struct {
	limiterSignals
	observeError error
}

func (signals *observedStopSignals) IsStopped(process core.ProcessRef) (bool, error) {
	if process.ID.PID == 42 {
		return true, signals.observeError
	}
	return false, nil
}

func TestCPULimiterPreservesExternalStopsAndObservationFailures(t *testing.T) {
	for _, err := range []error{nil, errors.New("state unavailable")} {
		signals := &observedStopSignals{observeError: err}
		limiter := NewCPULimiter(signals)
		limiter.Update([]CPULimitRequest{{AppID: core.AppID{Name: "Mixed"}, Processes: []core.ProcessRef{{ID: core.ProcessID{PID: 42}}, {ID: core.ProcessID{PID: 43}}}, RunFor: time.Millisecond, StopFor: time.Hour}})
		if !signals.waitForStops(1, time.Second) {
			limiter.StopAll()
			t.Fatal("runnable member was not stopped")
		}
		limiter.StopAll()
		if signals.stopCount() != 1 || signals.continueCount() != 1 || signals.stopped[0] != 43 || signals.continued[0] != 43 {
			t.Fatalf("external stop was touched: stopped=%v continued=%v", signals.stopped, signals.continued)
		}
	}
}

func TestCPULimiterDeduplicatesAndTransfersPIDOwnership(t *testing.T) {
	signals := &limiterSignals{}
	limiter := NewCPULimiter(signals)
	t.Cleanup(limiter.StopAll)
	process := core.ProcessRef{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}}
	first := CPULimitRequest{AppID: core.AppID{Name: "First"}, Processes: []core.ProcessRef{process}, RunFor: time.Millisecond, StopFor: time.Hour}
	second := first
	second.AppID = core.AppID{Name: "Second"}
	limiter.Update([]CPULimitRequest{first, second})
	if len(limiter.entries) != 1 {
		t.Fatal("overlapping workers created")
	}
	if !signals.waitForStops(1, time.Second) {
		t.Fatal("worker did not stop")
	}
	old := limiter.entries[first.AppID.Key()]
	limiter.Update([]CPULimitRequest{second})
	select {
	case <-old.done:
	default:
		t.Fatal("old worker still owns regrouped PID")
	}
	if signals.continueCount() != 1 {
		t.Fatal("old stopped owner was not restored")
	}
	if !signals.waitForStops(2, time.Second) {
		t.Fatal("replacement worker did not start")
	}
	limiter.StopAll()
	if signals.continueCount() != 2 {
		t.Fatal("replacement did not restore its own stop")
	}
}
