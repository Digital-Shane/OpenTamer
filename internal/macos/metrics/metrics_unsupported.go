//go:build !darwin || !cgo

package metrics

import "github.com/Digital-Shane/open-tamer/internal/core"

type MetricsSampler struct{}

func NewMetricsSampler() *MetricsSampler {
	return &MetricsSampler{}
}

func (sampler *MetricsSampler) SampleProcesses(processes []core.ProcessRef) ([]core.ProcessMetrics, error) {
	return nil, ErrUnsupported
}

func (sampler *MetricsSampler) SampleSystemCPU() (core.SystemCPUSample, error) {
	return core.SystemCPUSample{}, ErrUnsupported
}
