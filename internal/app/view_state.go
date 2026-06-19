package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/observe"
	"github.com/Digital-Shane/open-tamer/internal/ui"
)

const (
	topProcessesWindow   = 60 * time.Second
	topProcessesLimit    = 10
	cpuGraphProcessLimit = topProcessesLimit
)

func (controller *Controller) buildMenuBarStateLocked(now time.Time, status string) ui.MenuBarViewState {
	cfg := controller.cfg
	alert := controller.systemAlertLevelLocked(now)
	logicalCPUCount := runtime.NumCPU()
	allRows := ui.WithSystemCPUPercent(ui.ApplyTrackingToProcessRows(ui.BuildAllProcessRows(controller.lastGroups, controller.lastAppSamples, controller.lastStatuses, true), cfg.Rules), logicalCPUCount)
	currentRows := ui.WithSystemCPUPercent(ui.ApplyTrackingToProcessRows(controller.topProcessRowsLocked(now), cfg.Rules), logicalCPUCount)
	averageRows := ui.WithSystemCPUPercent(ui.ApplyTrackingToProcessRows(controller.averageProcessRowsLocked(now), cfg.Rules), logicalCPUCount)
	rows := ui.WithAverageCPUForCurrentRows(currentRows, averageRows)
	if core.NormalizeTopProcessesSortMode(cfg.Preferences.TopProcessesSort) == core.TopProcessesSortAverage {
		rows = ui.WithCurrentCPUForAverageRows(limitProcessRows(averageRows, topProcessesLimit), allRows)
	}
	rows = ui.AppendSystemCPUResidualRow(rows, controller.lastTotalCPU, logicalCPUCount, topProcessesLimit)
	menuBarRows := ui.WithSystemCPUPercent(ui.BuildMenuBarRows(cfg.Rules, controller.lastGroups, controller.lastAppSamples, controller.lastStatuses, true), logicalCPUCount)
	menuBarRows = ui.WithAverageCPUForCurrentRows(menuBarRows, averageRows)
	tracked := ui.WithSystemCPUPercent(ui.BuildTrackedRows(cfg.Rules, controller.lastGroups, controller.lastAppSamples, controller.lastStatuses, true), logicalCPUCount)
	tracked = ui.WithAverageCPUForCurrentRows(tracked, averageRows)
	managed := ui.WithManagedSystemCPUPercent(ui.BuildManagedRows(cfg.Rules, controller.lastGroups, controller.lastAppSamples, controller.lastStatuses), logicalCPUCount)
	managed = ui.WithManagedAverageCPU(managed, averageRows)
	graph := controller.cpuGraphLocked(rows, logicalCPUCount, now)
	if status == "" {
		status = fmt.Sprintf("%d apps observed", len(controller.lastGroups))
	}
	return ui.MenuBarViewState{
		Enabled:         cfg.Preferences.ManagementEnabled,
		ShowMenuBarIcon: cfg.Preferences.ShowMenuBarIcon,
		Preferences:     cfg.Preferences,
		TotalCPU:        controller.lastTotalCPU,
		AlertLevel:      alert,
		StatusMessage:   status,
		LastUpdated:     now,
		CPUGraph:        graph,
		MenuBarApps:     menuBarRows,
		TrackedApps:     tracked,
		TopProcesses:    rows,
		AllProcesses:    allRows,
		ManagedApps:     managed,
	}
}

func (controller *Controller) systemAlertLevelLocked(now time.Time) core.AlertLevel {
	preferences := controller.cfg.Preferences
	if !preferences.ManagementEnabled {
		controller.systemHighCPUSince = time.Time{}
		return core.AlertLevelOff
	}
	if !preferences.HighCPUDetectionEnabled ||
		preferences.HighCPUThreshold <= 0 ||
		preferences.HighCPUDuration <= 0 ||
		controller.lastTotalCPU < preferences.HighCPUThreshold {
		controller.systemHighCPUSince = time.Time{}
		return core.AlertLevelNormal
	}
	if controller.systemHighCPUSince.IsZero() {
		controller.systemHighCPUSince = now
		return core.AlertLevelNormal
	}
	if now.Sub(controller.systemHighCPUSince) < preferences.HighCPUDuration {
		return core.AlertLevelNormal
	}
	return core.AlertLevelHigh
}

func (controller *Controller) topProcessRowsLocked(now time.Time) []ui.ProcessRow {
	return ui.BuildTopProcessRows(controller.lastGroups, controller.lastAppSamples, controller.lastStatuses, true, topProcessesLimit)
}

func (controller *Controller) averageProcessRowsLocked(now time.Time) []ui.ProcessRow {
	if controller.history == nil {
		return nil
	}

	samples := observe.SummarizeAppCPUWindow(controller.history.Window(topProcessesWindow), topProcessesWindow)
	if len(samples) == 0 {
		return nil
	}
	return ui.BuildTopProcessRows(controller.lastGroups, samples, controller.lastStatuses, true, 0)
}

func limitProcessRows(rows []ui.ProcessRow, limit int) []ui.ProcessRow {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func (controller *Controller) cpuGraphLocked(rows []ui.ProcessRow, logicalCPUCount int, now time.Time) ui.GraphSeries {
	window := core.NormalizeCPUGraphWindow(controller.cfg.Preferences.CPUGraphWindow)
	samples := controller.lastAppSamples
	if controller.history != nil {
		if history := controller.history.Window(window); len(history) > 0 {
			samples = history
		}
	}
	graph := ui.BuildProcessCPUGraph(samples, rows, cpuGraphProcessLimit)
	graph = ui.WithGraphWindow(graph, now.Add(-window), now)
	if core.NormalizeCPUDisplayMode(controller.cfg.Preferences.CPUDisplayMode) == core.CPUDisplayModeSystemNormalized {
		graph = ui.WithGraphSystemCPUPercent(graph, logicalCPUCount)
	}
	return graph
}
