package observe

import (
	"sort"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type ProcessMetrics = core.ProcessMetrics
type ProcessCPUUsage = core.ProcessCPUUsage
type SystemCPUSample = core.SystemCPUSample
type SystemCPUUsage = core.SystemCPUUsage
type AppCPUSample = core.AppCPUSample

type CPUUsageCalculator struct {
	previous map[string]ProcessMetrics
}

func NewCPUUsageCalculator() *CPUUsageCalculator {
	return &CPUUsageCalculator{previous: make(map[string]ProcessMetrics)}
}

func (calculator *CPUUsageCalculator) Update(samples []ProcessMetrics) []ProcessCPUUsage {
	if calculator.previous == nil {
		calculator.previous = make(map[string]ProcessMetrics)
	}

	next := make(map[string]ProcessMetrics, len(samples))
	usages := make([]ProcessCPUUsage, 0, len(samples))

	for _, current := range samples {
		key := current.Process.ID.Key()
		next[key] = current
		previous, ok := calculator.previous[key]
		if !ok {
			continue
		}
		window := current.SampledAt.Sub(previous.SampledAt)
		if window <= 0 {
			continue
		}
		delta := current.CPUSeconds - previous.CPUSeconds
		if delta < 0 {
			continue
		}
		usages = append(usages, ProcessCPUUsage{
			Process:         current.Process,
			CPUPercent:      (delta / window.Seconds()) * 100,
			CPUSecondsDelta: delta,
			MemoryBytes:     current.MemoryBytes,
			SampledAt:       current.SampledAt,
			SampleWindow:    window,
		})
	}

	calculator.previous = next
	sort.Slice(usages, func(i, j int) bool {
		return usages[i].CPUPercent > usages[j].CPUPercent
	})
	return usages
}

type SystemCPUCalculator struct {
	previous *SystemCPUSample
}

func (calculator *SystemCPUCalculator) Update(current SystemCPUSample) *SystemCPUUsage {
	if calculator.previous == nil {
		calculator.previous = &current
		return nil
	}
	previous := *calculator.previous
	calculator.previous = &current

	window := current.SampledAt.Sub(previous.SampledAt)
	if window <= 0 {
		return nil
	}

	user := current.User - previous.User
	nice := current.Nice - previous.Nice
	system := current.System - previous.System
	idle := current.Idle - previous.Idle
	total := user + nice + system + idle
	if total == 0 {
		return nil
	}

	active := total - idle
	return &SystemCPUUsage{
		TotalPercent:  float64(active) / float64(total) * 100,
		UserPercent:   float64(user) / float64(total) * 100,
		NicePercent:   float64(nice) / float64(total) * 100,
		SystemPercent: float64(system) / float64(total) * 100,
		IdlePercent:   float64(idle) / float64(total) * 100,
		SampledAt:     current.SampledAt,
		SampleWindow:  window,
	}
}

func AggregateAppCPU(groups []AppGroup, usages []ProcessCPUUsage) []AppCPUSample {
	usageByProcess := make(map[string]ProcessCPUUsage, len(usages))
	for _, usage := range usages {
		usageByProcess[usage.Process.ID.Key()] = usage
	}

	samples := make([]AppCPUSample, 0, len(groups))
	for _, group := range groups {
		sample := AppCPUSample{
			AppID:      group.ID,
			ProcessIDs: group.ProcessIDs(),
		}
		for _, process := range group.Processes {
			usage, ok := usageByProcess[process.ID.Key()]
			if !ok {
				continue
			}
			sample.CPUPercent += usage.CPUPercent
			sample.CPUSeconds += usage.CPUSecondsDelta
			sample.MemoryBytes += usage.MemoryBytes
			if usage.SampledAt.After(sample.SampledAt) {
				sample.SampledAt = usage.SampledAt
			}
			if usage.SampleWindow > sample.SampleWindow {
				sample.SampleWindow = usage.SampleWindow
			}
		}
		if sample.SampledAt.IsZero() {
			continue
		}
		samples = append(samples, sample)
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].CPUPercent > samples[j].CPUPercent
	})
	return samples
}

func SummarizeAppCPUWindow(samples []AppCPUSample, window time.Duration) []AppCPUSample {
	if len(samples) == 0 {
		return nil
	}

	latest := samples[0].SampledAt
	earliestStart := sampleStart(samples[0])
	for _, sample := range samples[1:] {
		if sample.SampledAt.After(latest) {
			latest = sample.SampledAt
		}
		start := sampleStart(sample)
		if start.Before(earliestStart) {
			earliestStart = start
		}
	}

	windowStart := earliestStart
	if window > 0 {
		cutoff := latest.Add(-window)
		if windowStart.Before(cutoff) {
			windowStart = cutoff
		}
	}
	denominator := latest.Sub(windowStart)
	if denominator <= 0 {
		return nil
	}

	type appAccumulator struct {
		sample         AppCPUSample
		latestSampleAt time.Time
	}

	byApp := make(map[string]*appAccumulator)
	for _, sample := range samples {
		cpuSeconds := cpuSecondsInsideWindow(sample, windowStart)
		key := sample.AppID.Key()
		accumulator := byApp[key]
		if accumulator == nil {
			accumulator = &appAccumulator{
				sample: AppCPUSample{
					AppID:        sample.AppID,
					SampledAt:    latest,
					SampleWindow: denominator,
				},
			}
			byApp[key] = accumulator
		}
		accumulator.sample.CPUSeconds += cpuSeconds
		if sample.SampledAt.After(accumulator.latestSampleAt) {
			accumulator.sample.ProcessIDs = append([]ProcessID(nil), sample.ProcessIDs...)
			accumulator.sample.MemoryBytes = sample.MemoryBytes
			accumulator.latestSampleAt = sample.SampledAt
		}
	}

	result := make([]AppCPUSample, 0, len(byApp))
	for _, accumulator := range byApp {
		accumulator.sample.CPUPercent = (accumulator.sample.CPUSeconds / denominator.Seconds()) * 100
		result = append(result, accumulator.sample)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CPUPercent > result[j].CPUPercent
	})
	return result
}

type CPUHistory struct {
	MaxAge     time.Duration
	MaxSamples int
	samples    []AppCPUSample
}

func NewCPUHistory(maxAge time.Duration, maxSamples int) *CPUHistory {
	return &CPUHistory{MaxAge: maxAge, MaxSamples: maxSamples}
}

func (history *CPUHistory) AddBatch(samples []AppCPUSample) {
	if len(samples) == 0 {
		return
	}
	history.samples = append(history.samples, samples...)
	history.prune(samples[len(samples)-1].SampledAt)
}

func (history *CPUHistory) Window(duration time.Duration) []AppCPUSample {
	if duration <= 0 || len(history.samples) == 0 {
		return append([]AppCPUSample(nil), history.samples...)
	}
	cutoff := history.samples[len(history.samples)-1].SampledAt.Add(-duration)
	result := make([]AppCPUSample, 0, len(history.samples))
	for _, sample := range history.samples {
		if !sample.SampledAt.Before(cutoff) {
			result = append(result, sample)
		}
	}
	return result
}

func (history *CPUHistory) prune(now time.Time) {
	if history.MaxAge > 0 {
		cutoff := now.Add(-history.MaxAge)
		kept := history.samples[:0]
		for _, sample := range history.samples {
			if !sample.SampledAt.Before(cutoff) {
				kept = append(kept, sample)
			}
		}
		history.samples = kept
	}
	if history.MaxSamples > 0 && len(history.samples) > history.MaxSamples {
		history.samples = history.samples[len(history.samples)-history.MaxSamples:]
	}
}

func sampleStart(sample AppCPUSample) time.Time {
	if sample.SampleWindow <= 0 {
		return sample.SampledAt
	}
	return sample.SampledAt.Add(-sample.SampleWindow)
}

func cpuSecondsInsideWindow(sample AppCPUSample, windowStart time.Time) float64 {
	if sample.SampleWindow <= 0 {
		return sample.CPUSeconds
	}

	start := sampleStart(sample)
	end := sample.SampledAt
	if !start.Before(windowStart) {
		return sample.CPUSeconds
	}
	if !end.After(windowStart) {
		return 0
	}

	included := end.Sub(windowStart)
	if included <= 0 {
		return 0
	}
	return sample.CPUSeconds * (included.Seconds() / sample.SampleWindow.Seconds())
}
