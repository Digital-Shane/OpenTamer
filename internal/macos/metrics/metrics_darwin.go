//go:build darwin && cgo

package metrics

/*
#include "metrics_bridge.h"
*/
import "C"

import (
	"fmt"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type MetricsSampler struct{}

func NewMetricsSampler() *MetricsSampler {
	return &MetricsSampler{}
}

func (sampler *MetricsSampler) SampleProcesses(processes []core.ProcessRef) ([]core.ProcessMetrics, error) {
	started := time.Now()
	sampledAt := started
	metrics := make([]core.ProcessMetrics, 0, len(processes))

	for _, process := range processes {
		if process.ID.PID <= 0 {
			continue
		}
		var native C.OpenTamerProcessMetrics
		if C.OpenTamerCopyProcessMetrics(C.int(process.ID.PID), &native) != 1 {
			continue
		}
		metrics = append(metrics, core.ProcessMetrics{
			Process:         process,
			CPUSeconds:      float64(native.cpu_time_ns) / 1_000_000_000,
			MemoryBytes:     uint64(native.resident_size),
			SampledAt:       sampledAt,
			SamplerDuration: time.Since(started),
		})
	}

	return metrics, nil
}

func (sampler *MetricsSampler) SampleSystemCPU() (core.SystemCPUSample, error) {
	started := time.Now()
	var native C.OpenTamerSystemCPULoad
	if C.OpenTamerCopySystemCPULoad(&native) != 1 {
		return core.SystemCPUSample{}, fmt.Errorf("sample system CPU")
	}
	return core.SystemCPUSample{
		User:            uint64(native.user),
		Nice:            uint64(native.nice),
		System:          uint64(native.system),
		Idle:            uint64(native.idle),
		SampledAt:       started,
		SamplerDuration: time.Since(started),
	}, nil
}
