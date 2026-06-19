package core

import (
	"testing"
	"time"
)

func TestSafetyPolicyBlocksEssentialSystemProcess(t *testing.T) {
	decision := NewSafetyPolicy(0).ClassifyProcess(ProcessRef{
		ID:   ProcessID{PID: 88},
		UID:  0,
		Name: "WindowServer",
	})

	if decision.Controllability != ControllabilityBlocked {
		t.Fatalf("controllability = %q, want blocked", decision.Controllability)
	}
	if decision.Reason != SafetyReasonEssentialSystem {
		t.Fatalf("reason = %q", decision.Reason)
	}
	if decision.CanPause {
		t.Fatal("essential process must not be pausable")
	}
}

func TestSafetyPolicyBlocksOpenTamerSelf(t *testing.T) {
	decision := NewSafetyPolicy(123).ClassifyProcess(ProcessRef{
		ID:             ProcessID{PID: 123},
		UID:            501,
		Name:           "opentamer",
		ExecutablePath: "/Applications/OpenTamer.app/Contents/MacOS/opentamer",
	})

	if decision.Controllability != ControllabilityBlocked {
		t.Fatalf("controllability = %q, want blocked", decision.Controllability)
	}
	if decision.Reason != SafetyReasonOpenTamerSelf {
		t.Fatalf("reason = %q", decision.Reason)
	}
}

func TestSafetyPolicyMarksRootServiceSlowOnly(t *testing.T) {
	decision := NewSafetyPolicy(0).ClassifyProcess(ProcessRef{
		ID:             ProcessID{PID: 7},
		UID:            0,
		Name:           "backupd",
		ExecutablePath: "/System/Library/CoreServices/backupd",
	})

	if decision.Controllability != ControllabilitySlowOnly {
		t.Fatalf("controllability = %q, want slow only", decision.Controllability)
	}
	if decision.AllowsRule(RuleModePauseInBackground) {
		t.Fatal("slow-only process must not allow pause")
	}
	if !decision.AllowsRule(RuleModeLowerPriorityInBackground) {
		t.Fatal("slow-only process should allow priority lowering")
	}
}

func TestRuntimeStateRecoveryListsOnlyOwnedPauses(t *testing.T) {
	state := NewRuntimeState()
	process := ProcessRef{ID: ProcessID{PID: 42, StartTime: time.Unix(1, 0)}, Name: "sleep"}
	state.RecordPause(process, AppID{Name: "Sleep"}, ControlReasonUserRule, time.Unix(2, 0))

	owned := state.OwnedPausedProcesses()
	if len(owned) != 1 {
		t.Fatalf("len(owned) = %d, want 1", len(owned))
	}
	if owned[0].Process.ID.PID != 42 {
		t.Fatalf("pid = %d, want 42", owned[0].Process.ID.PID)
	}
}
