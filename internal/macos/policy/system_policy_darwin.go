//go:build darwin && cgo

package policy

/*
#cgo darwin LDFLAGS: -framework IOKit -framework Foundation -framework AppKit
#include <stdlib.h>
#include "system_policy_bridge.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type SystemPolicyObserver struct{}

func NewSystemPolicyObserver() *SystemPolicyObserver {
	return &SystemPolicyObserver{}
}

func (observer *SystemPolicyObserver) Snapshot() (core.SystemPolicyState, error) {
	payload, err := copyCString(C.OpenTamerCopySystemPolicyJSON())
	if err != nil {
		return core.SystemPolicyState{}, err
	}
	var native struct {
		OnACPower               bool     `json:"onACPower"`
		BatteryPercent          *float64 `json:"batteryPercent,omitempty"`
		UserIdleNanoseconds     uint64   `json:"userIdleNanoseconds"`
		LastWakeUnixNanoseconds uint64   `json:"lastWakeUnixNanoseconds,omitempty"`
	}
	if err := json.Unmarshal([]byte(payload), &native); err != nil {
		return core.SystemPolicyState{}, fmt.Errorf("decode system policy: %w", err)
	}
	state := core.SystemPolicyState{
		OnACPower:      native.OnACPower,
		BatteryPercent: native.BatteryPercent,
		UserIdleFor:    time.Duration(native.UserIdleNanoseconds),
	}
	if native.LastWakeUnixNanoseconds > 0 {
		state.LastWakeAt = time.Unix(0, int64(native.LastWakeUnixNanoseconds))
	}
	return state, nil
}
