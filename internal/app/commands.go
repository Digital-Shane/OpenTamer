package app

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/ui"
)

const (
	commandToggleManagement = "toggle-management"
	commandRefresh          = "refresh"
	commandResetDefaults    = "reset-defaults"
)

func (controller *Controller) HandleMenuCommand(command string) {
	controller.mu.Lock()
	refresh := controller.handleMenuCommandLocked(command)
	controller.mu.Unlock()

	if refresh {
		go controller.Refresh()
	}
}

func (controller *Controller) handleMenuCommandLocked(command string) bool {
	parts := strings.Split(command, "|")
	switch parts[0] {
	case commandToggleManagement:
		controller.cfg.Preferences.ManagementEnabled = !controller.cfg.Preferences.ManagementEnabled
		controller.saveConfigLocked()
	case commandRefresh:
	case commandResetDefaults:
		controller.cfg.Preferences = config.DefaultConfig().Preferences
		controller.saveConfigLocked()
	case "graph-window":
		if len(parts) >= 2 {
			durationNanos, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				log.Printf("OpenTamer CPU graph window parse warning: %v", err)
				break
			}
			controller.cfg.Preferences.CPUGraphWindow = core.NormalizeCPUGraphWindow(time.Duration(durationNanos))
			controller.saveConfigLocked()
		}
	case "toggle-menu-icon":
		controller.cfg.Preferences.ShowMenuBarIcon = !controller.cfg.Preferences.ShowMenuBarIcon
		controller.saveConfigLocked()
	case "pref-bool", "pref-duration", "pref-float", "pref-string", "pref-clear":
		controller.handlePreferenceCommandLocked(parts)
	case "rule":
		controller.handleRuleCommandLocked(parts)
	case "disable-rule":
		if len(parts) >= 2 {
			controller.removeRuleLocked(parts[1])
			controller.saveConfigLocked()
		}
	case "quit":
		controller.shutdownLocked(core.ControlReasonGlobalDisabled)
		return false
	default:
		log.Printf("OpenTamer unknown menu command: %q", command)
	}
	return true
}

func (controller *Controller) handlePreferenceCommandLocked(parts []string) {
	if len(parts) < 2 {
		return
	}

	command := ui.PreferencesCommand{Field: ui.PreferenceField(parts[1])}
	switch parts[0] {
	case "pref-bool":
		if len(parts) < 3 {
			return
		}
		value, err := strconv.ParseBool(parts[2])
		if err != nil {
			log.Printf("OpenTamer boolean preference parse warning: %v", err)
			return
		}
		command.Kind = ui.CommandSetBoolPreference
		command.BoolValue = value
	case "pref-duration":
		if len(parts) < 3 {
			return
		}
		nanos, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			log.Printf("OpenTamer duration preference parse warning: %v", err)
			return
		}
		command.Kind = ui.CommandSetDurationPreference
		command.DurationValue = time.Duration(nanos)
	case "pref-float":
		if len(parts) < 3 {
			return
		}
		value, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			log.Printf("OpenTamer numeric preference parse warning: %v", err)
			return
		}
		command.Kind = ui.CommandSetFloatPreference
		command.FloatValue = value
	case "pref-string":
		if len(parts) < 3 {
			return
		}
		command.Kind = ui.CommandSetStringPreference
		command.StringValue = parts[2]
	case "pref-clear":
		command.Kind = ui.CommandClearPreference
	default:
		return
	}

	cfg, err := ui.ApplyPreferencesCommand(controller.cfg, command)
	if err != nil {
		log.Printf("OpenTamer preference update warning: %v", err)
		return
	}
	controller.cfg = cfg
	controller.saveConfigLocked()
}

func (controller *Controller) handleRuleCommandLocked(parts []string) {
	if len(parts) < 3 {
		return
	}
	mode := parts[1]
	appKey := parts[2]
	targetCPU := 0.0
	if mode == "limit" {
		if len(parts) < 4 {
			return
		}
		parsed, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			log.Printf("OpenTamer CPU limit parse warning: %v", err)
			return
		}
		targetCPU = parsed
		appKey = parts[3]
	}
	switch mode {
	case "untrack-menu-bar", "untrack-tray":
		controller.untrackLocationLocked(appKey, core.RuleTrackInMenuBar)
		return
	case "untrack-managed":
		controller.untrackLocationLocked(appKey, core.RuleTrackInManagedApps)
		return
	}
	group, ok := controller.groupByKeyLocked(appKey)
	if !ok {
		return
	}
	safety := core.SafetyDecisionFromGroup(group)
	existing, hasExisting := controller.ruleForAppLocked(group.ID)
	if mode == "pause" && hasExisting && existing.Mode == core.RuleModePauseInBackground {
		controller.clearPauseRuleLocked(existing, appKey)
		return
	}
	if strings.HasPrefix(mode, "track-") {
		rule := existing
		if !hasExisting {
			rule = core.AppRule{
				AppID:          group.ID,
				Mode:           core.RuleModeObserveOnly,
				BackgroundOnly: true,
				ProtectAudio:   true,
			}
		} else if !safety.AllowsRule(rule.Mode) {
			rule.Mode = core.RuleModeObserveOnly
			rule.BackgroundOnly = true
			rule.NiceValue = nil
			rule.CPUPercent = nil
			rule.HideWhenStopped = false
			rule.AllowBrowserPause = false
			rule.PreferEfficiencyCores = false
		}
		switch mode {
		case "track-menu-bar", "track-tray":
			if !hasExisting {
				rule.TrackIn = []core.RuleTrackingLocation{core.RuleTrackInMenuBar}
			} else {
				rule = rule.WithTrackIn(core.RuleTrackInMenuBar)
			}
		case "track-managed":
			if !hasExisting {
				rule.TrackIn = []core.RuleTrackingLocation{core.RuleTrackInManagedApps}
			} else {
				rule = rule.WithTrackIn(core.RuleTrackInManagedApps)
			}
		default:
			return
		}
		controller.upsertRuleLocked(rule, mode)
		return
	}
	trackIn := []core.RuleTrackingLocation{core.RuleTrackInManagedApps}
	if hasExisting {
		trackIn = existing.EffectiveTrackIn()
	} else if mode == "observe" {
		trackIn = []core.RuleTrackingLocation{core.RuleTrackInMenuBar, core.RuleTrackInManagedApps}
	}
	rule := core.AppRule{
		AppID:          group.ID,
		TrackIn:        trackIn,
		BackgroundOnly: true,
		ProtectAudio:   true,
	}
	switch mode {
	case "observe":
		rule.Mode = core.RuleModeObserveOnly
	case "priority", "priority-background":
		nice := core.DefaultBackgroundNice
		rule.Mode = core.RuleModeLowerPriorityInBackground
		rule.NiceValue = &nice
	case "priority-always":
		nice := core.DefaultBackgroundNice
		rule.Mode = core.RuleModeLowerPriorityInBackground
		rule.BackgroundOnly = false
		rule.NiceValue = &nice
	case "limit":
		rule.Mode = core.RuleModeLimitCPUInBackground
		rule.BackgroundOnly = false
		rule.CPUPercent = &targetCPU
	case "pause":
		rule.Mode = core.RuleModePauseInBackground
		rule.AllowBrowserPause = true
		rule.HideWhenStopped = true
	default:
		return
	}
	if !safety.AllowsRule(rule.Mode) {
		log.Printf("OpenTamer rule update blocked by safety: app=%s mode=%s", group.DisplayName(), rule.Mode)
		return
	}
	controller.upsertRuleLocked(rule, mode)
}

func (controller *Controller) upsertRuleLocked(rule core.AppRule, mode string) {
	rule.TrackIn = core.NormalizeTrackIn(rule.TrackIn)
	if len(rule.TrackIn) == 0 {
		rule.TrackIn = rule.EffectiveTrackIn()
	}
	cfg, err := ui.ApplyPreferencesCommand(controller.cfg, ui.PreferencesCommand{Kind: ui.CommandUpsertRule, Rule: rule})
	if err != nil {
		log.Printf("OpenTamer rule update warning: %v", err)
		return
	}
	if mode == "limit" {
		cfg.Preferences.CPULimiterEnabled = true
	}
	controller.cfg = cfg
	controller.saveConfigLocked()
}

func (controller *Controller) clearPauseRuleLocked(rule core.AppRule, appKey string) {
	if !rule.TracksIn(core.RuleTrackInMenuBar) {
		controller.removeRuleLocked(appKey)
		controller.saveConfigLocked()
		return
	}
	rule = core.AppRule{
		AppID:          rule.AppID,
		Mode:           core.RuleModeObserveOnly,
		TrackIn:        rule.EffectiveTrackIn(),
		BackgroundOnly: true,
		ProtectAudio:   rule.ProtectAudio,
	}
	controller.upsertRuleLocked(rule, "")
}

func (controller *Controller) ruleForAppLocked(app core.AppID) (core.AppRule, bool) {
	for _, rule := range controller.cfg.Rules {
		if rule.Matches(app) {
			return rule, true
		}
	}
	return core.AppRule{}, false
}

func (controller *Controller) untrackLocationLocked(appKey string, location core.RuleTrackingLocation) {
	for _, rule := range controller.cfg.Rules {
		if rule.AppID.Key() != appKey {
			continue
		}

		rule = rule.WithoutTrackIn(location)
		if len(rule.EffectiveTrackIn()) == 0 {
			if rule.Mode == core.RuleModeObserveOnly {
				controller.removeRuleLocked(appKey)
				controller.saveConfigLocked()
				return
			}
		}
		controller.upsertRuleLocked(rule, "")
		return
	}
}

func (controller *Controller) removeRuleLocked(appKey string) {
	for _, rule := range controller.cfg.Rules {
		if rule.AppID.Key() == appKey {
			cfg, err := ui.ApplyPreferencesCommand(controller.cfg, ui.PreferencesCommand{Kind: ui.CommandResetRule, AppID: rule.AppID})
			if err != nil {
				log.Printf("OpenTamer rule reset warning: %v", err)
				return
			}
			controller.cfg = cfg
			return
		}
	}
}

func (controller *Controller) groupByKeyLocked(appKey string) (core.AppGroup, bool) {
	for _, group := range controller.lastGroups {
		if group.ID.Key() == appKey {
			return group, true
		}
	}
	return core.AppGroup{}, false
}
