//go:build !darwin || !cgo

package policy

import "github.com/Digital-Shane/open-tamer/internal/core"

type SystemPolicyObserver struct{}

func NewSystemPolicyObserver() *SystemPolicyObserver {
	return &SystemPolicyObserver{}
}

func (observer *SystemPolicyObserver) Snapshot() (core.SystemPolicyState, error) {
	return core.SystemPolicyState{}, ErrUnsupported
}
