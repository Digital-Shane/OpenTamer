//go:build darwin && cgo

package app_test

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/app"
	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/macos"
)

// This opt-in check sends real signals only to its own disposable worker.
func TestGlobalCPULimitLive(t *testing.T) {
	if os.Getenv("OPENTAMER_LIVE_TEST") != "1" {
		t.Skip("set OPENTAMER_LIVE_TEST=1 to run the macOS CPU integration check")
	}
	workerCount := (runtime.NumCPU() + 39) / 40
	pids := make(map[int]bool, workerCount)
	for range workerCount {
		worker := exec.Command("/bin/sh", "-c", "while :; do :; done")
		if err := worker.Start(); err != nil {
			t.Fatal(err)
		}
		pids[worker.Process.Pid] = true
		t.Cleanup(func() {
			_ = worker.Process.Kill()
			_ = worker.Wait()
		})
	}

	adapters := macos.NewAdapters()
	adapters.Processes = liveWorkerEnumerator{base: adapters.Processes, pids: pids}
	adapters.Metrics = liveWorkerMetrics{base: adapters.Metrics}
	adapters.Audio = liveSilentAudio{}
	adapters.SystemPolicy = liveSystemPolicy{}
	store := config.NewStore(t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Preferences.WakeGrace = 0
	cfg.Preferences.HighCPUDetectionEnabled = false
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	controller, err := app.NewController(store, nil, adapters)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { controller.Shutdown(core.ControlReasonGlobalDisabled) })
	processes, err := adapters.Processes.Processes()
	if err != nil || len(processes) != workerCount {
		t.Fatalf("worker enumeration: %v, %d processes", err, len(processes))
	}
	measure := func(duration time.Duration) float64 {
		t.Helper()
		before, err := adapters.Metrics.SampleProcesses(processes)
		if err != nil || len(before) != workerCount {
			t.Fatalf("sample before: %v", err)
		}
		started := time.Now()
		for time.Since(started) < duration {
			controller.Refresh()
			time.Sleep(200 * time.Millisecond)
		}
		after, err := adapters.Metrics.SampleProcesses(processes)
		if err != nil || len(after) != workerCount {
			t.Fatalf("sample after: %v", err)
		}
		cpuSeconds := 0.0
		for i := range before {
			cpuSeconds += after[i].CPUSeconds - before[i].CPUSeconds
		}
		return cpuSeconds / time.Since(started).Seconds() * 100
	}
	baseline := measure(2 * time.Second)
	budget := 40 * float64(workerCount) / float64(runtime.NumCPU())
	controller.HandleMenuCommand("pref-float|globalCPUPercent|" + strconv.FormatFloat(budget, 'f', -1, 64))
	measure(4 * time.Second)
	limited := measure(5 * time.Second)
	controller.HandleMenuCommand("pref-clear|globalCPUPercent")
	controller.Refresh()
	restored := measure(2 * time.Second)
	t.Logf("worker CPU: baseline %.1f%%, limited %.1f%%, restored %.1f%%", baseline, limited, restored)
	if baseline < 70*float64(workerCount) {
		t.Fatalf("worker did not establish a busy baseline: %.1f%%", baseline)
	}
	want := 40 * float64(workerCount)
	if limited < want*0.6 || limited > want*1.3 {
		t.Errorf("worker CPU %.1f%% outside budget tolerance around %.1f%%", limited, want)
	}
	if restored < baseline*0.8 {
		t.Errorf("clearing global limit did not restore worker CPU: %.1f%% -> %.1f%%", baseline, restored)
	}
}

type liveWorkerEnumerator struct {
	base app.ProcessEnumerator
	pids map[int]bool
}

func (enumerator liveWorkerEnumerator) Processes() ([]core.ProcessRef, error) {
	processes, err := enumerator.base.Processes()
	workers := make([]core.ProcessRef, 0, len(enumerator.pids))
	for _, process := range processes {
		if enumerator.pids[process.ID.PID] {
			workers = append(workers, process)
		}
	}
	return workers, err
}

func (enumerator liveWorkerEnumerator) ProcessGeneration(pid int) (core.ProcessID, error) {
	return enumerator.base.(app.ProcessGenerationLookup).ProcessGeneration(pid)
}

type liveWorkerMetrics struct{ base app.MetricsSampler }

func (metrics liveWorkerMetrics) SampleProcesses(processes []core.ProcessRef) ([]core.ProcessMetrics, error) {
	return metrics.base.SampleProcesses(processes)
}

func (liveWorkerMetrics) SampleSystemCPU() (core.SystemCPUSample, error) {
	now := time.Now()
	return core.SystemCPUSample{Idle: uint64(now.UnixNano()), SampledAt: now}, nil
}

type liveSilentAudio struct{}

func (liveSilentAudio) AudioActive() (bool, error) { return false, nil }

type liveSystemPolicy struct{}

func (liveSystemPolicy) Snapshot() (core.SystemPolicyState, error) {
	return core.SystemPolicyState{}, nil
}
