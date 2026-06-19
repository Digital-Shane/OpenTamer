package core

import "maps"

import "time"

type ControlActionType string

const (
	ControlActionNoop            ControlActionType = "noop"
	ControlActionPause           ControlActionType = "pause"
	ControlActionResume          ControlActionType = "resume"
	ControlActionSetPriority     ControlActionType = "set_priority"
	ControlActionRestorePriority ControlActionType = "restore_priority"
	ControlActionLimitCPU        ControlActionType = "limit_cpu"
	ControlActionWakeBriefly     ControlActionType = "wake_briefly"
	ControlActionHide            ControlActionType = "hide"
	ControlActionActivate        ControlActionType = "activate"
	ControlActionQuit            ControlActionType = "quit"
	ControlActionUnsupported     ControlActionType = "unsupported"
)

type ControlReason string

const (
	ControlReasonUnknown           ControlReason = "unknown"
	ControlReasonUserRule          ControlReason = "user_rule"
	ControlReasonForeground        ControlReason = "foreground"
	ControlReasonGlobalDisabled    ControlReason = "global_disabled"
	ControlReasonSafetyBlocked     ControlReason = "safety_blocked"
	ControlReasonPolicyBlocked     ControlReason = "policy_blocked"
	ControlReasonPowerPolicy       ControlReason = "power_policy"
	ControlReasonIdlePolicy        ControlReason = "idle_policy"
	ControlReasonAudioProtection   ControlReason = "audio_protection"
	ControlReasonBrowserProtection ControlReason = "browser_protection"
	ControlReasonLaunchGrace       ControlReason = "launch_grace"
	ControlReasonWakeGrace         ControlReason = "wake_grace"
	ControlReasonPeriodicWake      ControlReason = "periodic_wake"
	ControlReasonUnsupportedRule   ControlReason = "unsupported_rule"
)

type ControlAction struct {
	Type       ControlActionType `json:"type"`
	AppID      AppID             `json:"appID"`
	Processes  []ProcessID       `json:"processes"`
	Reason     ControlReason     `json:"reason"`
	NiceValue  *int              `json:"niceValue,omitempty"`
	CPUPercent *float64          `json:"cpuPercent,omitempty"`
	RunFor     time.Duration     `json:"runFor,omitempty"`
	StopFor    time.Duration     `json:"stopFor,omitempty"`
	Message    string            `json:"message,omitempty"`
	At         time.Time         `json:"at"`
}

type RuntimeState struct {
	PausedProcesses map[string]PausedProcessControl `json:"pausedProcesses"`
	PriorityChanges map[string]PriorityChange       `json:"priorityChanges"`
	HiddenApps      map[string]HiddenAppControl     `json:"hiddenApps"`
	AppStates       map[string]AppControlRuntime    `json:"appStates"`
	LastActions     []ControlAction                 `json:"lastActions"`
}

func NewRuntimeState() RuntimeState {
	return RuntimeState{
		PausedProcesses: make(map[string]PausedProcessControl),
		PriorityChanges: make(map[string]PriorityChange),
		HiddenApps:      make(map[string]HiddenAppControl),
		AppStates:       make(map[string]AppControlRuntime),
		LastActions:     make([]ControlAction, 0),
	}
}

func (state RuntimeState) Clone() RuntimeState {
	clone := RuntimeState{
		PausedProcesses: make(map[string]PausedProcessControl, len(state.PausedProcesses)),
		PriorityChanges: make(map[string]PriorityChange, len(state.PriorityChanges)),
		HiddenApps:      make(map[string]HiddenAppControl, len(state.HiddenApps)),
		AppStates:       make(map[string]AppControlRuntime, len(state.AppStates)),
		LastActions:     append([]ControlAction(nil), state.LastActions...),
	}
	maps.Copy(clone.PausedProcesses, state.PausedProcesses)
	maps.Copy(clone.PriorityChanges, state.PriorityChanges)
	maps.Copy(clone.HiddenApps, state.HiddenApps)
	maps.Copy(clone.AppStates, state.AppStates)
	return clone
}

func (state RuntimeState) IsPausedByOpenTamer(id ProcessID) bool {
	_, ok := state.PausedProcesses[id.Key()]
	return ok
}

func (state RuntimeState) OwnedPausedProcesses() []PausedProcessControl {
	owned := make([]PausedProcessControl, 0, len(state.PausedProcesses))
	for _, paused := range state.PausedProcesses {
		owned = append(owned, paused)
	}
	return owned
}

func (state RuntimeState) IsHiddenByOpenTamer(app AppID) bool {
	_, ok := state.HiddenApps[app.Key()]
	return ok
}

func (state *RuntimeState) RecordPause(process ProcessRef, app AppID, reason ControlReason, at time.Time) {
	state.ensureMaps()
	state.PausedProcesses[process.ID.Key()] = PausedProcessControl{
		Process:    process,
		AppID:      app,
		PausedAt:   at,
		Reason:     reason,
		Reversible: true,
	}
}

func (state *RuntimeState) ClearPause(id ProcessID) {
	state.ensureMaps()
	delete(state.PausedProcesses, id.Key())
}

func (state *RuntimeState) RecordPriorityChange(process ProcessRef, app AppID, original, target int, reason ControlReason, at time.Time) {
	state.ensureMaps()
	state.PriorityChanges[process.ID.Key()] = PriorityChange{
		Process:      process,
		AppID:        app,
		OriginalNice: original,
		TargetNice:   target,
		ChangedAt:    at,
		Reason:       reason,
		Reversible:   true,
	}
}

func (state *RuntimeState) ClearPriorityChange(id ProcessID) {
	state.ensureMaps()
	delete(state.PriorityChanges, id.Key())
}

func (state *RuntimeState) RecordHidden(process ProcessRef, app AppID, reason ControlReason, at time.Time) {
	state.ensureMaps()
	state.HiddenApps[app.Key()] = HiddenAppControl{Process: process, AppID: app, HiddenAt: at, Reason: reason}
}

func (state *RuntimeState) ClearHidden(app AppID) {
	state.ensureMaps()
	delete(state.HiddenApps, app.Key())
}

func (state *RuntimeState) AppendAction(action ControlAction, limit int) {
	state.ensureMaps()
	state.LastActions = append(state.LastActions, action)
	if limit > 0 && len(state.LastActions) > limit {
		state.LastActions = state.LastActions[len(state.LastActions)-limit:]
	}
}

func (state *RuntimeState) EnsureMaps() {
	state.ensureMaps()
}

func (state *RuntimeState) ensureMaps() {
	if state.PausedProcesses == nil {
		state.PausedProcesses = make(map[string]PausedProcessControl)
	}
	if state.PriorityChanges == nil {
		state.PriorityChanges = make(map[string]PriorityChange)
	}
	if state.HiddenApps == nil {
		state.HiddenApps = make(map[string]HiddenAppControl)
	}
	if state.AppStates == nil {
		state.AppStates = make(map[string]AppControlRuntime)
	}
	if state.LastActions == nil {
		state.LastActions = make([]ControlAction, 0)
	}
}

type PausedProcessControl struct {
	Process    ProcessRef    `json:"process"`
	AppID      AppID         `json:"appID"`
	PausedAt   time.Time     `json:"pausedAt"`
	Reason     ControlReason `json:"reason"`
	Reversible bool          `json:"reversible"`
}

type PriorityChange struct {
	Process      ProcessRef    `json:"process"`
	AppID        AppID         `json:"appID"`
	OriginalNice int           `json:"originalNice"`
	TargetNice   int           `json:"targetNice"`
	ChangedAt    time.Time     `json:"changedAt"`
	Reason       ControlReason `json:"reason"`
	Reversible   bool          `json:"reversible"`
}

type HiddenAppControl struct {
	Process  ProcessRef    `json:"process"`
	AppID    AppID         `json:"appID"`
	HiddenAt time.Time     `json:"hiddenAt"`
	Reason   ControlReason `json:"reason"`
}

type AppControlRuntime struct {
	AppID              AppID         `json:"appID"`
	FirstSeenAt        time.Time     `json:"firstSeenAt"`
	BackgroundSince    time.Time     `json:"backgroundSince"`
	LastPeriodicWakeAt time.Time     `json:"lastPeriodicWakeAt"`
	LastEvaluatedAt    time.Time     `json:"lastEvaluatedAt"`
	Status             AppStatus     `json:"status"`
	CPULimitTarget     float64       `json:"cpuLimitTarget,omitempty"`
	CPULimitRunFor     time.Duration `json:"cpuLimitRunFor,omitempty"`
	CPULimitStopFor    time.Duration `json:"cpuLimitStopFor,omitempty"`
}
