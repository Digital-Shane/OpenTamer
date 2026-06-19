package policy

import (
	"strings"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type ProtectionState struct {
	AudioActive      bool            `json:"audioActive"`
	BrowserLikeByApp map[string]bool `json:"browserLikeByApp"`
}

type AudioActivityObserver interface {
	AudioActive() (bool, error)
}

func (state ProtectionState) Blocks(group core.AppGroup, rule core.AppRule) (core.ControlReason, string, bool) {
	if !ruleAffectsBackgroundWork(rule.Mode) {
		return "", "", false
	}
	if state.IsBrowserLike(group.ID) && rule.Mode == core.RuleModePauseInBackground && !rule.AllowBrowserPause {
		return core.ControlReasonBrowserProtection, "browser-like apps default to slow-only protection", true
	}
	if rule.ProtectAudio && state.AudioActive {
		return core.ControlReasonAudioProtection, "sound is playing", true
	}
	return "", "", false
}

func (state ProtectionState) IsBrowserLike(app core.AppID) bool {
	if state.BrowserLikeByApp != nil && state.BrowserLikeByApp[app.Key()] {
		return true
	}
	return IsKnownBrowser(app)
}

func ruleAffectsBackgroundWork(mode core.RuleMode) bool {
	switch mode {
	case core.RuleModePauseInBackground, core.RuleModeLowerPriorityInBackground, core.RuleModeLimitCPUInBackground:
		return true
	default:
		return false
	}
}

func IsKnownBrowser(app core.AppID) bool {
	bundleID := strings.ToLower(app.BundleID)
	name := strings.ToLower(app.Name)
	switch bundleID {
	case "com.apple.safari",
		"com.google.chrome",
		"com.google.chrome.canary",
		"org.mozilla.firefox",
		"com.microsoft.edgemac",
		"com.brave.browser",
		"com.operasoftware.opera",
		"company.thebrowser.browser":
		return true
	}
	switch name {
	case "safari", "google chrome", "chrome", "firefox", "microsoft edge", "brave browser", "opera", "arc":
		return true
	default:
		return false
	}
}
