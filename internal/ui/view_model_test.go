package ui

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestBuildProcessRowsSortsAndFiltersUnsafeByDefault(t *testing.T) {
	normal := core.AppID{Name: "Editor"}
	blocked := core.AppID{Name: "WindowServer"}
	groups := []core.AppGroup{
		{
			ID:              blocked,
			Controllability: core.ControllabilityBlocked,
			SafetyReason:    core.SafetyReasonEssentialSystem,
			Status:          core.AppStatusBlockedBySafety,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 2}, Name: "WindowServer"}},
		},
		{
			ID:              normal,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 1}, Name: "Editor"}},
		},
	}
	samples := []core.AppCPUSample{{AppID: normal, CPUPercent: 30}}

	rows := BuildProcessRows(groups, samples, nil, false, 10)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Name != "Editor" || rows[0].CPUPercent != 30 {
		t.Fatalf("row = %#v", rows[0])
	}
}

func TestBuildAllProcessRowsSortsByName(t *testing.T) {
	groups := []core.AppGroup{
		{
			ID:              core.AppID{Name: "Zulu"},
			Controllability: core.ControllabilityNormal,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 2}, Name: "Zulu"}},
		},
		{
			ID:              core.AppID{Name: "Alpha"},
			Controllability: core.ControllabilityNormal,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 1}, Name: "Alpha"}},
		},
	}

	rows := BuildAllProcessRows(groups, nil, nil, false)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Name != "Alpha" || rows[1].Name != "Zulu" {
		t.Fatalf("rows not name sorted: %#v", rows)
	}
}

func TestBuildTopProcessRowsOnlyIncludesSampledCPUApps(t *testing.T) {
	active := core.AppID{Name: "Active"}
	idle := core.AppID{Name: "Idle"}
	groups := []core.AppGroup{
		{
			ID:              idle,
			Controllability: core.ControllabilityNormal,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 2}, Name: "Idle"}},
		},
		{
			ID:              active,
			Controllability: core.ControllabilityNormal,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 1}, Name: "Active"}},
		},
	}

	rows := BuildTopProcessRows(groups, []core.AppCPUSample{{AppID: active, CPUPercent: 10}}, nil, false, 10)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].AppID != active {
		t.Fatalf("row = %#v, want active app only", rows[0])
	}
}

func TestApplyTrackingToProcessRowsAnnotatesLocations(t *testing.T) {
	app := core.AppID{Name: "Editor"}
	rows := ApplyTrackingToProcessRows(
		[]ProcessRow{{AppID: app, AppKey: app.Key(), Name: "Editor"}},
		[]core.AppRule{{
			AppID:   app,
			Mode:    core.RuleModeObserveOnly,
			TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar},
		}},
	)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if !rows[0].TrackedInMenuBar || rows[0].TrackedInManagedApps {
		t.Fatalf("tracking flags = menu bar %v managed %v", rows[0].TrackedInMenuBar, rows[0].TrackedInManagedApps)
	}
}

func TestWithSystemCPUPercentNormalizesWithoutChangingProcessCPU(t *testing.T) {
	rows := []ProcessRow{{Name: "Editor", CPUPercent: 80}}

	normalized := WithSystemCPUPercent(rows, 8)

	if normalized[0].CPUPercent != 80 {
		t.Fatalf("process cpu = %v, want 80", normalized[0].CPUPercent)
	}
	if normalized[0].SystemCPUPercent != 10 {
		t.Fatalf("system cpu = %v, want 10", normalized[0].SystemCPUPercent)
	}
	if rows[0].SystemCPUPercent != 0 {
		t.Fatalf("input row was mutated: %#v", rows[0])
	}
}

func TestWithManagedSystemCPUPercentNormalizesWithoutChangingProcessCPU(t *testing.T) {
	rows := []ManagedAppRow{{Name: "Editor", CPUPercent: 80}}

	normalized := WithManagedSystemCPUPercent(rows, 8)

	if normalized[0].CPUPercent != 80 {
		t.Fatalf("process cpu = %v, want 80", normalized[0].CPUPercent)
	}
	if normalized[0].SystemCPUPercent != 10 {
		t.Fatalf("system cpu = %v, want 10", normalized[0].SystemCPUPercent)
	}
	if rows[0].SystemCPUPercent != 0 {
		t.Fatalf("input row was mutated: %#v", rows[0])
	}
}

func TestWithManagedAverageCPUUsesMatchingProcessAverage(t *testing.T) {
	app := core.AppID{Name: "Editor"}
	rows := []ManagedAppRow{{
		AppID:            app,
		AppKey:           app.Key(),
		Name:             "Editor",
		CPUPercent:       80,
		SystemCPUPercent: 10,
	}}
	averageRows := []ProcessRow{{
		AppID:            app,
		AppKey:           app.Key(),
		Name:             "Editor",
		CPUPercent:       24,
		SystemCPUPercent: 3,
		HasCPUSample:     true,
	}}

	withAverage := WithManagedAverageCPU(rows, averageRows)

	if withAverage[0].CPUPercent != 80 || withAverage[0].SystemCPUPercent != 10 {
		t.Fatalf("current CPU = process %v system %v, want 80 and 10", withAverage[0].CPUPercent, withAverage[0].SystemCPUPercent)
	}
	if withAverage[0].AverageCPUPercent != 24 || withAverage[0].AverageSystemCPUPercent != 3 {
		t.Fatalf("average CPU = process %v system %v, want 24 and 3", withAverage[0].AverageCPUPercent, withAverage[0].AverageSystemCPUPercent)
	}
	if rows[0].AverageCPUPercent != 0 || rows[0].AverageSystemCPUPercent != 0 {
		t.Fatalf("input row was mutated: %#v", rows[0])
	}
}

func TestWithCurrentCPUForAverageRowsKeepsAverageAndDisplaysCurrent(t *testing.T) {
	app := core.AppID{Name: "Editor"}
	averageRows := []ProcessRow{{
		AppID:            app,
		AppKey:           app.Key(),
		Name:             "Editor",
		CPUPercent:       40,
		SystemCPUPercent: 5,
	}}
	currentRows := []ProcessRow{{
		AppID:            app,
		AppKey:           app.Key(),
		Name:             "Editor",
		PID:              42,
		CPUPercent:       8,
		SystemCPUPercent: 1,
		HasCPUSample:     true,
		Status:           core.AppStatusObserved,
	}}

	rows := WithCurrentCPUForAverageRows(averageRows, currentRows)

	if rows[0].CPUPercent != 8 || rows[0].SystemCPUPercent != 1 {
		t.Fatalf("current CPU = process %v system %v, want 8 and 1", rows[0].CPUPercent, rows[0].SystemCPUPercent)
	}
	if rows[0].AverageCPUPercent != 40 || rows[0].AverageSystemCPUPercent != 5 {
		t.Fatalf("average CPU = process %v system %v, want 40 and 5", rows[0].AverageCPUPercent, rows[0].AverageSystemCPUPercent)
	}
	if rows[0].PID != 42 {
		t.Fatalf("pid = %d, want current pid 42", rows[0].PID)
	}
}

func TestWithAverageCPUForCurrentRowsKeepsCurrentOrder(t *testing.T) {
	appA := core.AppID{Name: "A"}
	appB := core.AppID{Name: "B"}
	currentRows := []ProcessRow{
		{AppID: appB, AppKey: appB.Key(), Name: "B", CPUPercent: 80, SystemCPUPercent: 10, HasCPUSample: true},
		{AppID: appA, AppKey: appA.Key(), Name: "A", CPUPercent: 40, SystemCPUPercent: 5, HasCPUSample: true},
	}
	averageRows := []ProcessRow{
		{AppID: appA, AppKey: appA.Key(), Name: "A", CPUPercent: 120, SystemCPUPercent: 15, HasCPUSample: true},
		{AppID: appB, AppKey: appB.Key(), Name: "B", CPUPercent: 16, SystemCPUPercent: 2, HasCPUSample: true},
	}

	rows := WithAverageCPUForCurrentRows(currentRows, averageRows)

	if rows[0].AppID != appB || rows[1].AppID != appA {
		t.Fatalf("row order = %#v, want current order B then A", rows)
	}
	if rows[0].CPUPercent != 80 || rows[0].SystemCPUPercent != 10 {
		t.Fatalf("B current CPU = process %v system %v, want 80 and 10", rows[0].CPUPercent, rows[0].SystemCPUPercent)
	}
	if rows[0].AverageCPUPercent != 16 || rows[0].AverageSystemCPUPercent != 2 {
		t.Fatalf("B average CPU = process %v system %v, want 16 and 2", rows[0].AverageCPUPercent, rows[0].AverageSystemCPUPercent)
	}
}

func TestAppendSystemCPUResidualRowAccountsForVisibleGap(t *testing.T) {
	rows := []ProcessRow{
		{Name: "A", CPUPercent: 40, SystemCPUPercent: 5},
		{Name: "B", CPUPercent: 16, SystemCPUPercent: 2},
	}

	rows = AppendSystemCPUResidualRow(rows, 20, 8, 3)

	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 with residual bucket", len(rows))
	}
	bucket := rows[2]
	if !bucket.SystemBucket || bucket.Name != "System / Other" {
		t.Fatalf("bucket = %#v, want system bucket", bucket)
	}
	if bucket.SystemCPUPercent != 13 {
		t.Fatalf("bucket system cpu = %v, want 13", bucket.SystemCPUPercent)
	}
	if bucket.CPUPercent != 104 {
		t.Fatalf("bucket process cpu = %v, want 104", bucket.CPUPercent)
	}
	if bucket.CanPause || bucket.CanSlow {
		t.Fatalf("bucket capabilities = pause %v slow %v, want both disabled", bucket.CanPause, bucket.CanSlow)
	}
}

func TestAppendSystemCPUResidualRowMakesRoomWhenLimited(t *testing.T) {
	rows := []ProcessRow{
		{Name: "A", CPUPercent: 40, SystemCPUPercent: 5},
		{Name: "B", CPUPercent: 16, SystemCPUPercent: 2},
	}

	rows = AppendSystemCPUResidualRow(rows, 20, 8, 2)

	if len(rows) != 2 {
		t.Fatalf("rows = %d, want limited rows with bucket", len(rows))
	}
	if rows[0].Name != "A" || !rows[1].SystemBucket {
		t.Fatalf("rows = %#v, want first app plus system bucket", rows)
	}
	if rows[1].SystemCPUPercent != 15 {
		t.Fatalf("bucket system cpu = %v, want 15", rows[1].SystemCPUPercent)
	}
}

func TestBuildMenuBarRowsIncludesMenuBarTrackedRulesOnly(t *testing.T) {
	menuBarApp := core.AppID{Name: "Menu Bar"}
	managedApp := core.AppID{Name: "Managed"}
	groups := []core.AppGroup{
		{
			ID:              menuBarApp,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 1}, Name: "Menu Bar"}},
		},
		{
			ID:              managedApp,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 2}, Name: "Managed"}},
		},
	}

	rows := BuildMenuBarRows(
		[]core.AppRule{
			{AppID: menuBarApp, Mode: core.RuleModeObserveOnly, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar}},
			{AppID: managedApp, Mode: core.RuleModeObserveOnly, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps}},
		},
		groups,
		[]core.AppCPUSample{{AppID: menuBarApp, CPUPercent: 7}},
		nil,
		false,
	)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].AppID != menuBarApp || rows[0].CPUPercent != 7 {
		t.Fatalf("row = %#v, want menu bar app at 7%%", rows[0])
	}
	if !rows[0].TrackedInMenuBar || rows[0].TrackedInManagedApps {
		t.Fatalf("tracking flags = menu bar %v tracked apps %v, want menu bar only", rows[0].TrackedInMenuBar, rows[0].TrackedInManagedApps)
	}
}

func TestBuildTrackedRowsIncludesObserveOnlyTrackedApps(t *testing.T) {
	trackedApp := core.AppID{Name: "Tracked"}
	managedApp := core.AppID{Name: "Managed"}
	groups := []core.AppGroup{
		{
			ID:              trackedApp,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusObserved,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 1}, Name: "Tracked"}},
		},
		{
			ID:              managedApp,
			Controllability: core.ControllabilityNormal,
			Status:          core.AppStatusPaused,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 2}, Name: "Managed"}},
		},
	}

	rows := BuildTrackedRows(
		[]core.AppRule{
			{AppID: trackedApp, Mode: core.RuleModeObserveOnly, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps}},
			{AppID: managedApp, Mode: core.RuleModePauseInBackground, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps}},
		},
		groups,
		[]core.AppCPUSample{{AppID: trackedApp, CPUPercent: 3}},
		nil,
		false,
	)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].AppID != trackedApp || rows[0].CPUPercent != 3 {
		t.Fatalf("row = %#v, want tracked app at 3%%", rows[0])
	}
	if rows[0].TrackedInMenuBar || !rows[0].TrackedInManagedApps {
		t.Fatalf("tracking flags = menu bar %v tracked apps %v, want tracked apps only", rows[0].TrackedInMenuBar, rows[0].TrackedInManagedApps)
	}
}

func TestBuildManagedRowsUsesStatuses(t *testing.T) {
	app := core.AppID{Name: "Editor"}
	rows := BuildManagedRows(
		[]core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
		[]core.AppGroup{{ID: app, Status: core.AppStatusPaused}},
		[]core.AppCPUSample{{AppID: app, CPUPercent: 20}},
		map[string]core.AppStatus{app.Key(): core.AppStatusPaused},
	)

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Status != core.AppStatusPaused {
		t.Fatalf("row = %#v", rows[0])
	}
	if rows[0].CPUPercent != 20 {
		t.Fatalf("cpu percent = %v, want 20", rows[0].CPUPercent)
	}
	if rows[0].RuleLabel != "Pause in Background" {
		t.Fatalf("rule label = %q", rows[0].RuleLabel)
	}
}

func TestBuildManagedRowsSkipsObserveOnlyTrackingRules(t *testing.T) {
	menuBarApp := core.AppID{Name: "Menu Bar"}
	trackedApp := core.AppID{Name: "Tracked"}
	managedApp := core.AppID{Name: "Managed"}
	rows := BuildManagedRows(
		[]core.AppRule{
			{AppID: menuBarApp, Mode: core.RuleModeObserveOnly, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInMenuBar}},
			{AppID: trackedApp, Mode: core.RuleModeObserveOnly, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps}},
			{AppID: managedApp, Mode: core.RuleModePauseInBackground, TrackIn: []core.RuleTrackingLocation{core.RuleTrackInManagedApps}},
		},
		nil,
		nil,
		nil,
	)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].AppID != managedApp {
		t.Fatalf("row = %#v, want managed app", rows[0])
	}
	if rows[0].RuleLabel != "Pause in Background" {
		t.Fatalf("rule label = %q, want Pause in Background", rows[0].RuleLabel)
	}
}

func TestBuildManagedRowsIncludesSafetyCapabilities(t *testing.T) {
	app := core.AppID{Name: "OpenTamer"}
	rows := BuildManagedRows(
		[]core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}},
		[]core.AppGroup{{
			ID:              app,
			Controllability: core.ControllabilityBlocked,
			SafetyReason:    core.SafetyReasonOpenTamerSelf,
			Processes:       []core.ProcessRef{{ID: core.ProcessID{PID: 42}, Name: "OpenTamer"}},
		}},
		nil,
		nil,
	)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].BlockerReason != string(core.SafetyReasonOpenTamerSelf) {
		t.Fatalf("blocker = %q, want %q", rows[0].BlockerReason, core.SafetyReasonOpenTamerSelf)
	}
	if rows[0].CanPause || rows[0].CanSlow {
		t.Fatalf("capabilities = pause %v slow %v, want both disabled", rows[0].CanPause, rows[0].CanSlow)
	}
}

func TestBuildManagedRowsLabelsPriorityScope(t *testing.T) {
	backgroundApp := core.AppID{Name: "Background"}
	alwaysApp := core.AppID{Name: "Always"}
	rows := BuildManagedRows(
		[]core.AppRule{
			{AppID: backgroundApp, Mode: core.RuleModeLowerPriorityInBackground, BackgroundOnly: true},
			{AppID: alwaysApp, Mode: core.RuleModeLowerPriorityInBackground, BackgroundOnly: false},
		},
		nil,
		nil,
		nil,
	)

	labels := map[string]string{}
	for _, row := range rows {
		labels[row.Name] = row.RuleLabel
	}
	if labels["Background"] != "Lower Priority in Background" {
		t.Fatalf("background label = %q", labels["Background"])
	}
	if labels["Always"] != "Lower Priority Always" {
		t.Fatalf("always label = %q", labels["Always"])
	}
}

func TestBuildProcessCPUGraphKeepsProcessLinesInRowOrder(t *testing.T) {
	appA := core.AppID{Name: "A"}
	appB := core.AppID{Name: "B"}
	at1 := time.Unix(10, 0)
	at2 := time.Unix(13, 0)

	graph := BuildProcessCPUGraph(
		[]core.AppCPUSample{
			{AppID: appA, CPUPercent: 10, SampledAt: at1},
			{AppID: appB, CPUPercent: 20, SampledAt: at1},
			{AppID: appA, CPUPercent: 30, SampledAt: at2},
			{AppID: appB, CPUPercent: 40, SampledAt: at2},
		},
		[]ProcessRow{
			{AppID: appB, AppKey: appB.Key(), Name: "B"},
			{AppID: appA, AppKey: appA.Key(), Name: "A"},
		},
		2,
	)

	if len(graph.Points) != 2 {
		t.Fatalf("aggregate points = %d, want 2", len(graph.Points))
	}
	if graph.Points[0].TotalCPU != 30 || graph.Points[1].TotalCPU != 70 {
		t.Fatalf("aggregate points = %#v, want totals 30 and 70", graph.Points)
	}
	if len(graph.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(graph.Lines))
	}
	if graph.Lines[0].Name != "B" || graph.Lines[1].Name != "A" {
		t.Fatalf("line order = %#v, want B then A", graph.Lines)
	}
	if graph.Lines[0].Points[1].AppCPU != 40 {
		t.Fatalf("B second point = %v, want 40", graph.Lines[0].Points[1].AppCPU)
	}
	if graph.Lines[0].Points[1].AtUnix != float64(at2.Unix()) {
		t.Fatalf("B second point atUnix = %v, want %v", graph.Lines[0].Points[1].AtUnix, float64(at2.Unix()))
	}
}

func TestWithGraphSystemCPUPercentNormalizesLinesAndAggregate(t *testing.T) {
	graph := WithGraphSystemCPUPercent(GraphSeries{
		Points: []GraphPoint{{TotalCPU: 80}},
		Lines: []GraphLine{{
			AppKey: "app",
			Name:   "App",
			Points: []GraphPoint{{AppCPU: 40}},
		}},
		Scale: "linear",
	}, 8)

	if graph.Points[0].TotalCPU != 10 {
		t.Fatalf("aggregate CPU = %v, want 10", graph.Points[0].TotalCPU)
	}
	if graph.Lines[0].Points[0].AppCPU != 5 {
		t.Fatalf("line CPU = %v, want 5", graph.Lines[0].Points[0].AppCPU)
	}
}

func TestWithGraphWindowStoresWindowBounds(t *testing.T) {
	start := time.Unix(100, 0)
	end := start.Add(5 * time.Minute)
	graph := WithGraphWindow(GraphSeries{Scale: "linear"}, start, end)

	if graph.WindowStartUnix != float64(start.Unix()) {
		t.Fatalf("window start = %v, want %v", graph.WindowStartUnix, float64(start.Unix()))
	}
	if graph.WindowEndUnix != float64(end.Unix()) {
		t.Fatalf("window end = %v, want %v", graph.WindowEndUnix, float64(end.Unix()))
	}
}
