//go:build darwin && cgo

package metrics

import (
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/observe"
)

func TestMetricsSamplerSamplesCurrentProcess(t *testing.T) {
	process := core.ProcessRef{ID: core.ProcessID{PID: os.Getpid()}, Name: "go-test"}
	metrics, err := NewMetricsSampler().SampleProcesses([]core.ProcessRef{process})
	if err != nil {
		t.Fatalf("sample processes: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("len(metrics) = %d, want 1", len(metrics))
	}
	if metrics[0].SampledAt.IsZero() {
		t.Fatal("expected sampled time")
	}
}

func TestMetricsSamplerSamplesSystemCPU(t *testing.T) {
	sample, err := NewMetricsSampler().SampleSystemCPU()
	if err != nil {
		t.Fatalf("sample system CPU: %v", err)
	}
	if sample.User+sample.Nice+sample.System+sample.Idle == 0 {
		t.Fatal("expected non-zero system CPU counters")
	}
}

func TestMetricsSamplerReportsBusyProcessCPU(t *testing.T) {
	yesPath, err := exec.LookPath("yes")
	if err != nil {
		t.Skip("yes command unavailable")
	}

	cmd := exec.Command(yesPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("start busy process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	process := core.ProcessRef{ID: core.ProcessID{PID: cmd.Process.Pid}, Name: "yes"}
	sampler := NewMetricsSampler()
	calculator := observe.NewCPUUsageCalculator()

	first, err := sampler.SampleProcesses([]core.ProcessRef{process})
	if err != nil {
		t.Fatalf("sample first process metrics: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("len(first) = %d, want 1", len(first))
	}
	calculator.Update(first)

	time.Sleep(500 * time.Millisecond)

	second, err := sampler.SampleProcesses([]core.ProcessRef{process})
	if err != nil {
		t.Fatalf("sample second process metrics: %v", err)
	}
	usage := calculator.Update(second)
	if len(usage) != 1 {
		t.Fatalf("len(usage) = %d, want 1", len(usage))
	}
	if usage[0].CPUPercent < 20 {
		t.Fatalf("cpu percent = %.2f, want >= 20", usage[0].CPUPercent)
	}
}
