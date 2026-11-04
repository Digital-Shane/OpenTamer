package app

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/observe"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

type budgetTestMetrics struct {
	count   int
	failure string
	entered chan struct{}
	release chan struct{}
}

func (metrics *budgetTestMetrics) SampleProcesses(processes []core.ProcessRef) ([]core.ProcessMetrics, error) {
	if metrics.entered != nil {
		close(metrics.entered)
		<-metrics.release
		metrics.entered = nil
	}
	metrics.count++
	if metrics.failure == "process" {
		return nil, errors.New("process metrics unavailable")
	}
	return []core.ProcessMetrics{{Process: processes[0], CPUSeconds: float64(metrics.count * runtime.NumCPU()), SampledAt: time.Unix(int64(metrics.count), 0)}}, nil
}

func (metrics *budgetTestMetrics) SampleSystemCPU() (core.SystemCPUSample, error) {
	if metrics.failure == "system" {
		return core.SystemCPUSample{}, errors.New("system metrics unavailable")
	}
	return core.SystemCPUSample{User: uint64(metrics.count * 100), SampledAt: time.Unix(int64(metrics.count), 0)}, nil
}

type budgetTestProcesses struct{}

func (budgetTestProcesses) Processes() ([]core.ProcessRef, error) {
	return []core.ProcessRef{{ID: core.ProcessID{PID: 98765, StartTime: time.Unix(1, 0)}, UID: os.Getuid(), Name: "Budget Worker", ExecutablePath: "/tmp/budget-worker"}}, nil
}

type budgetTestAudio struct{ metrics *budgetTestMetrics }

func (audio budgetTestAudio) AudioActive() (bool, error) {
	if audio.metrics.failure == "audio" {
		return false, errors.New("audio unavailable")
	}
	return false, nil
}

type budgetTestPolicy struct{ metrics *budgetTestMetrics }

func (policy budgetTestPolicy) Snapshot() (core.SystemPolicyState, error) {
	if policy.metrics.failure == "policy" {
		return core.SystemPolicyState{}, errors.New("policy unavailable")
	}
	return core.SystemPolicyState{}, nil
}

func budgetTestController(t *testing.T) (*Controller, *budgetTestMetrics) {
	t.Helper()
	metrics := &budgetTestMetrics{}
	signals := &limiterSignals{}
	cfg := config.DefaultConfig()
	cfg.Preferences.AggregateByName = true
	cfg.Preferences.ManagementEnabled = true
	cfg.Preferences.CPULimiterEnabled = true
	cfg.Preferences.GlobalCPUPercent = floatPtr(1)
	cfg.Preferences.WakeGrace = 0
	cfg.Preferences.DisableWhenUserIdleLongerThan = time.Hour
	cfg.Preferences.HighCPUDetectionEnabled = false
	controller := &Controller{
		store: config.NewStore(t.TempDir()), cfg: cfg, runtime: core.NewRuntimeState(), stats: core.NewStatsDocument(),
		processes: budgetTestProcesses{}, metrics: metrics, audio: budgetTestAudio{metrics}, system: budgetTestPolicy{metrics},
		cpu: observe.NewCPUUsageCalculator(), systemCPU: &observe.SystemCPUCalculator{}, scheduler: apppolicy.NewScheduler(), highCPU: NewHighCPUDetector(),
		limiter: NewCPULimiter(signals), groupCtl: &AppGroupController{Signals: signals},
	}
	t.Cleanup(func() { controller.Shutdown(core.ControlReasonGlobalDisabled) })
	return controller, metrics
}

func TestGlobalCPUControllerTelemetryFailuresSuspendOnlyGlobal(t *testing.T) {
	for _, failure := range []string{"system", "process", "audio", "policy"} {
		t.Run(failure, func(t *testing.T) {
			controller, metrics := budgetTestController(t)
			controller.Refresh()
			controller.Refresh()
			if len(controller.limiter.entries) != 1 {
				t.Fatal("baseline global limiter was not established")
			}
			metrics.failure = failure
			controller.Refresh()
			if len(controller.limiter.entries) != 0 {
				t.Fatal("failed observation left global limiter running")
			}
			if failure == "system" && controller.lastTotalCPU != 100 {
				t.Fatal("last display reading was lost")
			}
			for _, state := range controller.runtime.AppStates {
				if state.CPULimitRunFor != 0 {
					t.Fatal("stale duty retained after observation failure")
				}
			}
		})
	}
}

func TestShutdownSerializesWithGatedRefreshAndPreventsRestart(t *testing.T) {
	for _, quit := range []bool{false, true} {
		t.Run(map[bool]string{false: "shutdown", true: "menu quit"}[quit], func(t *testing.T) {
			controller, metrics := budgetTestController(t)
			controller.Refresh()
			controller.Refresh()
			entered, release := make(chan struct{}), make(chan struct{})
			metrics.entered, metrics.release = entered, release
			refreshed := make(chan struct{})
			go func() { controller.Refresh(); close(refreshed) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("refresh did not reach gate")
			}
			stopped := make(chan struct{})
			go func() {
				if quit {
					controller.HandleMenuCommand("quit")
				} else {
					controller.Shutdown(core.ControlReasonGlobalDisabled)
				}
				close(stopped)
			}()
			select {
			case <-stopped:
				close(release)
				<-refreshed
				t.Fatal("shutdown passed in-flight refresh")
			case <-time.After(20 * time.Millisecond):
			}
			close(release)
			select {
			case <-refreshed:
			case <-time.After(time.Second):
				t.Fatal("refresh deadlocked")
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("shutdown deadlocked")
			}
			count := metrics.count
			controller.Refresh()
			if metrics.count != count || len(controller.limiter.entries) != 0 || !controller.stopped {
				t.Fatal("refresh restarted after shutdown")
			}
		})
	}
}
