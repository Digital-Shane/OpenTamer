package ui

import (
	"fmt"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

type CommandKind string
type PreferenceField string

const (
	CommandSetManagementEnabled  CommandKind = "set_management_enabled"
	CommandSetStatsInterval      CommandKind = "set_stats_interval"
	CommandSetCPUGraphWindow     CommandKind = "set_cpu_graph_window"
	CommandSetBoolPreference     CommandKind = "set_bool_preference"
	CommandSetDurationPreference CommandKind = "set_duration_preference"
	CommandSetFloatPreference    CommandKind = "set_float_preference"
	CommandSetStringPreference   CommandKind = "set_string_preference"
	CommandClearPreference       CommandKind = "clear_preference"
	CommandUpsertRule            CommandKind = "upsert_rule"
	CommandResetGlobalDefaults   CommandKind = "reset_global_defaults"
	CommandResetRule             CommandKind = "reset_rule"
)

const (
	PreferenceManagementEnabled             PreferenceField = "managementEnabled"
	PreferenceCPULimiterEnabled             PreferenceField = "cpuLimiterEnabled"
	PreferenceAggregateByName               PreferenceField = "aggregateByName"
	PreferenceShowMenuBarIcon               PreferenceField = "showMenuBarIcon"
	PreferenceTopProcessesSort              PreferenceField = "topProcessesSort"
	PreferenceCPUDisplayMode                PreferenceField = "cpuDisplayMode"
	PreferenceStatsInterval                 PreferenceField = "statsInterval"
	PreferenceAveragingWindow               PreferenceField = "averagingWindow"
	PreferenceCPUGraphWindow                PreferenceField = "cpuGraphWindow"
	PreferenceWakeGrace                     PreferenceField = "wakeGrace"
	PreferenceDisableWhenACBatteryAbove     PreferenceField = "disableWhenACBatteryAbove"
	PreferenceDisableWhenUserIdleLongerThan PreferenceField = "disableWhenUserIdleLongerThan"
	PreferenceHighCPUDetectionEnabled       PreferenceField = "highCPUDetectionEnabled"
	PreferenceHighCPUThreshold              PreferenceField = "highCPUThreshold"
	PreferenceHighCPUDuration               PreferenceField = "highCPUDuration"
	PreferenceHighCPUCooldown               PreferenceField = "highCPUCooldown"
)

type PreferencesCommand struct {
	Kind          CommandKind     `json:"kind"`
	Field         PreferenceField `json:"field,omitempty"`
	BoolValue     bool            `json:"boolValue,omitempty"`
	DurationValue time.Duration   `json:"durationValue,omitempty"`
	FloatValue    float64         `json:"floatValue,omitempty"`
	StringValue   string          `json:"stringValue,omitempty"`
	Rule          core.AppRule    `json:"rule"`
	AppID         core.AppID      `json:"appID"`
}

func ApplyPreferencesCommand(cfg config.Config, command PreferencesCommand) (config.Config, error) {
	switch command.Kind {
	case CommandSetManagementEnabled:
		cfg.Preferences.ManagementEnabled = command.BoolValue
	case CommandSetStatsInterval:
		if command.DurationValue <= 0 {
			return cfg, fmt.Errorf("stats interval must be positive")
		}
		cfg.Preferences.StatsInterval = command.DurationValue
	case CommandSetCPUGraphWindow:
		cfg.Preferences.CPUGraphWindow = core.NormalizeCPUGraphWindow(command.DurationValue)
	case CommandSetBoolPreference:
		if err := setBoolPreference(&cfg.Preferences, command.Field, command.BoolValue); err != nil {
			return cfg, err
		}
	case CommandSetDurationPreference:
		if err := setDurationPreference(&cfg.Preferences, command.Field, command.DurationValue); err != nil {
			return cfg, err
		}
	case CommandSetFloatPreference:
		if err := setFloatPreference(&cfg.Preferences, command.Field, command.FloatValue); err != nil {
			return cfg, err
		}
	case CommandSetStringPreference:
		if err := setStringPreference(&cfg.Preferences, command.Field, command.StringValue); err != nil {
			return cfg, err
		}
	case CommandClearPreference:
		if err := clearPreference(&cfg.Preferences, command.Field); err != nil {
			return cfg, err
		}
	case CommandUpsertRule:
		var err error
		cfg.Rules, err = upsertRule(cfg.Rules, command.Rule)
		if err != nil {
			return cfg, err
		}
	case CommandResetGlobalDefaults:
		defaults := config.DefaultConfig()
		cfg.Preferences = defaults.Preferences
	case CommandResetRule:
		cfg.Rules = removeRule(cfg.Rules, command.AppID)
	default:
		return cfg, fmt.Errorf("unsupported preferences command %q", command.Kind)
	}
	return cfg, nil
}

func setBoolPreference(preferences *core.GlobalPreferences, field PreferenceField, value bool) error {
	switch field {
	case PreferenceManagementEnabled:
		preferences.ManagementEnabled = value
	case PreferenceCPULimiterEnabled:
		preferences.CPULimiterEnabled = value
	case PreferenceAggregateByName:
		preferences.AggregateByName = value
	case PreferenceShowMenuBarIcon:
		preferences.ShowMenuBarIcon = value
	case PreferenceHighCPUDetectionEnabled:
		preferences.HighCPUDetectionEnabled = value
	default:
		return fmt.Errorf("unsupported boolean preference %q", field)
	}
	return nil
}

func setDurationPreference(preferences *core.GlobalPreferences, field PreferenceField, value time.Duration) error {
	if value < 0 {
		return fmt.Errorf("%s must not be negative", field)
	}
	switch field {
	case PreferenceStatsInterval:
		if value <= 0 {
			return fmt.Errorf("stats interval must be positive")
		}
		preferences.StatsInterval = value
	case PreferenceAveragingWindow:
		if value <= 0 {
			return fmt.Errorf("averaging window must be positive")
		}
		preferences.AveragingWindow = value
	case PreferenceCPUGraphWindow:
		preferences.CPUGraphWindow = core.NormalizeCPUGraphWindow(value)
	case PreferenceWakeGrace:
		preferences.WakeGrace = value
	case PreferenceDisableWhenUserIdleLongerThan:
		preferences.DisableWhenUserIdleLongerThan = value
	case PreferenceHighCPUDuration:
		if value <= 0 {
			return fmt.Errorf("high CPU duration must be positive")
		}
		preferences.HighCPUDuration = value
	case PreferenceHighCPUCooldown:
		if value <= 0 {
			return fmt.Errorf("high CPU cooldown must be positive")
		}
		preferences.HighCPUCooldown = value
	default:
		return fmt.Errorf("unsupported duration preference %q", field)
	}
	return nil
}

func setFloatPreference(preferences *core.GlobalPreferences, field PreferenceField, value float64) error {
	switch field {
	case PreferenceHighCPUThreshold:
		if value <= 0 {
			return fmt.Errorf("high CPU threshold must be positive")
		}
		preferences.HighCPUThreshold = value
	case PreferenceDisableWhenACBatteryAbove:
		if value <= 0 || value > 100 {
			return fmt.Errorf("AC battery threshold must be between 0 and 100")
		}
		threshold := value
		preferences.DisableWhenACBatteryAbove = &threshold
	default:
		return fmt.Errorf("unsupported numeric preference %q", field)
	}
	return nil
}

func setStringPreference(preferences *core.GlobalPreferences, field PreferenceField, value string) error {
	switch field {
	case PreferenceTopProcessesSort:
		normalized := core.NormalizeTopProcessesSortMode(value)
		if normalized != value {
			return fmt.Errorf("unsupported top processes sort mode %q", value)
		}
		preferences.TopProcessesSort = normalized
	case PreferenceCPUDisplayMode:
		normalized := core.NormalizeCPUDisplayMode(value)
		if normalized != value {
			return fmt.Errorf("unsupported CPU display mode %q", value)
		}
		preferences.CPUDisplayMode = normalized
	default:
		return fmt.Errorf("unsupported string preference %q", field)
	}
	return nil
}

func clearPreference(preferences *core.GlobalPreferences, field PreferenceField) error {
	switch field {
	case PreferenceDisableWhenACBatteryAbove:
		preferences.DisableWhenACBatteryAbove = nil
	default:
		return fmt.Errorf("unsupported clear preference %q", field)
	}
	return nil
}

func upsertRule(rules []core.AppRule, rule core.AppRule) ([]core.AppRule, error) {
	if rule.AppID.IsEmpty() {
		return rules, fmt.Errorf("rule app id is required")
	}
	if rule.Mode == core.RuleModeLimitCPUInBackground && (rule.CPUPercent == nil || *rule.CPUPercent < apppolicy.MinCPULimitPercent) {
		return rules, fmt.Errorf("cpu limit target must be at least %s", apppolicy.FormatCPULimitPercent(apppolicy.MinCPULimitPercent))
	}
	for i, existing := range rules {
		if existing.AppID.Matches(rule.AppID) {
			next := append([]core.AppRule(nil), rules...)
			next[i] = rule
			return next, nil
		}
	}
	return append(append([]core.AppRule(nil), rules...), rule), nil
}

func removeRule(rules []core.AppRule, app core.AppID) []core.AppRule {
	next := make([]core.AppRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.AppID.Matches(app) {
			next = append(next, rule)
		}
	}
	return next
}
