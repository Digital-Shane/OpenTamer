package observe

import (
	"reflect"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestProcessGrouperGroupsStandaloneInstancesByAppID(t *testing.T) {
	start := time.Unix(100, 0)
	processes := []ProcessRef{
		{ID: ProcessID{PID: 12, StartTime: start}, UID: 501, Name: "worker", ExecutablePath: "/tmp/a/worker"},
		{ID: ProcessID{PID: 11, StartTime: start}, UID: 501, Name: "worker", ExecutablePath: "/tmp/b/worker"},
		{ID: ProcessID{PID: 10, StartTime: start}, UID: 501, Name: "worker", ExecutablePath: "/tmp/a/worker"},
		{ID: ProcessID{PID: 13, StartTime: start}, UID: 501, Name: "alpha", ExecutablePath: "/tmp/alpha"},
	}
	processes = append(processes, processes[0])
	duplicate := processes[2]
	duplicate.ID.StartTime = start.Add(time.Second)
	processes = append(processes, duplicate)

	grouper := NewProcessGrouper()
	groups := grouper.Group(processes, nil)
	if len(groups) != 3 {
		t.Fatalf("len(groups) = %d, want 3", len(groups))
	}
	if groups[0].DisplayName() != "alpha" {
		t.Fatalf("first group = %q, want alpha", groups[0].DisplayName())
	}
	want := map[string][]ProcessID{
		"path:/tmp/a/worker": {processes[2].ID, processes[0].ID},
		"path:/tmp/b/worker": {processes[1].ID},
		"path:/tmp/alpha":    {processes[3].ID},
	}
	total := 0
	for _, group := range groups {
		key := group.ID.Key()
		ids, ok := want[key]
		if !ok {
			t.Fatalf("unexpected or duplicate group key %q", key)
		}
		if !reflect.DeepEqual(group.ProcessIDs(), ids) {
			t.Fatalf("%s process IDs = %#v, want %#v", key, group.ProcessIDs(), ids)
		}
		total += len(group.ProcessIDs())
		delete(want, key)
	}
	if total != 4 || len(want) != 0 {
		t.Fatalf("total process IDs = %d, missing groups = %v", total, want)
	}

	for i := range processes {
		processes[i].ID.PID += 100
		processes[i].ID.StartTime = start.Add(time.Hour)
	}
	restarted := grouper.Group(processes, nil)
	if len(restarted) != len(groups) {
		t.Fatalf("restarted groups = %d, want %d", len(restarted), len(groups))
	}
	for _, group := range restarted {
		found := false
		for _, previous := range groups {
			if group.ID.Key() == previous.ID.Key() {
				found = true
			}
		}
		if !found {
			t.Fatalf("group key changed after restart: %q", group.ID.Key())
		}
	}
}

func TestProcessGrouperStandaloneInstancesUseMostRestrictiveSafety(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			processes := []ProcessRef{
				{ID: ProcessID{PID: 10}, UID: 501, Name: "worker", ExecutablePath: "/tmp/worker"},
				{ID: ProcessID{PID: 11}, UID: 0, Name: "worker", ExecutablePath: "/tmp/worker"},
			}
			grouper := NewProcessGrouper()
			want := grouper.Safety.ClassifyProcess(processes[1])
			if blocked {
				grouper.Safety.SelfPID = 11
				want = grouper.Safety.ClassifyProcess(processes[1])
			}
			if reverse {
				processes[0], processes[1] = processes[1], processes[0]
			}
			groups := grouper.Group(processes, nil)
			if len(groups) != 1 {
				t.Fatalf("blocked=%v reverse=%v: len(groups) = %d, want 1", blocked, reverse, len(groups))
			}
			if got := core.SafetyDecisionFromGroup(groups[0]); got != want {
				t.Fatalf("blocked=%v reverse=%v: safety = %#v, want %#v", blocked, reverse, got, want)
			}
		}
	}
}

func TestProcessGrouperPreservesHintedGroupWithStandaloneInstances(t *testing.T) {
	processes := []ProcessRef{
		{ID: ProcessID{PID: 10}, UID: 501, Name: "Example", ExecutablePath: "/tmp/Example"},
		{ID: ProcessID{PID: 11}, UID: 501, Name: "Example", ExecutablePath: "/tmp/Example"},
		{ID: ProcessID{PID: 12}, ParentPID: 10, UID: 501, Name: "helper", ExecutablePath: "/tmp/helper"},
		{ID: ProcessID{PID: 20}, UID: 501, Name: "worker", ExecutablePath: "/tmp/worker"},
		{ID: ProcessID{PID: 21}, UID: 501, Name: "worker", ExecutablePath: "/tmp/worker"},
	}
	id := AppID{BundleID: "com.example.app", Name: "Example"}
	hints := []AppProcessHint{{AppID: id, PrimaryPID: processes[0].ID, ExecutablePath: "/tmp/Example"}}
	groups := NewProcessGrouper().Group(processes, hints)
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].ID != id {
		t.Fatalf("hinted ID = %#v, want %#v", groups[0].ID, id)
	}
	for i, group := range groups {
		var want []ProcessID
		if i == 0 {
			want = []ProcessID{processes[0].ID, processes[1].ID, processes[2].ID}
		} else {
			want = []ProcessID{processes[3].ID, processes[4].ID}
		}
		if !reflect.DeepEqual(group.ProcessIDs(), want) {
			t.Fatalf("%s process IDs = %#v, want %#v", group.ID.Key(), group.ProcessIDs(), want)
		}
	}
}
