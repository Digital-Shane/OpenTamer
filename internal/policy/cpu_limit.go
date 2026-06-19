package policy

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type CPULimitReason string

const (
	MinCPULimitPercent = 0.01

	CPULimitReasonNone       CPULimitReason = ""
	CPULimitReasonDisabled   CPULimitReason = "cpu limiter disabled"
	CPULimitReasonForeground CPULimitReason = "foreground app"
	CPULimitReasonProtected  CPULimitReason = "protected app"
	CPULimitReasonNoTarget   CPULimitReason = "no target CPU"
	CPULimitReasonUnderLimit CPULimitReason = "under target CPU"
	CPULimitReasonLimit      CPULimitReason = "limit CPU"
)

type CPULimitPlanner struct {
	Enabled        bool
	Cycle          time.Duration
	MaxStop        time.Duration
	BrowserMaxStop time.Duration
	MinRun         time.Duration
}

func DefaultCPULimitPlanner(enabled bool) CPULimitPlanner {
	return CPULimitPlanner{
		Enabled:        enabled,
		Cycle:          10 * time.Second,
		MaxStop:        9999 * time.Millisecond,
		BrowserMaxStop: 9999 * time.Millisecond,
		MinRun:         time.Millisecond,
	}
}

func FormatCPULimitPercent(value float64) string {
	text := strconv.FormatFloat(value, 'f', 3, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" || text == "-0" {
		text = "0"
	}
	return text + "%"
}

type CPULimitInput struct {
	Group       core.AppGroup
	ObservedCPU float64
	TargetCPU   float64
	Foreground  bool
	Protected   bool
	BrowserLike bool
}

type CPULimitPlan struct {
	Active      bool           `json:"active"`
	Reason      CPULimitReason `json:"reason"`
	ObservedCPU float64        `json:"observedCPU"`
	TargetCPU   float64        `json:"targetCPU"`
	RunFor      time.Duration  `json:"runFor"`
	StopFor     time.Duration  `json:"stopFor"`
	Explanation string         `json:"explanation"`
}

func (planner CPULimitPlanner) Plan(input CPULimitInput) CPULimitPlan {
	cycle := planner.Cycle
	if cycle <= 0 {
		cycle = time.Second
	}
	minRun := planner.MinRun
	if minRun <= 0 {
		minRun = 50 * time.Millisecond
	}
	maxStop := planner.MaxStop
	if maxStop <= 0 {
		maxStop = cycle - minRun
	}
	if input.BrowserLike && planner.BrowserMaxStop > 0 && planner.BrowserMaxStop < maxStop {
		maxStop = planner.BrowserMaxStop
	}

	base := CPULimitPlan{
		ObservedCPU: input.ObservedCPU,
		TargetCPU:   input.TargetCPU,
	}

	if !planner.Enabled {
		base.Reason = CPULimitReasonDisabled
		base.Explanation = "CPU limiting is disabled globally."
		return base
	}
	if input.Foreground {
		base.Reason = CPULimitReasonForeground
		base.Explanation = "CPU limiting stops when the app is foreground."
		return base
	}
	if input.Protected {
		base.Reason = CPULimitReasonProtected
		base.Explanation = "CPU limiting is blocked by a protection policy."
		return base
	}
	if input.TargetCPU < MinCPULimitPercent {
		base.Reason = CPULimitReasonNoTarget
		base.Explanation = fmt.Sprintf("No CPU target of at least %s was configured.", FormatCPULimitPercent(MinCPULimitPercent))
		return base
	}
	if input.ObservedCPU <= input.TargetCPU {
		base.Reason = CPULimitReasonUnderLimit
		base.Explanation = fmt.Sprintf("Observed CPU %.1f%% is at or below target %s.", input.ObservedCPU, FormatCPULimitPercent(input.TargetCPU))
		return base
	}

	stopRatio := 1 - (input.TargetCPU / input.ObservedCPU)
	stopFor := min(time.Duration(math.Round(float64(cycle)*stopRatio)), maxStop)
	runFor := cycle - stopFor
	if runFor < minRun {
		runFor = minRun
		stopFor = max(cycle-runFor, 0)
	}

	return CPULimitPlan{
		Active:      stopFor > 0,
		Reason:      CPULimitReasonLimit,
		ObservedCPU: input.ObservedCPU,
		TargetCPU:   input.TargetCPU,
		RunFor:      runFor,
		StopFor:     stopFor,
		Explanation: fmt.Sprintf("Observed CPU %.1f%% is above target %s; run for %s and pause for %s per cycle.", input.ObservedCPU, FormatCPULimitPercent(input.TargetCPU), runFor, stopFor),
	}
}
