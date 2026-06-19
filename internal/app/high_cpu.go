package app

import (
	"runtime"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type HighCPUNotice struct {
	AppID       core.AppID    `json:"appID"`
	AppName     string        `json:"appName"`
	CPUPercent  float64       `json:"cpuPercent"`
	ExceededFor time.Duration `json:"exceededFor"`
	At          time.Time     `json:"at"`
}

type Notifier interface {
	NotifyHighCPU(HighCPUNotice) error
}

type HighCPUConfig struct {
	Enabled         bool
	Threshold       float64
	Duration        time.Duration
	Cooldown        time.Duration
	LogicalCPUCount int
}

type highCPUAppState struct {
	ExceededSince  time.Time
	LastNotifiedAt time.Time
}

type HighCPUDetector struct {
	states map[string]highCPUAppState
}

func NewHighCPUDetector() *HighCPUDetector {
	return &HighCPUDetector{states: make(map[string]highCPUAppState)}
}

func HighCPUConfigFromPreferences(preferences core.GlobalPreferences) HighCPUConfig {
	return HighCPUConfig{
		Enabled:         preferences.HighCPUDetectionEnabled,
		Threshold:       preferences.HighCPUThreshold,
		Duration:        preferences.HighCPUDuration,
		Cooldown:        preferences.HighCPUCooldown,
		LogicalCPUCount: runtime.NumCPU(),
	}
}

func (detector *HighCPUDetector) Update(config HighCPUConfig, groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, now time.Time) []HighCPUNotice {
	if detector.states == nil {
		detector.states = make(map[string]highCPUAppState)
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !config.Enabled || config.Threshold <= 0 || config.Duration <= 0 {
		return nil
	}

	notices := make([]HighCPUNotice, 0)
	for _, sample := range samples {
		key := sample.AppID.Key()
		state := detector.states[key]
		systemCPUPercent := systemNormalizedCPUPercent(sample.CPUPercent, config.LogicalCPUCount)
		if managedStatusExplainsCPU(statuses[key]) || systemCPUPercent < config.Threshold {
			state.ExceededSince = time.Time{}
			detector.states[key] = state
			continue
		}
		if state.ExceededSince.IsZero() {
			state.ExceededSince = now
			detector.states[key] = state
			continue
		}
		exceededFor := now.Sub(state.ExceededSince)
		if exceededFor < config.Duration {
			detector.states[key] = state
			continue
		}
		if config.Cooldown > 0 && !state.LastNotifiedAt.IsZero() && now.Sub(state.LastNotifiedAt) < config.Cooldown {
			detector.states[key] = state
			continue
		}

		notice := HighCPUNotice{
			AppID:       sample.AppID,
			AppName:     sample.AppID.DisplayName(),
			CPUPercent:  systemCPUPercent,
			ExceededFor: exceededFor,
			At:          now,
		}
		notices = append(notices, notice)
		state.LastNotifiedAt = now
		detector.states[key] = state
	}
	return notices
}

func systemNormalizedCPUPercent(cpuPercent float64, logicalCPUCount int) float64 {
	if logicalCPUCount <= 0 {
		logicalCPUCount = 1
	}
	return cpuPercent / float64(logicalCPUCount)
}

func managedStatusExplainsCPU(status core.AppStatus) bool {
	switch status {
	case core.AppStatusPaused, core.AppStatusPriorityLowered, core.AppStatusTemporarilyAwake:
		return true
	default:
		return false
	}
}
