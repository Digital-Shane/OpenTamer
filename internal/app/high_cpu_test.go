package app

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestHighCPUDetectorWaitsForDuration(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := highCPUGroup(app, 42)
	detector := NewHighCPUDetector()
	config := HighCPUConfig{Enabled: true, Threshold: 50, Duration: 5 * time.Second}

	first := detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 80}}, nil, time.Unix(10, 0))
	if len(first) != 0 {
		t.Fatalf("first notices = %#v", first)
	}
	second := detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 80}}, nil, time.Unix(16, 0))
	if len(second) != 1 {
		t.Fatalf("second notices = %#v, want one", second)
	}
	if second[0].AppName != "Example" || second[0].CPUPercent != 80 {
		t.Fatalf("notice = %#v", second[0])
	}
}

func TestHighCPUDetectorHonorsCooldown(t *testing.T) {
	app := core.AppID{Name: "Example"}
	group := highCPUGroup(app, 42)
	detector := NewHighCPUDetector()
	config := HighCPUConfig{Enabled: true, Threshold: 50, Duration: time.Second, Cooldown: 10 * time.Second}

	detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 80}}, nil, time.Unix(1, 0))
	first := detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 80}}, nil, time.Unix(3, 0))
	second := detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 80}}, nil, time.Unix(5, 0))

	if len(first) != 1 {
		t.Fatalf("first notices = %#v", first)
	}
	if len(second) != 0 {
		t.Fatalf("cooldown notices = %#v", second)
	}
}

func TestHighCPUDetectorReportsSystemCPUPercent(t *testing.T) {
	app := core.AppID{Name: "Example"}
	group := highCPUGroup(app, 42)
	detector := NewHighCPUDetector()
	config := HighCPUConfig{
		Enabled:         true,
		Threshold:       70,
		Duration:        time.Second,
		LogicalCPUCount: 8,
	}

	detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 600}}, nil, time.Unix(1, 0))
	notices := detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 600}}, nil, time.Unix(3, 0))
	if len(notices) != 1 {
		t.Fatalf("notices = %#v, want one", notices)
	}
	if notices[0].CPUPercent != 75 {
		t.Fatalf("notice CPU = %v, want system CPU 75", notices[0].CPUPercent)
	}
}

func TestHighCPUDetectorComparesThresholdAgainstSystemCPUPercent(t *testing.T) {
	app := core.AppID{Name: "Example"}
	group := highCPUGroup(app, 42)
	detector := NewHighCPUDetector()
	config := HighCPUConfig{
		Enabled:         true,
		Threshold:       70,
		Duration:        time.Second,
		LogicalCPUCount: 8,
	}

	detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 400}}, nil, time.Unix(1, 0))
	notices := detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 400}}, nil, time.Unix(3, 0))
	if len(notices) != 0 {
		t.Fatalf("notices = %#v, want none because system CPU is 50%%", notices)
	}
}

func TestHighCPUDetectionDisabled(t *testing.T) {
	app := core.AppID{Name: "Example"}
	group := highCPUGroup(app, 42)
	detector := NewHighCPUDetector()
	config := HighCPUConfig{Enabled: false, Threshold: 50, Duration: time.Second}
	notices := detector.Update(config, []core.AppGroup{group}, []core.AppCPUSample{{AppID: app, CPUPercent: 80}}, nil, time.Unix(1, 0))
	if len(notices) != 0 {
		t.Fatalf("disabled notices = %#v", notices)
	}
}

func highCPUGroup(app core.AppID, pid int) core.AppGroup {
	return core.AppGroup{
		ID:              app,
		Controllability: core.ControllabilityNormal,
		Status:          core.AppStatusObserved,
		Processes: []core.ProcessRef{{
			ID:   core.ProcessID{PID: pid},
			Name: app.DisplayName(),
		}},
	}
}
