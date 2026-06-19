//go:build darwin && cgo

package policy

import (
	"testing"
	"time"
)

func TestSystemPolicyObserverSnapshot(t *testing.T) {
	snapshot, err := NewSystemPolicyObserver().Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.UserIdleFor < 0 {
		t.Fatalf("idle duration = %s", snapshot.UserIdleFor)
	}
	if !snapshot.LastWakeAt.IsZero() && snapshot.LastWakeAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("invalid wake timestamp = %s", snapshot.LastWakeAt)
	}
}
