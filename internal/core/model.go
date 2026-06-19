package core

import "time"

type AppKind string

const (
	AppKindUnknown       AppKind = "unknown"
	AppKindUserApp       AppKind = "user_app"
	AppKindHelper        AppKind = "helper"
	AppKindSystemService AppKind = "system_service"
	AppKindAccessibility AppKind = "accessibility"
	AppKindEssential     AppKind = "essential"
	AppKindProtected     AppKind = "protected"
)

type AppStatus string

const (
	AppStatusUnmanaged        AppStatus = "unmanaged"
	AppStatusObserved         AppStatus = "observed"
	AppStatusEligible         AppStatus = "eligible"
	AppStatusWaiting          AppStatus = "waiting"
	AppStatusPaused           AppStatus = "paused"
	AppStatusPriorityLowered  AppStatus = "priority_lowered"
	AppStatusCPULimited       AppStatus = "cpu_limited"
	AppStatusTemporarilyAwake AppStatus = "temporarily_awake"
	AppStatusBlockedBySafety  AppStatus = "blocked_by_safety"
	AppStatusBlockedByPolicy  AppStatus = "blocked_by_policy"
	AppStatusRestoring        AppStatus = "restoring"
	AppStatusUnsupported      AppStatus = "unsupported"
)

type Controllability string

const (
	ControllabilityUnknown  Controllability = "unknown"
	ControllabilityNormal   Controllability = "normal"
	ControllabilitySlowOnly Controllability = "slow_only"
	ControllabilityBlocked  Controllability = "blocked"
)

type AppGroup struct {
	ID              AppID           `json:"id"`
	Processes       []ProcessRef    `json:"processes"`
	Kind            AppKind         `json:"kind"`
	Controllability Controllability `json:"controllability"`
	SafetyReason    SafetyReason    `json:"safetyReason,omitempty"`
	Status          AppStatus       `json:"status"`
}

func (group AppGroup) DisplayName() string {
	if !group.ID.IsEmpty() {
		return group.ID.DisplayName()
	}
	if len(group.Processes) > 0 {
		return group.Processes[0].DisplayName()
	}
	return "Unknown App"
}

func (group AppGroup) ProcessIDs() []ProcessID {
	ids := make([]ProcessID, 0, len(group.Processes))
	for _, process := range group.Processes {
		ids = append(ids, process.ID)
	}
	return ids
}

type ProcessSample struct {
	Process     ProcessRef `json:"process"`
	CPUSeconds  float64    `json:"cpuSeconds"`
	MemoryBytes uint64     `json:"memoryBytes"`
	Timestamp   time.Time  `json:"timestamp"`
}

type AppCPUSample struct {
	AppID        AppID         `json:"appID"`
	ProcessIDs   []ProcessID   `json:"processIDs"`
	CPUPercent   float64       `json:"cpuPercent"`
	CPUSeconds   float64       `json:"cpuSeconds"`
	MemoryBytes  uint64        `json:"memoryBytes"`
	SampledAt    time.Time     `json:"sampledAt"`
	SampleWindow time.Duration `json:"sampleWindow"`
}

type AlertLevel string

const (
	AlertLevelNormal AlertLevel = "normal"
	AlertLevelHigh   AlertLevel = "high"
	AlertLevelOff    AlertLevel = "off"
)
