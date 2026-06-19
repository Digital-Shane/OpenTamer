package observe

import (
	"testing"
	"time"
)

func TestCPUUsageCalculatorComputesDeltaPercent(t *testing.T) {
	process := ProcessRef{ID: ProcessID{PID: 1, StartTime: time.Unix(1, 0)}, Name: "worker"}
	calculator := NewCPUUsageCalculator()

	first := calculator.Update([]ProcessMetrics{
		{Process: process, CPUSeconds: 10, SampledAt: time.Unix(100, 0)},
	})
	if len(first) != 0 {
		t.Fatalf("first sample produced usage: %#v", first)
	}

	second := calculator.Update([]ProcessMetrics{
		{Process: process, CPUSeconds: 11.5, SampledAt: time.Unix(103, 0)},
	})
	if len(second) != 1 {
		t.Fatalf("len(second) = %d, want 1", len(second))
	}
	if second[0].CPUPercent != 50 {
		t.Fatalf("cpu percent = %v, want 50", second[0].CPUPercent)
	}
}

func TestCPUUsageCalculatorDoesNotSpikeOnExitedProcess(t *testing.T) {
	process := ProcessRef{ID: ProcessID{PID: 1, StartTime: time.Unix(1, 0)}, Name: "worker"}
	calculator := NewCPUUsageCalculator()
	calculator.Update([]ProcessMetrics{{Process: process, CPUSeconds: 10, SampledAt: time.Unix(100, 0)}})
	calculator.Update(nil)

	reappeared := ProcessRef{ID: ProcessID{PID: 1, StartTime: time.Unix(5, 0)}, Name: "worker"}
	usage := calculator.Update([]ProcessMetrics{{Process: reappeared, CPUSeconds: 100, SampledAt: time.Unix(110, 0)}})
	if len(usage) != 0 {
		t.Fatalf("expected no usage for new process generation, got %#v", usage)
	}
}

func TestAggregateAppCPUSumsHelperProcesses(t *testing.T) {
	app := AppID{BundleID: "com.example.app", Name: "Example"}
	processes := []ProcessRef{
		{ID: ProcessID{PID: 10, StartTime: time.Unix(1, 0)}, Name: "Example"},
		{ID: ProcessID{PID: 11, StartTime: time.Unix(2, 0)}, Name: "Example Helper"},
	}
	groups := []AppGroup{{ID: app, Processes: processes}}
	usages := []ProcessCPUUsage{
		{Process: processes[0], CPUPercent: 25, CPUSecondsDelta: 0.25, MemoryBytes: 10, SampledAt: time.Unix(5, 0), SampleWindow: time.Second},
		{Process: processes[1], CPUPercent: 15, CPUSecondsDelta: 0.15, MemoryBytes: 20, SampledAt: time.Unix(5, 0), SampleWindow: time.Second},
	}

	samples := AggregateAppCPU(groups, usages)
	if len(samples) != 1 {
		t.Fatalf("len(samples) = %d, want 1", len(samples))
	}
	if samples[0].CPUPercent != 40 {
		t.Fatalf("cpu percent = %v, want 40", samples[0].CPUPercent)
	}
	if samples[0].MemoryBytes != 30 {
		t.Fatalf("memory bytes = %d, want 30", samples[0].MemoryBytes)
	}
}

func TestSummarizeAppCPUWindowAveragesAcrossSharedWindow(t *testing.T) {
	appA := AppID{Name: "A"}
	appB := AppID{Name: "B"}
	samples := []AppCPUSample{
		{
			AppID:        appA,
			ProcessIDs:   []ProcessID{{PID: 1}},
			CPUSeconds:   3,
			MemoryBytes:  10,
			SampledAt:    time.Unix(3, 0),
			SampleWindow: 3 * time.Second,
		},
		{
			AppID:        appA,
			ProcessIDs:   []ProcessID{{PID: 1}},
			CPUSeconds:   0,
			MemoryBytes:  11,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 3 * time.Second,
		},
		{
			AppID:        appB,
			ProcessIDs:   []ProcessID{{PID: 2}},
			CPUSeconds:   1.5,
			MemoryBytes:  20,
			SampledAt:    time.Unix(6, 0),
			SampleWindow: 3 * time.Second,
		},
	}

	averages := SummarizeAppCPUWindow(samples, 6*time.Second)
	if len(averages) != 2 {
		t.Fatalf("len(averages) = %d, want 2", len(averages))
	}
	if averages[0].AppID != appA || averages[0].CPUPercent != 50 {
		t.Fatalf("first average = %#v, want app A at 50%%", averages[0])
	}
	if averages[1].AppID != appB || averages[1].CPUPercent != 25 {
		t.Fatalf("second average = %#v, want app B at 25%%", averages[1])
	}
	if averages[0].MemoryBytes != 11 {
		t.Fatalf("memory bytes = %d, want latest value 11", averages[0].MemoryBytes)
	}
}

func TestSummarizeAppCPUWindowClipsPartialSamples(t *testing.T) {
	app := AppID{Name: "A"}
	averages := SummarizeAppCPUWindow([]AppCPUSample{
		{
			AppID:        app,
			CPUSeconds:   4,
			SampledAt:    time.Unix(12, 0),
			SampleWindow: 4 * time.Second,
		},
		{
			AppID:        AppID{Name: "B"},
			CPUSeconds:   0,
			SampledAt:    time.Unix(20, 0),
			SampleWindow: 4 * time.Second,
		},
	}, 10*time.Second)

	if len(averages) != 2 {
		t.Fatalf("len(averages) = %d, want 2", len(averages))
	}
	for _, average := range averages {
		if average.AppID == app && average.CPUPercent != 20 {
			t.Fatalf("cpu percent = %v, want 20", average.CPUPercent)
		}
	}
}

func TestCPUHistoryAddBatchKeepsBoundedWindow(t *testing.T) {
	history := NewCPUHistory(3*time.Second, 0)
	history.AddBatch([]AppCPUSample{
		{AppID: AppID{Name: "App"}, CPUPercent: 1, SampledAt: time.Unix(1, 0)},
		{AppID: AppID{Name: "App"}, CPUPercent: 2, SampledAt: time.Unix(2, 0)},
		{AppID: AppID{Name: "App"}, CPUPercent: 3, SampledAt: time.Unix(5, 0)},
	})

	window := history.Window(10 * time.Second)
	if len(window) != 2 {
		t.Fatalf("len(window) = %d, want 2", len(window))
	}
	if window[0].CPUPercent != 2 || window[1].CPUPercent != 3 {
		t.Fatalf("retained samples = %#v, want latest two samples", window)
	}
}

func BenchmarkCPUHistoryPerProcessRefreshPattern(b *testing.B) {
	samples := cpuHistoryBenchmarkSamples(60, 200)

	b.Run("add-batch-per-refresh", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			history := NewCPUHistory(coreBenchmarkWindow, 0)
			for offset := 0; offset < len(samples); offset += coreBenchmarkProcessCount {
				history.AddBatch(samples[offset : offset+coreBenchmarkProcessCount])
			}
		}
	})
}

func TestSystemCPUCalculatorComputesActivePercent(t *testing.T) {
	calculator := &SystemCPUCalculator{}
	if usage := calculator.Update(SystemCPUSample{User: 10, System: 10, Idle: 80, SampledAt: time.Unix(1, 0)}); usage != nil {
		t.Fatalf("first system sample produced usage: %#v", usage)
	}

	usage := calculator.Update(SystemCPUSample{User: 20, System: 20, Idle: 160, SampledAt: time.Unix(2, 0)})
	if usage == nil {
		t.Fatal("expected second system sample to produce usage")
	}
	if usage.TotalPercent != 20 {
		t.Fatalf("total percent = %v, want 20", usage.TotalPercent)
	}
}

const (
	coreBenchmarkProcessCount = 200
	coreBenchmarkWindow       = 30 * time.Minute
)

func cpuHistoryBenchmarkSamples(ticks int, processes int) []AppCPUSample {
	samples := make([]AppCPUSample, 0, ticks*processes)
	start := time.Unix(100, 0)
	for tick := range ticks {
		sampledAt := start.Add(time.Duration(tick) * 3 * time.Second)
		for process := range processes {
			samples = append(samples, AppCPUSample{
				AppID:      AppID{Name: "App"},
				CPUPercent: float64(process % 100),
				SampledAt:  sampledAt,
			})
		}
	}
	return samples
}
