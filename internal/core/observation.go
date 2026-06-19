package core

import "time"

type AppProcessHint struct {
	AppID          AppID     `json:"appID"`
	PrimaryPID     ProcessID `json:"primaryPID"`
	ExecutablePath string    `json:"executablePath,omitempty"`
	BundlePath     string    `json:"bundlePath,omitempty"`
	UID            int       `json:"uid,omitempty"`
}

type ProcessMetrics struct {
	Process         ProcessRef    `json:"process"`
	CPUSeconds      float64       `json:"cpuSeconds"`
	MemoryBytes     uint64        `json:"memoryBytes"`
	SampledAt       time.Time     `json:"sampledAt"`
	SamplerDuration time.Duration `json:"samplerDuration,omitempty"`
}

type ProcessCPUUsage struct {
	Process         ProcessRef    `json:"process"`
	CPUPercent      float64       `json:"cpuPercent"`
	CPUSecondsDelta float64       `json:"cpuSecondsDelta"`
	MemoryBytes     uint64        `json:"memoryBytes"`
	SampledAt       time.Time     `json:"sampledAt"`
	SampleWindow    time.Duration `json:"sampleWindow"`
}

type SystemCPUSample struct {
	User            uint64        `json:"user"`
	Nice            uint64        `json:"nice"`
	System          uint64        `json:"system"`
	Idle            uint64        `json:"idle"`
	SampledAt       time.Time     `json:"sampledAt"`
	SamplerDuration time.Duration `json:"samplerDuration,omitempty"`
}

type SystemCPUUsage struct {
	TotalPercent  float64       `json:"totalPercent"`
	UserPercent   float64       `json:"userPercent"`
	NicePercent   float64       `json:"nicePercent"`
	SystemPercent float64       `json:"systemPercent"`
	IdlePercent   float64       `json:"idlePercent"`
	SampledAt     time.Time     `json:"sampledAt"`
	SampleWindow  time.Duration `json:"sampleWindow"`
}

type ProcessMetricsSampler interface {
	SampleProcesses([]ProcessRef) ([]ProcessMetrics, error)
}

type SystemMetricsSampler interface {
	SampleSystemCPU() (SystemCPUSample, error)
}

type HistoryStore interface {
	Add(AppCPUSample)
	Window(time.Duration) []AppCPUSample
}
