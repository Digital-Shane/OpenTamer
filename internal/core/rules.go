package core

import (
	"slices"
	"time"
)

type RuleMode string
type RuleTrackingLocation string

const (
	RuleModeObserveOnly               RuleMode = "observe_only"
	RuleModePauseInBackground         RuleMode = "pause_in_background"
	RuleModeLowerPriorityInBackground RuleMode = "lower_priority_in_background"
	RuleModeLimitCPUInBackground      RuleMode = "limit_cpu_in_background"
	RuleModeHideAfterIdle             RuleMode = "hide_after_idle"
	RuleModeQuitAfterIdle             RuleMode = "quit_after_idle"
)

const (
	RuleTrackInNone        RuleTrackingLocation = "none"
	RuleTrackInMenuBar     RuleTrackingLocation = "menu_bar"
	RuleTrackInManagedApps RuleTrackingLocation = "managed_apps"

	legacyRuleTrackInTray RuleTrackingLocation = "tray"
)

const (
	DefaultCPUGraphWindow = 5 * time.Minute
	MinCPUGraphWindow     = time.Minute
	MaxCPUGraphWindow     = 30 * time.Minute
)

func (mode RuleMode) SupportedInMVP() bool {
	switch mode {
	case RuleModeObserveOnly, RuleModePauseInBackground, RuleModeLowerPriorityInBackground, RuleModeLimitCPUInBackground:
		return true
	default:
		return false
	}
}

type AppRule struct {
	AppID                 AppID                  `json:"appID"`
	Mode                  RuleMode               `json:"mode"`
	TrackIn               []RuleTrackingLocation `json:"trackIn,omitempty"`
	BackgroundOnly        bool                   `json:"backgroundOnly"`
	WaitBeforeApply       time.Duration          `json:"waitBeforeApply"`
	LaunchGrace           time.Duration          `json:"launchGrace"`
	PeriodicWakeEvery     time.Duration          `json:"periodicWakeEvery"`
	NiceValue             *int                   `json:"niceValue,omitempty"`
	CPUPercent            *float64               `json:"cpuPercent,omitempty"`
	HideWhenStopped       bool                   `json:"hideWhenStopped"`
	ProtectAudio          bool                   `json:"protectAudio"`
	AllowBrowserPause     bool                   `json:"allowBrowserPause"`
	PreferEfficiencyCores bool                   `json:"preferEfficiencyCores"`
}

func (rule AppRule) Matches(app AppID) bool {
	return rule.AppID.Matches(app)
}

func (rule AppRule) TracksIn(location RuleTrackingLocation) bool {
	return slices.Contains(rule.EffectiveTrackIn(), location)
}

func (rule AppRule) EffectiveTrackIn() []RuleTrackingLocation {
	if len(rule.TrackIn) > 0 {
		normalized := NormalizeTrackIn(rule.TrackIn)
		if len(normalized) == 1 && normalized[0] == RuleTrackInNone {
			return nil
		}
		if len(normalized) > 0 {
			return normalized
		}
	}
	if rule.Mode == RuleModeObserveOnly {
		return []RuleTrackingLocation{RuleTrackInMenuBar, RuleTrackInManagedApps}
	}
	return []RuleTrackingLocation{RuleTrackInManagedApps}
}

func (rule AppRule) WithTrackIn(location RuleTrackingLocation) AppRule {
	rule.TrackIn = NormalizeTrackIn(append(rule.EffectiveTrackIn(), location))
	return rule
}

func (rule AppRule) WithoutTrackIn(location RuleTrackingLocation) AppRule {
	locations := make([]RuleTrackingLocation, 0, len(rule.EffectiveTrackIn()))
	for _, current := range rule.EffectiveTrackIn() {
		if current != location {
			locations = append(locations, current)
		}
	}
	if len(locations) == 0 {
		rule.TrackIn = []RuleTrackingLocation{RuleTrackInNone}
		return rule
	}
	rule.TrackIn = NormalizeTrackIn(locations)
	return rule
}

func NormalizeTrackIn(locations []RuleTrackingLocation) []RuleTrackingLocation {
	hasNone := false
	seen := make(map[RuleTrackingLocation]bool, len(locations))
	for _, location := range locations {
		switch location {
		case RuleTrackInNone:
			hasNone = true
		case RuleTrackInMenuBar, RuleTrackInManagedApps:
			seen[location] = true
		case legacyRuleTrackInTray:
			seen[RuleTrackInMenuBar] = true
		}
	}
	if len(seen) == 0 && hasNone {
		return []RuleTrackingLocation{RuleTrackInNone}
	}
	ordered := make([]RuleTrackingLocation, 0, len(seen))
	for _, location := range []RuleTrackingLocation{RuleTrackInMenuBar, RuleTrackInManagedApps} {
		if seen[location] {
			ordered = append(ordered, location)
		}
	}
	return ordered
}

type GlobalPreferences struct {
	ManagementEnabled             bool          `json:"managementEnabled"`
	CPULimiterEnabled             bool          `json:"cpuLimiterEnabled"`
	AggregateByName               bool          `json:"aggregateByName"`
	ShowMenuBarIcon               bool          `json:"showMenuBarIcon"`
	TopProcessesSort              string        `json:"topProcessesSort"`
	CPUDisplayMode                string        `json:"cpuDisplayMode"`
	StatsInterval                 time.Duration `json:"statsInterval"`
	AveragingWindow               time.Duration `json:"averagingWindow"`
	CPUGraphWindow                time.Duration `json:"cpuGraphWindow"`
	WakeGrace                     time.Duration `json:"wakeGrace"`
	DisableWhenACBatteryAbove     *float64      `json:"disableWhenACBatteryAbove,omitempty"`
	DisableWhenUserIdleLongerThan time.Duration `json:"disableWhenUserIdleLongerThan,omitempty"`
	HighCPUDetectionEnabled       bool          `json:"highCPUDetectionEnabled"`
	HighCPUThreshold              float64       `json:"highCPUThreshold"`
	HighCPUDuration               time.Duration `json:"highCPUDuration"`
	HighCPUCooldown               time.Duration `json:"highCPUCooldown"`
	Theme                         string        `json:"theme"`
}

const (
	TopProcessesSortCurrent = "current"
	TopProcessesSortAverage = "average"
)

const (
	CPUDisplayModePerCoreProcess   = "per_core_process"
	CPUDisplayModeSystemNormalized = "system_normalized"
)

type AppStateStore interface {
	GetRules() ([]AppRule, error)
	GetRuntimeState() RuntimeState
	SetRuntimeState(RuntimeState) error
}

func NormalizeCPUGraphWindow(window time.Duration) time.Duration {
	if window <= 0 {
		return DefaultCPUGraphWindow
	}
	if window < MinCPUGraphWindow {
		return MinCPUGraphWindow
	}
	if window > MaxCPUGraphWindow {
		return MaxCPUGraphWindow
	}
	return window
}

func NormalizeTopProcessesSortMode(mode string) string {
	switch mode {
	case TopProcessesSortAverage:
		return TopProcessesSortAverage
	default:
		return TopProcessesSortCurrent
	}
}

func NormalizeCPUDisplayMode(mode string) string {
	switch mode {
	case CPUDisplayModeSystemNormalized:
		return CPUDisplayModeSystemNormalized
	default:
		return CPUDisplayModePerCoreProcess
	}
}

type Clock interface {
	Now() time.Time
}

func NormalizeThemeMode(mode string) string {
	switch mode {
	case "light", "dark":
		return mode
	default:
		return "system"
	}
}
