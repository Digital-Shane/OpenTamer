package core

import "strings"

type SafetyReason string

const (
	SafetyReasonNone             SafetyReason = ""
	SafetyReasonEssentialSystem  SafetyReason = "essential system process"
	SafetyReasonAccessibility    SafetyReason = "accessibility process"
	SafetyReasonOpenTamerSelf    SafetyReason = "OpenTamer process"
	SafetyReasonPrivilegedHelper SafetyReason = "OpenTamer privileged helper"
	SafetyReasonProtectedProcess SafetyReason = "protected process"
	SafetyReasonRootOwned        SafetyReason = "owned by root"
	SafetyReasonUnknown          SafetyReason = "unknown process"
	SafetyReasonSlowOnly         SafetyReason = "slow only"
)

type SafetyDecision struct {
	Kind            AppKind         `json:"kind"`
	Controllability Controllability `json:"controllability"`
	Reason          SafetyReason    `json:"reason,omitempty"`
	CanPause        bool            `json:"canPause"`
	CanSlow         bool            `json:"canSlow"`
	CanHide         bool            `json:"canHide"`
	CanQuit         bool            `json:"canQuit"`
}

func (decision SafetyDecision) AllowsRule(mode RuleMode) bool {
	switch mode {
	case RuleModeObserveOnly:
		return true
	case RuleModePauseInBackground:
		return decision.CanPause
	case RuleModeLowerPriorityInBackground, RuleModeLimitCPUInBackground:
		return decision.CanSlow
	case RuleModeHideAfterIdle:
		return decision.CanHide
	case RuleModeQuitAfterIdle:
		return decision.CanQuit
	default:
		return false
	}
}

type SafetyPolicy struct {
	SelfPID            int
	HelperNames        map[string]struct{}
	EssentialNames     map[string]struct{}
	AccessibilityNames map[string]struct{}
}

func NewSafetyPolicy(selfPID int) SafetyPolicy {
	return SafetyPolicy{
		SelfPID: selfPID,
		HelperNames: map[string]struct{}{
			"opentamer-helper": {},
		},
		EssentialNames: map[string]struct{}{
			"kernel_task":  {},
			"launchd":      {},
			"loginwindow":  {},
			"WindowServer": {},
			"hidd":         {},
			"securityd":    {},
			"tccd":         {},
		},
		AccessibilityNames: map[string]struct{}{
			"VoiceOver":            {},
			"AXVisualSupportAgent": {},
		},
	}
}

func (policy SafetyPolicy) ClassifyProcess(process ProcessRef) SafetyDecision {
	name := strings.TrimSpace(process.Name)
	if policy.SelfPID > 0 && process.ID.PID == policy.SelfPID {
		return blockedDecision(AppKindEssential, SafetyReasonOpenTamerSelf)
	}
	if _, ok := policy.HelperNames[name]; ok {
		return blockedDecision(AppKindEssential, SafetyReasonPrivilegedHelper)
	}
	if _, ok := policy.EssentialNames[name]; ok {
		return blockedDecision(AppKindEssential, SafetyReasonEssentialSystem)
	}
	if _, ok := policy.AccessibilityNames[name]; ok {
		return blockedDecision(AppKindAccessibility, SafetyReasonAccessibility)
	}
	if process.UID == 0 && process.ExecutablePath == "" {
		return blockedDecision(AppKindProtected, SafetyReasonProtectedProcess)
	}
	if process.UID == 0 {
		return slowOnlyDecision(AppKindSystemService, SafetyReasonRootOwned)
	}
	if process.ExecutablePath == "" {
		return slowOnlyDecision(AppKindUnknown, SafetyReasonUnknown)
	}
	return normalDecision(AppKindUserApp)
}

func (policy SafetyPolicy) ClassifyGroup(group AppGroup) SafetyDecision {
	if len(group.Processes) == 0 {
		return slowOnlyDecision(AppKindUnknown, SafetyReasonUnknown)
	}

	final := normalDecision(group.Kind)
	for _, process := range group.Processes {
		decision := policy.ClassifyProcess(process)
		if decision.Controllability == ControllabilityBlocked {
			return decision
		}
		if decision.Controllability == ControllabilitySlowOnly {
			final = decision
		}
	}
	return final
}

func SafetyDecisionFromGroup(group AppGroup) SafetyDecision {
	switch group.Controllability {
	case ControllabilityBlocked:
		reason := group.SafetyReason
		if reason == "" {
			reason = SafetyReasonProtectedProcess
		}
		return blockedDecision(group.Kind, reason)
	case ControllabilitySlowOnly:
		reason := group.SafetyReason
		if reason == "" {
			reason = SafetyReasonSlowOnly
		}
		return slowOnlyDecision(group.Kind, reason)
	case ControllabilityNormal:
		return normalDecision(group.Kind)
	default:
		return slowOnlyDecision(group.Kind, SafetyReasonUnknown)
	}
}

func ApplySafetyDecision(group AppGroup, decision SafetyDecision) AppGroup {
	group.Kind = decision.Kind
	group.Controllability = decision.Controllability
	group.SafetyReason = decision.Reason
	return group
}

func normalDecision(kind AppKind) SafetyDecision {
	if kind == "" {
		kind = AppKindUserApp
	}
	return SafetyDecision{
		Kind:            kind,
		Controllability: ControllabilityNormal,
		CanPause:        true,
		CanSlow:         true,
		CanHide:         true,
		CanQuit:         true,
	}
}

func slowOnlyDecision(kind AppKind, reason SafetyReason) SafetyDecision {
	if kind == "" {
		kind = AppKindUnknown
	}
	return SafetyDecision{
		Kind:            kind,
		Controllability: ControllabilitySlowOnly,
		Reason:          reason,
		CanSlow:         true,
	}
}

func blockedDecision(kind AppKind, reason SafetyReason) SafetyDecision {
	if kind == "" {
		kind = AppKindProtected
	}
	return SafetyDecision{
		Kind:            kind,
		Controllability: ControllabilityBlocked,
		Reason:          reason,
	}
}
