package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

type ProcessRow struct {
	PID                     int            `json:"pid"`
	AppID                   core.AppID     `json:"appID"`
	AppKey                  string         `json:"appKey"`
	Name                    string         `json:"name"`
	CPUPercent              float64        `json:"cpuPercent"`
	SystemCPUPercent        float64        `json:"systemCPUPercent,omitempty"`
	AverageCPUPercent       float64        `json:"averageCPUPercent,omitempty"`
	AverageSystemCPUPercent float64        `json:"averageSystemCPUPercent,omitempty"`
	HasCPUSample            bool           `json:"hasCPUSample,omitempty"`
	Status                  core.AppStatus `json:"status"`
	BlockerReason           string         `json:"blockerReason,omitempty"`
	QuickAction             string         `json:"quickAction,omitempty"`
	CanPause                bool           `json:"canPause"`
	CanSlow                 bool           `json:"canSlow"`
	TrackedInMenuBar        bool           `json:"trackedInMenuBar,omitempty"`
	TrackedInManagedApps    bool           `json:"trackedInManagedApps,omitempty"`
	SystemBucket            bool           `json:"systemBucket,omitempty"`
}

type ManagedAppRow struct {
	Name                    string         `json:"name"`
	AppID                   core.AppID     `json:"appID"`
	AppKey                  string         `json:"appKey"`
	RuleMode                core.RuleMode  `json:"ruleMode"`
	RuleLabel               string         `json:"ruleLabel"`
	CPUPercent              float64        `json:"cpuPercent"`
	SystemCPUPercent        float64        `json:"systemCPUPercent,omitempty"`
	AverageCPUPercent       float64        `json:"averageCPUPercent,omitempty"`
	AverageSystemCPUPercent float64        `json:"averageSystemCPUPercent,omitempty"`
	Status                  core.AppStatus `json:"status"`
	BlockerReason           string         `json:"blockerReason,omitempty"`
	CanPause                bool           `json:"canPause"`
	CanSlow                 bool           `json:"canSlow"`
	TrackedInMenuBar        bool           `json:"trackedInMenuBar,omitempty"`
	TrackedInManagedApps    bool           `json:"trackedInManagedApps,omitempty"`
}

type GraphPoint struct {
	At       time.Time `json:"at"`
	AtUnix   float64   `json:"atUnix,omitempty"`
	TotalCPU float64   `json:"totalCPU"`
	AppCPU   float64   `json:"appCPU,omitempty"`
}

type GraphLine struct {
	AppKey string       `json:"appKey"`
	Name   string       `json:"name"`
	Points []GraphPoint `json:"points"`
}

type GraphSeries struct {
	Points          []GraphPoint `json:"points"`
	Lines           []GraphLine  `json:"lines"`
	Scale           string       `json:"scale"`
	WindowStartUnix float64      `json:"windowStartUnix,omitempty"`
	WindowEndUnix   float64      `json:"windowEndUnix,omitempty"`
}

type MenuBarViewState struct {
	Enabled         bool                   `json:"enabled"`
	ShowMenuBarIcon bool                   `json:"showMenuBarIcon"`
	Preferences     core.GlobalPreferences `json:"preferences"`
	TotalCPU        float64                `json:"totalCPU"`
	AlertLevel      core.AlertLevel        `json:"alertLevel"`
	StatusMessage   string                 `json:"statusMessage,omitempty"`
	LastUpdated     time.Time              `json:"lastUpdated"`
	CPUGraph        GraphSeries            `json:"cpuGraph"`
	MenuBarApps     []ProcessRow           `json:"menuBarApps"`
	TrackedApps     []ProcessRow           `json:"trackedApps"`
	TopProcesses    []ProcessRow           `json:"topProcesses"`
	AllProcesses    []ProcessRow           `json:"allProcesses"`
	ManagedApps     []ManagedAppRow        `json:"managedApps"`
}

func BuildProcessRows(groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, includeSystem bool, limit int) []ProcessRow {
	return buildProcessRows(groups, samples, statuses, includeSystem, limit, false)
}

func BuildTopProcessRows(groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, includeSystem bool, limit int) []ProcessRow {
	return buildProcessRows(groups, samples, statuses, includeSystem, limit, true)
}

func buildProcessRows(groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, includeSystem bool, limit int, requireSample bool) []ProcessRow {
	sampleByApp := make(map[string]core.AppCPUSample, len(samples))
	for _, sample := range samples {
		sampleByApp[sample.AppID.Key()] = sample
	}

	rows := make([]ProcessRow, 0, len(groups))
	for _, group := range groups {
		if !includeSystem && group.Controllability == core.ControllabilityBlocked {
			continue
		}
		sample, hasSample := sampleByApp[group.ID.Key()]
		if requireSample && (!hasSample || sample.CPUPercent <= 0) {
			continue
		}
		status := statuses[group.ID.Key()]
		if status == "" {
			status = group.Status
		}
		safety := core.SafetyDecisionFromGroup(group)
		rows = append(rows, ProcessRow{
			PID:           primaryPID(group),
			AppID:         group.ID,
			AppKey:        group.ID.Key(),
			Name:          group.DisplayName(),
			CPUPercent:    sample.CPUPercent,
			HasCPUSample:  hasSample,
			Status:        status,
			BlockerReason: string(group.SafetyReason),
			QuickAction:   quickActionForStatus(status),
			CanPause:      safety.AllowsRule(core.RuleModePauseInBackground),
			CanSlow:       safety.AllowsRule(core.RuleModeLowerPriorityInBackground),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CPUPercent == rows[j].CPUPercent {
			return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
		}
		return rows[i].CPUPercent > rows[j].CPUPercent
	})
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func BuildAllProcessRows(groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, includeSystem bool) []ProcessRow {
	rows := BuildProcessRows(groups, samples, statuses, includeSystem, 0)
	sort.Slice(rows, func(i, j int) bool {
		left := strings.ToLower(rows[i].Name)
		right := strings.ToLower(rows[j].Name)
		if left == right {
			return rows[i].PID < rows[j].PID
		}
		return left < right
	})
	return rows
}

func ApplyTrackingToProcessRows(rows []ProcessRow, rules []core.AppRule) []ProcessRow {
	next := append([]ProcessRow(nil), rows...)
	for i := range next {
		if rule, ok := matchingRuleForApp(next[i].AppID, rules); ok {
			next[i].TrackedInMenuBar = rule.TracksIn(core.RuleTrackInMenuBar)
			next[i].TrackedInManagedApps = rule.TracksIn(core.RuleTrackInManagedApps)
		}
	}
	return next
}

func WithSystemCPUPercent(rows []ProcessRow, logicalCPUCount int) []ProcessRow {
	if logicalCPUCount <= 0 {
		logicalCPUCount = 1
	}
	next := append([]ProcessRow(nil), rows...)
	for i := range next {
		next[i].SystemCPUPercent = next[i].CPUPercent / float64(logicalCPUCount)
	}
	return next
}

func WithManagedSystemCPUPercent(rows []ManagedAppRow, logicalCPUCount int) []ManagedAppRow {
	if logicalCPUCount <= 0 {
		logicalCPUCount = 1
	}
	next := append([]ManagedAppRow(nil), rows...)
	for i := range next {
		next[i].SystemCPUPercent = next[i].CPUPercent / float64(logicalCPUCount)
	}
	return next
}

func processRowsByAppKey(rows []ProcessRow) map[string]ProcessRow {
	rowsByApp := make(map[string]ProcessRow, len(rows))
	for _, row := range rows {
		key := row.AppKey
		if key == "" {
			key = row.AppID.Key()
		}
		if key == "" {
			continue
		}
		rowsByApp[key] = row
	}
	return rowsByApp
}

func WithCurrentCPUForAverageRows(averageRows []ProcessRow, currentRows []ProcessRow) []ProcessRow {
	currentByApp := processRowsByAppKey(currentRows)
	next := append([]ProcessRow(nil), averageRows...)
	for i := range next {
		key := next[i].AppKey
		if key == "" {
			key = next[i].AppID.Key()
		}
		next[i].AverageCPUPercent = next[i].CPUPercent
		next[i].AverageSystemCPUPercent = next[i].SystemCPUPercent
		current, ok := currentByApp[key]
		if !ok || !current.HasCPUSample {
			continue
		}
		next[i].PID = current.PID
		next[i].CPUPercent = current.CPUPercent
		next[i].SystemCPUPercent = current.SystemCPUPercent
		next[i].Status = current.Status
		next[i].BlockerReason = current.BlockerReason
		next[i].QuickAction = current.QuickAction
		next[i].CanPause = current.CanPause
		next[i].CanSlow = current.CanSlow
	}
	return next
}

func WithAverageCPUForCurrentRows(currentRows []ProcessRow, averageRows []ProcessRow) []ProcessRow {
	averageByApp := processRowsByAppKey(averageRows)
	next := append([]ProcessRow(nil), currentRows...)
	for i := range next {
		key := next[i].AppKey
		if key == "" {
			key = next[i].AppID.Key()
		}
		average, ok := averageByApp[key]
		if !ok || !average.HasCPUSample {
			next[i].AverageCPUPercent = next[i].CPUPercent
			next[i].AverageSystemCPUPercent = next[i].SystemCPUPercent
			continue
		}
		next[i].AverageCPUPercent = average.CPUPercent
		next[i].AverageSystemCPUPercent = average.SystemCPUPercent
	}
	return next
}

func WithManagedAverageCPU(rows []ManagedAppRow, averageRows []ProcessRow) []ManagedAppRow {
	averageByApp := processRowsByAppKey(averageRows)
	next := append([]ManagedAppRow(nil), rows...)
	for i := range next {
		key := next[i].AppKey
		if key == "" {
			key = next[i].AppID.Key()
		}
		average, ok := averageByApp[key]
		if !ok || !average.HasCPUSample {
			next[i].AverageCPUPercent = next[i].CPUPercent
			next[i].AverageSystemCPUPercent = next[i].SystemCPUPercent
			continue
		}
		next[i].AverageCPUPercent = average.CPUPercent
		next[i].AverageSystemCPUPercent = average.SystemCPUPercent
	}
	return next
}

func AppendSystemCPUResidualRow(rows []ProcessRow, totalSystemCPU float64, logicalCPUCount int, limit int) []ProcessRow {
	if logicalCPUCount <= 0 {
		logicalCPUCount = 1
	}

	next := append([]ProcessRow(nil), rows...)
	visibleRows := next
	if limit > 0 && len(next) >= limit {
		visibleRows = next[:limit-1]
	}

	visibleSystemCPU := 0.0
	for _, row := range visibleRows {
		visibleSystemCPU += row.SystemCPUPercent
	}
	residualSystemCPU := totalSystemCPU - visibleSystemCPU
	if residualSystemCPU <= 0.005 {
		return next
	}

	residualProcessCPU := residualSystemCPU * float64(logicalCPUCount)
	return append(visibleRows, ProcessRow{
		Name:                    "System / Other",
		CPUPercent:              residualProcessCPU,
		SystemCPUPercent:        residualSystemCPU,
		AverageCPUPercent:       residualProcessCPU,
		AverageSystemCPUPercent: residualSystemCPU,
		HasCPUSample:            true,
		Status:                  core.AppStatusObserved,
		BlockerReason:           "Unlisted processes and system CPU not attributed to sampled apps",
		CanPause:                false,
		CanSlow:                 false,
		SystemBucket:            true,
	})
}

func BuildMenuBarRows(rules []core.AppRule, groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, includeSystem bool) []ProcessRow {
	return buildRuleProcessRows(rules, groups, samples, statuses, includeSystem, func(rule core.AppRule) bool {
		return rule.TracksIn(core.RuleTrackInMenuBar)
	})
}

func BuildTrackedRows(rules []core.AppRule, groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, includeSystem bool) []ProcessRow {
	return buildRuleProcessRows(rules, groups, samples, statuses, includeSystem, func(rule core.AppRule) bool {
		return rule.Mode == core.RuleModeObserveOnly && rule.TracksIn(core.RuleTrackInManagedApps)
	})
}

func buildRuleProcessRows(rules []core.AppRule, groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus, includeSystem bool, includeRule func(core.AppRule) bool) []ProcessRow {
	groupByApp := make(map[string]core.AppGroup, len(groups))
	for _, group := range groups {
		groupByApp[group.ID.Key()] = group
	}
	sampleByApp := make(map[string]core.AppCPUSample, len(samples))
	for _, sample := range samples {
		sampleByApp[sample.AppID.Key()] = sample
	}

	rows := make([]ProcessRow, 0)
	for _, rule := range rules {
		if !includeRule(rule) {
			continue
		}
		group, ok := groupByApp[rule.AppID.Key()]
		if !ok {
			continue
		}
		if !includeSystem && group.Controllability == core.ControllabilityBlocked {
			continue
		}
		sample, hasSample := sampleByApp[group.ID.Key()]
		status := statuses[group.ID.Key()]
		if status == "" {
			status = group.Status
		}
		safety := core.SafetyDecisionFromGroup(group)
		rows = append(rows, ProcessRow{
			PID:                  primaryPID(group),
			AppID:                group.ID,
			AppKey:               group.ID.Key(),
			Name:                 group.DisplayName(),
			CPUPercent:           sample.CPUPercent,
			HasCPUSample:         hasSample,
			Status:               status,
			BlockerReason:        string(group.SafetyReason),
			QuickAction:          quickActionForStatus(status),
			CanPause:             safety.AllowsRule(core.RuleModePauseInBackground),
			CanSlow:              safety.AllowsRule(core.RuleModeLowerPriorityInBackground),
			TrackedInMenuBar:     rule.TracksIn(core.RuleTrackInMenuBar),
			TrackedInManagedApps: rule.TracksIn(core.RuleTrackInManagedApps),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		left := strings.ToLower(rows[i].Name)
		right := strings.ToLower(rows[j].Name)
		if left == right {
			return rows[i].PID < rows[j].PID
		}
		return left < right
	})
	return rows
}

func BuildManagedRows(rules []core.AppRule, groups []core.AppGroup, samples []core.AppCPUSample, statuses map[string]core.AppStatus) []ManagedAppRow {
	groupByApp := make(map[string]core.AppGroup, len(groups))
	for _, group := range groups {
		groupByApp[group.ID.Key()] = group
	}
	sampleByApp := make(map[string]core.AppCPUSample, len(samples))
	for _, sample := range samples {
		sampleByApp[sample.AppID.Key()] = sample
	}

	rows := make([]ManagedAppRow, 0, len(rules))
	for _, rule := range rules {
		if rule.Mode == core.RuleModeObserveOnly || !rule.TracksIn(core.RuleTrackInManagedApps) {
			continue
		}
		group := groupByApp[rule.AppID.Key()]
		name := rule.AppID.DisplayName()
		if !group.ID.IsEmpty() {
			name = group.DisplayName()
		}
		status := statuses[rule.AppID.Key()]
		if status == "" {
			status = group.Status
		}
		sample := sampleByApp[rule.AppID.Key()]
		canPause := true
		canSlow := true
		if !group.ID.IsEmpty() {
			safety := core.SafetyDecisionFromGroup(group)
			canPause = safety.AllowsRule(core.RuleModePauseInBackground)
			canSlow = safety.AllowsRule(core.RuleModeLowerPriorityInBackground)
		}
		rows = append(rows, ManagedAppRow{
			Name:                 name,
			AppID:                rule.AppID,
			AppKey:               rule.AppID.Key(),
			RuleMode:             rule.Mode,
			RuleLabel:            ruleLabel(rule),
			CPUPercent:           sample.CPUPercent,
			Status:               status,
			BlockerReason:        string(group.SafetyReason),
			CanPause:             canPause,
			CanSlow:              canSlow,
			TrackedInMenuBar:     rule.TracksIn(core.RuleTrackInMenuBar),
			TrackedInManagedApps: true,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	return rows
}

func matchingRuleForApp(app core.AppID, rules []core.AppRule) (core.AppRule, bool) {
	for _, rule := range rules {
		if rule.Matches(app) {
			return rule, true
		}
	}
	return core.AppRule{}, false
}

func ruleLabel(rule core.AppRule) string {
	switch rule.Mode {
	case core.RuleModeObserveOnly:
		return "Track Only"
	case core.RuleModePauseInBackground:
		return "Pause in Background"
	case core.RuleModeLowerPriorityInBackground:
		if rule.BackgroundOnly {
			return "Lower Priority in Background"
		}
		return "Lower Priority Always"
	case core.RuleModeLimitCPUInBackground:
		if rule.CPUPercent != nil {
			return fmt.Sprintf("Limit CPU to %s", apppolicy.FormatCPULimitPercent(*rule.CPUPercent))
		}
		return "Limit CPU"
	case core.RuleModeHideAfterIdle:
		return "Hide After Idle"
	case core.RuleModeQuitAfterIdle:
		return "Quit After Idle"
	default:
		return string(rule.Mode)
	}
}

func BuildProcessCPUGraph(samples []core.AppCPUSample, rows []ProcessRow, limit int) GraphSeries {
	if limit <= 0 {
		limit = 5
	}

	type graphApp struct {
		key  string
		name string
	}

	selected := make([]graphApp, 0, limit)
	seen := make(map[string]bool)
	for _, row := range rows {
		if row.SystemBucket {
			continue
		}
		key := row.AppKey
		if key == "" {
			key = row.AppID.Key()
		}
		if key == "" || seen[key] {
			continue
		}
		name := row.Name
		if name == "" {
			name = row.AppID.DisplayName()
		}
		selected = append(selected, graphApp{key: key, name: name})
		seen[key] = true
		if len(selected) == limit {
			break
		}
	}

	if len(selected) == 0 {
		type total struct {
			key   string
			name  string
			value float64
		}
		totalsByApp := make(map[string]*total)
		for _, sample := range samples {
			key := sample.AppID.Key()
			if key == "" || sample.SampledAt.IsZero() {
				continue
			}
			item := totalsByApp[key]
			if item == nil {
				item = &total{key: key, name: sample.AppID.DisplayName()}
				totalsByApp[key] = item
			}
			item.value += sample.CPUPercent
		}
		totals := make([]total, 0, len(totalsByApp))
		for _, item := range totalsByApp {
			totals = append(totals, *item)
		}
		sort.Slice(totals, func(i, j int) bool {
			if totals[i].value == totals[j].value {
				return strings.ToLower(totals[i].name) < strings.ToLower(totals[j].name)
			}
			return totals[i].value > totals[j].value
		})
		for _, item := range totals {
			selected = append(selected, graphApp{key: item.key, name: item.name})
			seen[item.key] = true
			if len(selected) == limit {
				break
			}
		}
	}

	pointsByTime := make(map[time.Time]GraphPoint)
	linePointsByApp := make(map[string]map[time.Time]GraphPoint, len(selected))
	for _, app := range selected {
		linePointsByApp[app.key] = make(map[time.Time]GraphPoint)
	}

	for _, sample := range samples {
		if sample.SampledAt.IsZero() {
			continue
		}
		aggregate := pointsByTime[sample.SampledAt]
		aggregate.At = sample.SampledAt
		aggregate.TotalCPU += sample.CPUPercent
		pointsByTime[sample.SampledAt] = aggregate

		linePoints := linePointsByApp[sample.AppID.Key()]
		if linePoints == nil {
			continue
		}
		point := linePoints[sample.SampledAt]
		point.At = sample.SampledAt
		point.AppCPU += sample.CPUPercent
		linePoints[sample.SampledAt] = point
	}

	lines := make([]GraphLine, 0, len(selected))
	for _, app := range selected {
		points := sortedGraphPoints(linePointsByApp[app.key])
		if len(points) == 0 {
			continue
		}
		lines = append(lines, GraphLine{
			AppKey: app.key,
			Name:   app.name,
			Points: points,
		})
	}

	return GraphSeries{Points: sortedGraphPoints(pointsByTime), Lines: lines, Scale: "linear"}
}

func WithGraphSystemCPUPercent(graph GraphSeries, logicalCPUCount int) GraphSeries {
	if logicalCPUCount <= 0 {
		logicalCPUCount = 1
	}
	next := GraphSeries{
		Points:          append([]GraphPoint(nil), graph.Points...),
		Lines:           make([]GraphLine, len(graph.Lines)),
		Scale:           graph.Scale,
		WindowStartUnix: graph.WindowStartUnix,
		WindowEndUnix:   graph.WindowEndUnix,
	}
	for i := range next.Points {
		next.Points[i].TotalCPU /= float64(logicalCPUCount)
		next.Points[i].AppCPU /= float64(logicalCPUCount)
	}
	for i, line := range graph.Lines {
		next.Lines[i] = GraphLine{
			AppKey: line.AppKey,
			Name:   line.Name,
			Points: append([]GraphPoint(nil), line.Points...),
		}
		for j := range next.Lines[i].Points {
			next.Lines[i].Points[j].TotalCPU /= float64(logicalCPUCount)
			next.Lines[i].Points[j].AppCPU /= float64(logicalCPUCount)
		}
	}
	return next
}

func WithGraphWindow(graph GraphSeries, start time.Time, end time.Time) GraphSeries {
	if !start.IsZero() {
		graph.WindowStartUnix = unixSeconds(start)
	}
	if !end.IsZero() {
		graph.WindowEndUnix = unixSeconds(end)
	}
	return graph
}

func sortedGraphPoints(pointsByTime map[time.Time]GraphPoint) []GraphPoint {
	points := make([]GraphPoint, 0, len(pointsByTime))
	for _, point := range pointsByTime {
		point.AtUnix = unixSeconds(point.At)
		points = append(points, point)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].At.Before(points[j].At)
	})
	return points
}

func unixSeconds(at time.Time) float64 {
	if at.IsZero() {
		return 0
	}
	return float64(at.UnixNano()) / float64(time.Second)
}

func primaryPID(group core.AppGroup) int {
	if len(group.Processes) == 0 {
		return 0
	}
	return group.Processes[0].ID.PID
}

func quickActionForStatus(status core.AppStatus) string {
	switch status {
	case core.AppStatusPaused:
		return "resume"
	case core.AppStatusObserved, core.AppStatusEligible, core.AppStatusUnmanaged:
		return "manage"
	case core.AppStatusBlockedBySafety, core.AppStatusBlockedByPolicy:
		return "details"
	default:
		return "edit"
	}
}
