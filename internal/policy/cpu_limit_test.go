package policy

import (
	"testing"
	"time"
)

func TestCPULimitPlannerCreatesDutyCycle(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 50})

	if !plan.Active {
		t.Fatal("expected active plan")
	}
	if plan.StopFor != 5*time.Second {
		t.Fatalf("stop for = %s, want 5s", plan.StopFor)
	}
	if plan.RunFor != 5*time.Second {
		t.Fatalf("run for = %s, want 5s", plan.RunFor)
	}
	if plan.Explanation == "" {
		t.Fatal("expected explanation")
	}
}

func TestCPULimitPlannerCapsBrowserStopWindow(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 10, BrowserLike: true})

	if plan.StopFor != 9*time.Second {
		t.Fatalf("browser stop window = %s, want 9s", plan.StopFor)
	}
}

func TestCPULimitPlannerAllowsPointZeroOnePercent(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: MinCPULimitPercent})

	if !plan.Active {
		t.Fatal("expected active plan")
	}
	if plan.RunFor != time.Millisecond {
		t.Fatalf("run for = %s, want 1ms", plan.RunFor)
	}
	if plan.StopFor != 9999*time.Millisecond {
		t.Fatalf("stop for = %s, want 9999ms", plan.StopFor)
	}
}

func TestCPULimitPlannerStopsWhenForeground(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 50, Foreground: true})

	if plan.Active {
		t.Fatal("foreground app should not be actively limited")
	}
	if plan.Reason != CPULimitReasonForeground {
		t.Fatalf("reason = %q", plan.Reason)
	}
}

func TestCPULimitPlannerHonorsKillSwitch(t *testing.T) {
	planner := DefaultCPULimitPlanner(false)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 50})

	if plan.Active {
		t.Fatal("disabled planner should not be active")
	}
	if plan.Reason != CPULimitReasonDisabled {
		t.Fatalf("reason = %q", plan.Reason)
	}
}
