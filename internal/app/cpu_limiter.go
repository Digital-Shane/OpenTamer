package app

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type CPULimitRequest struct {
	AppID     core.AppID
	Processes []core.ProcessRef
	RunFor    time.Duration
	StopFor   time.Duration
}

type stoppedProcessObserver interface {
	IsStopped(core.ProcessRef) (bool, error)
}

type CPULimiter struct {
	signals   SignalController
	validator ProcessGenerationValidator
	mu        sync.Mutex
	entries   map[string]*cpuLimitEntry
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

func (limiter *CPULimiter) Update(requests []CPULimitRequest) {
	if limiter == nil || limiter.signals == nil {
		return
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
}

func (limiter *CPULimiter) StopAll() {
	if limiter == nil {
		return
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	for key, entry := range limiter.entries {
		limiter.stopEntryLocked(key, entry)
	}
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
	// Wait until the worker has resumed its processes and can no longer stop
	// them or interfere with a replacement duty cycle.
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
		stopped := limiter.stopProcesses(request.Processes)
		if entry.waitForLimitPhase(ctx, limitPhaseStop, time.Now()) {
			limiter.continueProcesses(stopped)
			return
		}
		limiter.continueProcesses(stopped)
	}
}

func (limiter *CPULimiter) stopProcesses(processes []core.ProcessRef) []core.ProcessRef {
	stopped := make([]core.ProcessRef, 0, len(processes))
	for _, process := range processes {
		if err := limiter.validateProcess(process); err != nil {
			log.Printf("OpenTamer CPU limiter stop skipped: pid %d %s: %v", process.ID.PID, process.DisplayName(), err)
			continue
		}
		if observer, ok := limiter.signals.(stoppedProcessObserver); ok {
			alreadyStopped, err := observer.IsStopped(process)
			if err != nil {
				log.Printf("OpenTamer CPU limiter state skipped: pid %d: %v", process.ID.PID, err)
				continue
			}
			if alreadyStopped {
				continue
			}
		}
		if err := limiter.signals.Stop(process); err != nil {
			log.Printf("OpenTamer CPU limiter stop failure: pid %d %s: %v", process.ID.PID, process.DisplayName(), err)
		} else {
			stopped = append(stopped, process)
		}
	}
	return stopped
}

func (limiter *CPULimiter) continueProcesses(processes []core.ProcessRef) {
	for _, process := range processes {
		if err := limiter.validateProcess(process); err != nil {
			log.Printf("OpenTamer CPU limiter continue skipped: pid %d %s: %v", process.ID.PID, process.DisplayName(), err)
			continue
		}
		if err := limiter.signals.Continue(process); err != nil {
			log.Printf("OpenTamer CPU limiter continue failure: pid %d %s: %v", process.ID.PID, process.DisplayName(), err)
		}
	}
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
	if !left.AppID.Matches(right.AppID) || left.RunFor != right.RunFor || left.StopFor != right.StopFor {
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
