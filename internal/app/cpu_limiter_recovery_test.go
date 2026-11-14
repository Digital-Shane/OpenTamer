package app

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type recoverySignals struct {
	mu               sync.Mutex
	stops, continues int
	continueErr      error
	stopErr          error
	external         bool
	observeErr       error
	attempted        chan struct{}
}

func (signals *recoverySignals) Stop(core.ProcessRef) error {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	signals.stops++
	return signals.stopErr
}

func (signals *recoverySignals) Continue(core.ProcessRef) error {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	signals.continues++
	if signals.attempted != nil {
		select {
		case signals.attempted <- struct{}{}:
		default:
		}
	}
	return signals.continueErr
}

func (signals *recoverySignals) IsStopped(core.ProcessRef) (bool, error) {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	return signals.external, signals.observeErr
}

func (signals *recoverySignals) setContinueError(err error) {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	signals.continueErr = err
}

func (signals *recoverySignals) counts() (int, int) {
	signals.mu.Lock()
	defer signals.mu.Unlock()
	return signals.stops, signals.continues
}

type recoveryValidator struct {
	mu                 sync.Mutex
	err                error
	automaticErr       error
	automaticDeniedPID int
	automaticCalls     int
}

func (validator *recoveryValidator) ValidateProcessGeneration(core.ProcessRef) error {
	validator.mu.Lock()
	defer validator.mu.Unlock()
	return validator.err
}

func (validator *recoveryValidator) ValidateAutomaticProcess(process core.ProcessRef) error {
	validator.mu.Lock()
	defer validator.mu.Unlock()
	validator.automaticCalls++
	if validator.automaticDeniedPID != 0 && process.ID.PID != validator.automaticDeniedPID {
		return nil
	}
	return validator.automaticErr
}

func (validator *recoveryValidator) setError(err error) {
	validator.mu.Lock()
	defer validator.mu.Unlock()
	validator.err = err
}

func recoveryRequest() CPULimitRequest {
	return CPULimitRequest{
		AppID:     core.AppID{Name: "Recovery"},
		Processes: []core.ProcessRef{{ID: core.ProcessID{PID: 42, StartTime: time.Unix(1, 0)}}},
		RunFor:    time.Hour, StopFor: time.Hour,
	}
}

func requireRecovery(t *testing.T, limiter *CPULimiter, count int) {
	t.Helper()
	if pending := limiter.PendingRecovery(); len(pending) != count {
		t.Fatalf("pending = %v, want %d generations", pending, count)
	}
}

func TestCPULimiterRecoveryKeepsFailedContinueAndBlocksStop(t *testing.T) {
	denied := errors.New("continue denied")
	signals := &recoverySignals{continueErr: denied}
	limiter := NewCPULimiter(signals)
	request := recoveryRequest()
	stopped := limiter.stopProcesses(request)
	requireRecovery(t, limiter, 0)
	if failures := limiter.RetryRecovery(); len(failures) != 0 {
		t.Fatal(failures)
	}
	if _, continues := signals.counts(); continues != 0 {
		t.Fatal("healthy stopped phase was resumed by retry")
	}
	limiter.continueProcesses(stopped)
	requireRecovery(t, limiter, 1)
	pending := limiter.PendingRecovery()
	pending[0].ID.PID = 99
	if limiter.PendingRecovery()[0].ID.PID != 42 {
		t.Fatal("snapshot aliases ledger")
	}
	if stopped := limiter.stopProcesses(request); len(stopped) != 0 {
		t.Fatal("stopped a generation with recovery debt")
	}
	failures := limiter.Update(nil)
	if len(failures) != 1 || !errors.Is(failures[0].Err, denied) || failures[0].Action != core.ControlActionResume || !failures[0].Process.ID.SameGeneration(request.Processes[0].ID) {
		t.Fatalf("failures = %v", failures)
	}
	signals.setContinueError(nil)
	if failures := limiter.Update(nil); len(failures) != 0 {
		t.Fatal(failures)
	}
	requireRecovery(t, limiter, 0)
	if stopped := limiter.stopProcesses(request); len(stopped) != 1 {
		t.Fatal("successful recovery did not release stop gate")
	} else {
		limiter.continueProcesses(stopped)
	}
}

func TestCPULimiterRecoveryValidationAndTerminalErrors(t *testing.T) {
	for _, source := range []string{"validation", "continue"} {
		for _, terminal := range []error{ErrProcessExited, ErrProcessGenerationMismatch} {
			t.Run(source+"/"+terminal.Error(), func(t *testing.T) {
				signals := &recoverySignals{}
				validator := &recoveryValidator{}
				limiter := NewCPULimiter(signals, validator)
				stopped := limiter.stopProcesses(recoveryRequest())
				validator.setError(errors.New("temporary lookup error"))
				limiter.continueProcesses(stopped)
				requireRecovery(t, limiter, 1)
				if failures := limiter.RetryRecovery(); len(failures) != 1 {
					t.Fatalf("failures = %v", failures)
				}
				if _, continues := signals.counts(); continues != 0 {
					t.Fatal("continued without validating generation")
				}
				if source == "validation" {
					validator.setError(fmt.Errorf("wrapped: %w", terminal))
				} else {
					validator.setError(nil)
					signals.setContinueError(fmt.Errorf("wrapped: %w", terminal))
				}
				if failures := limiter.RetryRecovery(); len(failures) != 0 {
					t.Fatal(failures)
				}
				requireRecovery(t, limiter, 0)
			})
		}
	}
}

func TestCPULimiterRecoveryValidationLaterSucceeds(t *testing.T) {
	signals := &recoverySignals{}
	validator := &recoveryValidator{}
	limiter := NewCPULimiter(signals, validator)
	stopped := limiter.stopProcesses(recoveryRequest())
	validator.setError(errors.New("lookup unavailable"))
	limiter.continueProcesses(stopped)
	requireRecovery(t, limiter, 1)
	validator.setError(nil)
	if failures := limiter.RetryRecovery(); len(failures) != 0 {
		t.Fatal(failures)
	}
	requireRecovery(t, limiter, 0)
	if _, continues := signals.counts(); continues != 1 {
		t.Fatalf("continues = %d", continues)
	}
}

func TestCPULimiterRecoveryPreservesExternalAndFailedStops(t *testing.T) {
	for _, signals := range []*recoverySignals{
		{external: true}, {observeErr: errors.New("unavailable")}, {stopErr: errors.New("denied")},
	} {
		limiter := NewCPULimiter(signals)
		limiter.continueProcesses(limiter.stopProcesses(recoveryRequest()))
		limiter.StopAll()
		requireRecovery(t, limiter, 0)
		if _, continues := signals.counts(); continues != 0 {
			t.Fatal("continued a process not stopped by limiter")
		}
	}
}

func TestCPULimiterRecoveryGenerationKeys(t *testing.T) {
	signals := &recoverySignals{continueErr: errors.New("denied")}
	limiter := NewCPULimiter(signals)
	request := recoveryRequest()
	limiter.continueProcesses(limiter.stopProcesses(request))
	request.Processes[0].ID.StartTime = time.Unix(2, 0)
	limiter.continueProcesses(limiter.stopProcesses(request))
	requireRecovery(t, limiter, 2)
	signals.setContinueError(nil)
	limiter.RetryRecovery()
	requireRecovery(t, limiter, 0)
}

func TestCPULimiterRecoveryBlocksWholeGroupAndRetriesOnlyItsMembers(t *testing.T) {
	signals := &recoverySignals{continueErr: errors.New("denied")}
	limiter := NewCPULimiter(signals)
	request := recoveryRequest()
	limiter.continueProcesses(limiter.stopProcesses(request))
	other := recoveryRequest()
	other.Processes[0].ID.PID = 99
	limiter.continueProcesses(limiter.stopProcesses(other))
	request.Processes = append(request.Processes, core.ProcessRef{ID: core.ProcessID{PID: 43}})
	_, before := signals.counts()
	limiter.retryProcessRecovery(request.Processes)
	if _, after := signals.counts(); after != before+1 {
		t.Fatalf("worker retried unrelated ownership: continues = %d, want %d", after, before+1)
	}
	if stopped := limiter.stopProcesses(request); len(stopped) != 0 {
		t.Fatal("unresolved first member allowed fresh group stops")
	}
	// The unresolved member must gate the group even when listed last.
	request.Processes[0], request.Processes[1] = request.Processes[1], request.Processes[0]
	if stopped := limiter.stopProcesses(request); len(stopped) != 0 {
		t.Fatal("unresolved last member allowed fresh group stops")
	}
	if stops, _ := signals.counts(); stops != 2 {
		t.Fatalf("stops = %d, want only two original owners", stops)
	}
	signals.setContinueError(nil)
	limiter.retryProcessRecovery(request.Processes)
	requireRecovery(t, limiter, 1)
	if limiter.PendingRecovery()[0].ID.PID != 99 {
		t.Fatal("scoped retry recovered unrelated process")
	}
	stopped := limiter.stopProcesses(request)
	if len(stopped) != 2 {
		t.Fatal("recovered group did not restart together")
	}
	limiter.continueProcesses(stopped)
	limiter.RetryRecovery()
	requireRecovery(t, limiter, 0)
}

func TestCPULimiterRecoveryConcurrentRetriesSerializeOwnership(t *testing.T) {
	signals := &recoverySignals{continueErr: errors.New("denied")}
	limiter := NewCPULimiter(signals)
	limiter.continueProcesses(limiter.stopProcesses(recoveryRequest()))
	signals.setContinueError(nil)
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			limiter.PendingRecovery()
			limiter.RetryRecovery()
		})
	}
	workers.Wait()
	if _, continues := signals.counts(); continues != 2 {
		t.Fatalf("concurrent retries duplicated CONT: %d", continues)
	}
	requireRecovery(t, limiter, 0)
}

func TestCPULimiterRecoveryStopAllSinglePass(t *testing.T) {
	signals := &recoverySignals{continueErr: errors.New("denied")}
	limiter := NewCPULimiter(signals)
	request := recoveryRequest()
	request.RunFor = time.Millisecond
	entry := startRecoveryWorker(t, limiter, request)
	if failures := limiter.StopAll(); len(failures) != 1 {
		t.Fatalf("shutdown failures = %v", failures)
	}
	select {
	case <-entry.done:
	default:
		t.Fatal("shutdown did not join worker")
	}
	if _, continues := signals.counts(); continues != 1 {
		t.Fatalf("shutdown attempts = %d, want exactly one", continues)
	}
	requireRecovery(t, limiter, 1)
	signals.setContinueError(nil)
	if failures := limiter.StopAll(); len(failures) != 0 {
		t.Fatal(failures)
	}
	requireRecovery(t, limiter, 0)
}

func TestCPULimiterRecoveryWorkerRetiresWithoutLimiterLock(t *testing.T) {
	signals := &recoverySignals{}
	limiter := NewCPULimiter(signals)
	t.Cleanup(func() { limiter.StopAll() })
	request := recoveryRequest()
	request.RunFor = time.Millisecond
	entry := startRecoveryWorker(t, limiter, request)
	limiter.mu.Lock()
	entry.cancel()
	select {
	case <-entry.done:
	case <-time.After(time.Second):
		limiter.mu.Unlock()
		t.Fatal("worker retirement depends on limiter lock")
	}
	limiter.mu.Unlock()
	limiter.StopAll()
	requireRecovery(t, limiter, 0)
}

func startRecoveryWorker(t *testing.T, limiter *CPULimiter, request CPULimitRequest) *cpuLimitEntry {
	t.Helper()
	limiter.Update([]CPULimitRequest{request})
	entry := limiter.entries[request.AppID.Key()]
	// Observe the completed STOP under the same lock as ownership registration.
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		limiter.recoveryMu.Lock()
		_, stopped := limiter.owned[request.Processes[0].ID.Key()]
		limiter.recoveryMu.Unlock()
		if stopped {
			return entry
		}
		select {
		case <-deadline:
			t.Fatal("worker did not stop")
		case <-ticker.C:
		}
	}
}

func TestCPULimiterRecoveryRetirementAndReplacement(t *testing.T) {
	signals := &recoverySignals{continueErr: errors.New("denied")}
	limiter := NewCPULimiter(signals)
	t.Cleanup(func() { signals.setContinueError(nil); limiter.StopAll() })
	first := recoveryRequest()
	first.RunFor = time.Millisecond
	old := startRecoveryWorker(t, limiter, first)
	requireRecovery(t, limiter, 0)
	second := first
	second.AppID = core.AppID{Name: "Replacement"}
	second.RunFor = time.Hour
	if failures := limiter.Update([]CPULimitRequest{second}); len(failures) != 1 {
		t.Fatalf("retirement failures = %v", failures)
	}
	select {
	case <-old.done:
	default:
		t.Fatal("replacement preceded retirement")
	}
	requireRecovery(t, limiter, 1)
	if stopped := limiter.stopProcesses(second); len(stopped) != 0 {
		t.Fatal("replacement stopped unresolved generation")
	}
	if failures := limiter.StopAll(); len(failures) != 1 {
		t.Fatalf("shutdown failures = %v", failures)
	}
	if len(limiter.entries) != 0 {
		t.Fatal("shutdown left workers")
	}
	requireRecovery(t, limiter, 1)
	stops, continues := signals.counts()
	if stops != 1 || continues != 2 {
		t.Fatalf("signals = %d/%d, want 1/2", stops, continues)
	}
	signals.setContinueError(nil)
	if failures := limiter.Update(nil); len(failures) != 0 {
		t.Fatal(failures)
	}
	requireRecovery(t, limiter, 0)
	if failures := limiter.StopAll(); len(failures) != 0 {
		t.Fatal(failures)
	}
	if _, count := signals.counts(); count != 3 {
		t.Fatalf("continues = %d", count)
	}
}

func TestCPULimiterRecoveryActiveWorkerRetries(t *testing.T) {
	signals := &recoverySignals{continueErr: errors.New("denied"), attempted: make(chan struct{}, 16)}
	limiter := NewCPULimiter(signals)
	t.Cleanup(func() { limiter.StopAll() })
	request := recoveryRequest()
	request.RunFor, request.StopFor = time.Millisecond, time.Hour
	startRecoveryWorker(t, limiter, request)
	// Fail the first resume explicitly, then let the same worker retry next cycle.
	limiter.continueProcesses(request.Processes)
	requireRecovery(t, limiter, 1)
	<-signals.attempted
	signals.setContinueError(nil)
	request.StopFor = time.Millisecond
	limiter.entries[request.AppID.Key()].update(request)
	select {
	case <-signals.attempted:
	case <-time.After(time.Second):
		t.Fatal("active worker did not retry")
	}
	limiter.StopAll()
	requireRecovery(t, limiter, 0)
}

func TestCPULimiterAutomaticValidation(t *testing.T) {
	request := recoveryRequest()
	request.Automatic = true
	signals := &recoverySignals{}
	validator := &recoveryValidator{automaticErr: errors.New("protected")}
	limiter := NewCPULimiter(signals, validator)
	if stopped := limiter.stopProcesses(request); len(stopped) != 0 {
		t.Fatal("automatic protection bypassed")
	}
	manual := request
	manual.Automatic = false
	if sameCPULimitRequest(request, manual) {
		t.Fatal("automatic flag ignored")
	}
	stopped := limiter.stopProcesses(manual)
	if len(stopped) != 1 {
		t.Fatal("automatic protection applied to manual request")
	}
	limiter.continueProcesses(stopped)
	// Optional interface absence retains fixture behavior.
	fixture := NewCPULimiter(signals)
	fixture.continueProcesses(fixture.stopProcesses(request))
	if validator.automaticCalls != 1 {
		t.Fatalf("automatic checks = %d", validator.automaticCalls)
	}
}

func TestCPULimiterAutomaticValidationRejectsWholeGroup(t *testing.T) {
	request := recoveryRequest()
	request.Automatic = true
	request.Processes = append(request.Processes, core.ProcessRef{ID: core.ProcessID{PID: 43, StartTime: time.Unix(1, 0)}})
	signals := &recoverySignals{}
	validator := &recoveryValidator{automaticErr: errors.New("exec changed to protected process"), automaticDeniedPID: 43}
	limiter := NewCPULimiter(signals, validator)
	if stopped := limiter.stopProcesses(request); len(stopped) != 0 {
		t.Fatal("unsafe second member allowed partial group stop")
	}
	if stops, _ := signals.counts(); stops != 0 {
		t.Fatalf("stops = %d, want zero before group preflight succeeds", stops)
	}
	if validator.automaticCalls != 2 {
		t.Fatalf("automatic checks = %d, want both members", validator.automaticCalls)
	}
	requireRecovery(t, limiter, 0)
	validator.automaticErr = nil
	stopped := limiter.stopProcesses(request)
	if len(stopped) != 2 {
		t.Fatal("safe group was not stopped after fresh preflight")
	}
	limiter.continueProcesses(stopped)
}

func TestCPULimiterUpdateReportsStopFailureAndClearsAfterSuccess(t *testing.T) {
	denied := errors.New("stop denied")
	signals := &recoverySignals{stopErr: denied}
	limiter := NewCPULimiter(signals)
	t.Cleanup(func() { limiter.StopAll() })
	request := recoveryRequest()
	limiter.Update([]CPULimitRequest{request})
	limiter.stopProcesses(request)
	failures := limiter.Update([]CPULimitRequest{request})
	if len(failures) != 1 || !errors.Is(failures[0].Err, denied) || failures[0].Action != core.ControlActionLimitCPU || !failures[0].Process.ID.SameGeneration(request.Processes[0].ID) {
		t.Fatalf("stop failures = %v", failures)
	}
	requireRecovery(t, limiter, 0)
	signals.mu.Lock()
	signals.stopErr = nil
	signals.mu.Unlock()
	stopped := limiter.stopProcesses(request)
	limiter.continueProcesses(stopped)
	if failures := limiter.Update([]CPULimitRequest{request}); len(failures) != 0 {
		t.Fatalf("successful stop retained failure: %v", failures)
	}
}

func TestCPULimiterUpdateReportsAutomaticPreflightFailure(t *testing.T) {
	denied := errors.New("protected second member")
	validator := &recoveryValidator{automaticErr: denied, automaticDeniedPID: 43}
	signals := &recoverySignals{}
	limiter := NewCPULimiter(signals, validator)
	t.Cleanup(func() { limiter.StopAll() })
	request := recoveryRequest()
	request.Automatic = true
	request.Processes = append(request.Processes, core.ProcessRef{ID: core.ProcessID{PID: 43}})
	limiter.stopProcesses(request)
	failures := limiter.Update([]CPULimitRequest{request})
	if len(failures) != 1 || !errors.Is(failures[0].Err, denied) || failures[0].Process.ID.PID != 43 || failures[0].Action != core.ControlActionLimitCPU {
		t.Fatalf("preflight failures = %v", failures)
	}
	if stops, _ := signals.counts(); stops != 0 {
		t.Fatal("failed preflight sent STOP")
	}
	if failures := limiter.Update(nil); len(failures) != 0 {
		t.Fatalf("removed request retained stop failure: %v", failures)
	}
}

func TestCPULimiterStopFailureClearsOnForeignStopOrExit(t *testing.T) {
	for _, exited := range []bool{false, true} {
		signals := &recoverySignals{stopErr: errors.New("denied")}
		validator := &recoveryValidator{}
		limiter := NewCPULimiter(signals, validator)
		request := recoveryRequest()
		limiter.stopProcesses(request)
		if exited {
			validator.setError(fmt.Errorf("wrapped: %w", ErrProcessExited))
		} else {
			signals.external = true
		}
		limiter.stopProcesses(request)
		if failures := limiter.Update([]CPULimitRequest{request}); len(failures) != 0 {
			t.Fatalf("foreign stop or exit retained failure: %v", failures)
		}
		limiter.StopAll()
	}
}
