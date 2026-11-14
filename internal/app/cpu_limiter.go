package app

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type CPULimitRequest struct {
	AppID     core.AppID
	Processes []core.ProcessRef
	RunFor    time.Duration
	StopFor   time.Duration
	Automatic bool
}

type automaticProcessValidator interface {
	ValidateAutomaticProcess(core.ProcessRef) error
}

type stoppedProcessObserver interface {
	IsStopped(core.ProcessRef) (bool, error)
}

type CPULimiter struct {
	signals   SignalController
	validator ProcessGenerationValidator
	mu        sync.Mutex
	entries   map[string]*cpuLimitEntry

	// Workers never acquire mu. Ownership and signal operations are serialized
	// separately so retirement can wait for workers while holding mu.
	recoveryMu   sync.Mutex
	owned        map[string]cpuLimitOwnership
	stopFailures map[string]ControlFailure
}

type cpuLimitOwnership struct {
	process  core.ProcessRef
	recovery bool
}

type cpuLimitEntry struct {
	mu      sync.RWMutex
	request CPULimitRequest
	cancel  context.CancelFunc
	updated chan struct{}
	done    chan struct{}
}

func NewCPULimiter(signals SignalController, validators ...ProcessGenerationValidator) *CPULimiter {
	limiter := &CPULimiter{
		signals: signals,
		entries: make(map[string]*cpuLimitEntry),
	}
	if len(validators) > 0 {
		limiter.validator = validators[0]
	}
	return limiter
}

// Update retires obsolete workers, retries recovery once, then updates workers.
// It returns recovery errors from this pass and current stop errors.
func (limiter *CPULimiter) Update(requests []CPULimitRequest) []ControlFailure {
	if limiter == nil || limiter.signals == nil {
		return nil
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if limiter.entries == nil {
		limiter.entries = make(map[string]*cpuLimitEntry)
	}

	accepted := make(map[string]CPULimitRequest, len(requests))
	owned := make(map[int]bool)
	for _, request := range requests {
		if request.AppID.IsEmpty() || len(request.Processes) == 0 || request.RunFor <= 0 || request.StopFor <= 0 {
			continue
		}
		key := request.AppID.Key()
		if _, ok := accepted[key]; ok {
			continue
		}
		overlaps := false
		for _, process := range request.Processes {
			if owned[process.ID.PID] {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}
		for _, process := range request.Processes {
			owned[process.ID.PID] = true
		}
		accepted[key] = request
	}
	// Finish every old owner before starting workers for regrouped PIDs.
	for key, entry := range limiter.entries {
		request, ok := accepted[key]
		if !ok || !sameCPULimitProcesses(entry.snapshot(), request) {
			limiter.stopEntryLocked(key, entry)
		}
	}
	failures := limiter.RetryRecovery()
	for key, request := range accepted {
		if existing := limiter.entries[key]; existing != nil {
			existingRequest := existing.snapshot()
			if sameCPULimitRequest(existingRequest, request) {
				continue
			}
			existing.update(request)
			continue
		}
		limiter.startEntryLocked(key, request)
	}
	limiter.recoveryMu.Lock()
	defer limiter.recoveryMu.Unlock()
	active := make(map[string]bool)
	for _, request := range accepted {
		for _, process := range request.Processes {
			active[process.ID.Key()] = true
		}
	}
	for key, failure := range limiter.stopFailures {
		if !active[key] {
			delete(limiter.stopFailures, key)
		} else {
			failures = append(failures, failure)
		}
	}
	return failures
}

// StopAll joins workers and makes one recovery pass. Failed ownership is kept
// for later Update, RetryRecovery, or StopAll calls; no retry goroutine remains.
func (limiter *CPULimiter) StopAll() []ControlFailure {
	if limiter == nil {
		return nil
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	for _, entry := range limiter.entries {
		entry.cancel()
	}
	for key, entry := range limiter.entries {
		limiter.stopEntryLocked(key, entry)
	}
	limiter.recoveryMu.Lock()
	clear(limiter.stopFailures)
	limiter.recoveryMu.Unlock()
	return limiter.RetryRecovery()
}

func (limiter *CPULimiter) startEntryLocked(key string, request CPULimitRequest) {
	ctx, cancel := context.WithCancel(context.Background())
	entry := &cpuLimitEntry{
		request: cloneCPULimitRequest(request),
		cancel:  cancel,
		updated: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	limiter.entries[key] = entry
	go limiter.run(ctx, entry)
}

func (limiter *CPULimiter) stopEntryLocked(key string, entry *cpuLimitEntry) {
	if entry == nil {
		return
	}
	entry.cancel()
	// The worker leaves unresolved ownership in the limiter before exiting.
	<-entry.done
	delete(limiter.entries, key)
}

func (limiter *CPULimiter) run(ctx context.Context, entry *cpuLimitEntry) {
	defer close(entry.done)
	for {
		if entry.waitForLimitPhase(ctx, limitPhaseRun, time.Now()) {
			return
		}
		request := entry.snapshot()
		limiter.retryProcessRecovery(request.Processes)
		if ctx.Err() != nil {
			return
		}
		stopped := limiter.stopProcesses(request)
		if entry.waitForLimitPhase(ctx, limitPhaseStop, time.Now()) {
			limiter.markRecovery(stopped)
			return
		}
		limiter.continueProcesses(stopped)
	}
}

func (limiter *CPULimiter) stopProcesses(request CPULimitRequest) []core.ProcessRef {
	limiter.recoveryMu.Lock()
	defer limiter.recoveryMu.Unlock()
	if limiter.owned == nil {
		limiter.owned = make(map[string]cpuLimitOwnership)
	}
	// A group cannot start another stopped phase until every member is recovered.
	for _, process := range request.Processes {
		if ownership, ok := limiter.owned[process.ID.Key()]; ok && ownership.recovery {
			return nil
		}
	}
	if validator, ok := limiter.validator.(automaticProcessValidator); request.Automatic && ok {
		for _, process := range request.Processes {
			if err := validator.ValidateAutomaticProcess(process); err != nil {
				limiter.recordStopFailureLocked(process, err)
				log.Printf("OpenTamer CPU limiter automatic stop skipped: pid %d: %v", process.ID.PID, err)
				return nil
			}
		}
	}
	stopped := make([]core.ProcessRef, 0, len(request.Processes))
	for _, process := range request.Processes {
		if _, owned := limiter.owned[process.ID.Key()]; owned {
			continue
		}
		if err := limiter.validateProcess(process); err != nil {
			limiter.recordStopFailureLocked(process, err)
			log.Printf("OpenTamer CPU limiter stop skipped: pid %d %s: %v", process.ID.PID, process.DisplayName(), err)
			continue
		}
		if observer, ok := limiter.signals.(stoppedProcessObserver); ok {
			alreadyStopped, err := observer.IsStopped(process)
			if err != nil {
				limiter.recordStopFailureLocked(process, err)
				log.Printf("OpenTamer CPU limiter state skipped: pid %d: %v", process.ID.PID, err)
				continue
			}
			if alreadyStopped {
				limiter.recordStopFailureLocked(process, nil)
				continue
			}
		}
		err := limiter.signals.Stop(process)
		limiter.recordStopFailureLocked(process, err)
		if err != nil {
			log.Printf("OpenTamer CPU limiter stop failure: pid %d %s: %v", process.ID.PID, process.DisplayName(), err)
		} else {
			limiter.owned[process.ID.Key()] = cpuLimitOwnership{process: process}
			stopped = append(stopped, process)
		}
	}
	return stopped
}

func (limiter *CPULimiter) recordStopFailureLocked(process core.ProcessRef, err error) {
	if !shouldReportControlFailure(err) {
		delete(limiter.stopFailures, process.ID.Key())
		return
	}
	if limiter.stopFailures == nil {
		limiter.stopFailures = make(map[string]ControlFailure)
	}
	limiter.stopFailures[process.ID.Key()] = ControlFailure{Process: process, Action: core.ControlActionLimitCPU, Err: err}
}

func (limiter *CPULimiter) continueProcesses(processes []core.ProcessRef) {
	limiter.recoveryMu.Lock()
	defer limiter.recoveryMu.Unlock()
	for _, process := range processes {
		if err := limiter.recoverProcessLocked(process); err != nil {
			log.Printf("OpenTamer CPU limiter recovery failure: pid %d %s: %v", process.ID.PID, process.DisplayName(), err)
		}
	}
}

func (limiter *CPULimiter) markRecovery(processes []core.ProcessRef) {
	limiter.recoveryMu.Lock()
	defer limiter.recoveryMu.Unlock()
	for _, process := range processes {
		if ownership, ok := limiter.owned[process.ID.Key()]; ok {
			ownership.recovery = true
			limiter.owned[process.ID.Key()] = ownership
		}
	}
}

// PendingRecovery snapshots unresolved ownership, excluding healthy stop phases.
func (limiter *CPULimiter) PendingRecovery() []core.ProcessRef {
	if limiter == nil {
		return nil
	}
	limiter.recoveryMu.Lock()
	defer limiter.recoveryMu.Unlock()
	return limiter.pendingRecoveryLocked()
}

func (limiter *CPULimiter) pendingRecoveryLocked() []core.ProcessRef {
	var pending []core.ProcessRef
	for _, ownership := range limiter.owned {
		if ownership.recovery {
			pending = append(pending, ownership.process)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].ID.Key() < pending[j].ID.Key() })
	return pending
}

// RetryRecovery makes one synchronous attempt per pending generation, without
// touching healthy stopped phases or depending on the current requests.
func (limiter *CPULimiter) RetryRecovery() []ControlFailure {
	if limiter == nil {
		return nil
	}
	limiter.recoveryMu.Lock()
	defer limiter.recoveryMu.Unlock()
	return limiter.retryProcessRecoveryLocked(limiter.pendingRecoveryLocked())
}

func (limiter *CPULimiter) retryProcessRecovery(processes []core.ProcessRef) {
	limiter.recoveryMu.Lock()
	defer limiter.recoveryMu.Unlock()
	for _, failure := range limiter.retryProcessRecoveryLocked(processes) {
		log.Printf("OpenTamer CPU limiter recovery failure: %s", failure.Error())
	}
}

func (limiter *CPULimiter) retryProcessRecoveryLocked(processes []core.ProcessRef) []ControlFailure {
	var failures []ControlFailure
	for _, process := range processes {
		if ownership, ok := limiter.owned[process.ID.Key()]; !ok || !ownership.recovery {
			continue
		}
		if err := limiter.recoverProcessLocked(process); err != nil {
			failures = append(failures, ControlFailure{Process: process, Action: core.ControlActionResume, Err: err})
		}
	}
	return failures
}

func (limiter *CPULimiter) recoverProcessLocked(process core.ProcessRef) error {
	key := process.ID.Key()
	ownership, ok := limiter.owned[key]
	if !ok {
		return nil
	}
	err := limiter.validateProcess(process)
	if err == nil {
		err = limiter.signals.Continue(process)
	}
	if err == nil || shouldClearStaleProcessState(err) {
		delete(limiter.owned, key)
		return nil
	}
	ownership.recovery = true
	limiter.owned[key] = ownership
	return err
}

func (limiter *CPULimiter) validateProcess(process core.ProcessRef) error {
	if limiter == nil || limiter.validator == nil {
		return nil
	}
	return limiter.validator.ValidateProcessGeneration(process)
}

type limitPhase int

const (
	limitPhaseRun limitPhase = iota
	limitPhaseStop
)

func (entry *cpuLimitEntry) waitForLimitPhase(ctx context.Context, phase limitPhase, started time.Time) bool {
	for {
		if ctx.Err() != nil {
			return true
		}
		duration := entry.phaseDuration(phase)
		remaining := duration - time.Since(started)
		if remaining <= 0 {
			return false
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			stopLimitTimer(timer)
			return true
		case <-entry.updated:
			stopLimitTimer(timer)
			continue
		case <-timer.C:
			return false
		}
	}
}

func (entry *cpuLimitEntry) phaseDuration(phase limitPhase) time.Duration {
	request := entry.snapshot()
	if phase == limitPhaseStop {
		return request.StopFor
	}
	return request.RunFor
}

func stopLimitTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func sameCPULimitRequest(left, right CPULimitRequest) bool {
	if !left.AppID.Matches(right.AppID) || left.RunFor != right.RunFor || left.StopFor != right.StopFor || left.Automatic != right.Automatic {
		return false
	}
	return sameCPULimitProcesses(left, right)
}

func sameCPULimitProcesses(left, right CPULimitRequest) bool {
	if !left.AppID.Matches(right.AppID) {
		return false
	}
	if len(left.Processes) != len(right.Processes) {
		return false
	}
	for i := range left.Processes {
		if !left.Processes[i].ID.SameGeneration(right.Processes[i].ID) {
			return false
		}
	}
	return true
}

func (entry *cpuLimitEntry) update(request CPULimitRequest) {
	entry.mu.Lock()
	entry.request = cloneCPULimitRequest(request)
	entry.mu.Unlock()

	entry.notifyUpdated()
}

func (entry *cpuLimitEntry) snapshot() CPULimitRequest {
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return cloneCPULimitRequest(entry.request)
}

func (entry *cpuLimitEntry) notifyUpdated() {
	select {
	case entry.updated <- struct{}{}:
	default:
	}
}

func cloneCPULimitRequest(request CPULimitRequest) CPULimitRequest {
	request.Processes = append([]core.ProcessRef(nil), request.Processes...)
	return request
}
