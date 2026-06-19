package observe

import (
	"testing"
	"time"
)

func TestProcessGrouperGroupsAppHelpers(t *testing.T) {
	start := time.Unix(100, 0)
	processes := []ProcessRef{
		{
			ID:             ProcessID{PID: 10, StartTime: start},
			UID:            501,
			Name:           "Example",
			ExecutablePath: "/Applications/Example.app/Contents/MacOS/Example",
			BundleID:       "com.example.app",
		},
		{
			ID:             ProcessID{PID: 11, StartTime: start.Add(time.Second)},
			ParentPID:      10,
			UID:            501,
			Name:           "Example Helper",
			ExecutablePath: "/Applications/Example.app/Contents/Frameworks/Example Helper.app/Contents/MacOS/Example Helper",
		},
	}
	hints := []AppProcessHint{
		{
			AppID:          AppID{BundleID: "com.example.app", Path: "/Applications/Example.app", Name: "Example"},
			PrimaryPID:     processes[0].ID,
			ExecutablePath: processes[0].ExecutablePath,
			BundlePath:     "/Applications/Example.app",
			UID:            501,
		},
	}

	groups := NewProcessGrouper().Group(processes, hints)
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Processes) != 2 {
		t.Fatalf("len(processes) = %d, want 2", len(groups[0].Processes))
	}
	if groups[0].Kind != AppKindUserApp {
		t.Fatalf("kind = %q", groups[0].Kind)
	}
}

func TestProcessGrouperClassifiesEssentialProcess(t *testing.T) {
	processes := []ProcessRef{
		{ID: ProcessID{PID: 88}, UID: 0, Name: "WindowServer"},
	}

	groups := NewProcessGrouper().Group(processes, nil)
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if groups[0].Kind != AppKindEssential {
		t.Fatalf("kind = %q, want essential", groups[0].Kind)
	}
	if groups[0].Controllability != ControllabilityBlocked {
		t.Fatalf("controllability = %q, want blocked", groups[0].Controllability)
	}
}

func TestProcessGrouperAggregatesProcessesByName(t *testing.T) {
	processes := []ProcessRef{
		{ID: ProcessID{PID: 10}, UID: 501, Name: "worker", ExecutablePath: "/tmp/a/worker"},
		{ID: ProcessID{PID: 11}, UID: 501, Name: "worker", ExecutablePath: "/tmp/b/worker"},
		{ID: ProcessID{PID: 12}, UID: 501, Name: "helper", ExecutablePath: "/tmp/helper"},
	}
	grouper := NewProcessGrouper()
	grouper.AggregateByName = true

	groups := grouper.Group(processes, []AppProcessHint{
		{AppID: AppID{BundleID: "com.example.a", Name: "App A"}, PrimaryPID: processes[0].ID},
		{AppID: AppID{BundleID: "com.example.b", Name: "App B"}, PrimaryPID: processes[1].ID},
	})

	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	worker := findGroupByName(groups, "worker")
	if len(worker.Processes) != 2 {
		t.Fatalf("worker processes = %#v, want two aggregated processes", worker.Processes)
	}
	if worker.ID.Key() != "name:worker" {
		t.Fatalf("worker key = %q, want name:worker", worker.ID.Key())
	}
}

func findGroupByName(groups []AppGroup, name string) AppGroup {
	for _, group := range groups {
		if group.DisplayName() == name {
			return group
		}
	}
	return AppGroup{}
}
