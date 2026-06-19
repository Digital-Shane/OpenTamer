package core

import "time"

const DefaultBackgroundNice = 20

type SystemPolicyState struct {
	OnACPower         bool          `json:"onACPower"`
	BatteryPercent    *float64      `json:"batteryPercent,omitempty"`
	UserIdleFor       time.Duration `json:"userIdleFor"`
	LastWakeAt        time.Time     `json:"lastWakeAt"`
	InStartupGrace    bool          `json:"inStartupGrace"`
	InWakeGrace       bool          `json:"inWakeGrace"`
	ManagementBlocked bool          `json:"managementBlocked"`
	BlockReason       ControlReason `json:"blockReason,omitempty"`
}

type SystemPolicyObserver interface {
	Snapshot() (SystemPolicyState, error)
}
