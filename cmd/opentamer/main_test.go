//go:build darwin && cgo

package main

import (
	"testing"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestFinishShutdownWaitsForSuccessfulRecovery(t *testing.T) {
	recovered := false
	terminations := 0
	shutdown := func(reason core.ControlReason) bool {
		if reason != core.ControlReasonGlobalDisabled {
			t.Fatalf("shutdown reason = %q", reason)
		}
		return recovered
	}
	terminate := func() { terminations++ }
	if finishShutdown(shutdown, terminate) || terminations != 0 {
		t.Fatal("failed recovery terminated the application")
	}
	recovered = true
	if !finishShutdown(shutdown, terminate) || terminations != 1 {
		t.Fatal("successful recovery did not terminate the application once")
	}
}
