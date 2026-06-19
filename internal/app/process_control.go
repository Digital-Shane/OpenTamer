package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

var ErrProcessExited = errors.New("process exited")
var ErrProcessGenerationMismatch = errors.New("process generation mismatch")
var ErrPriorityImprovementRequiresPrivilege = errors.New("priority improvement requires privileges")
var ErrUnsupportedController = errors.New("controller backend is not configured")

type SignalController interface {
	Stop(core.ProcessRef) error
	Continue(core.ProcessRef) error
}

type PriorityController interface {
	GetPriority(core.ProcessRef) (int, error)
	SetPriority(core.ProcessRef, int) error
}

type LifecycleController interface {
	Hide(core.ProcessRef) error
	Activate(core.ProcessRef) error
	Terminate(core.ProcessRef) error
}

type ProcessGenerationLookup interface {
	ProcessGeneration(pid int) (core.ProcessID, error)
}

type ProcessGenerationValidator interface {
	ValidateProcessGeneration(core.ProcessRef) error
}

type ProcessGenerationLookupValidator struct {
	Lookup ProcessGenerationLookup
}

func (validator ProcessGenerationLookupValidator) ValidateProcessGeneration(process core.ProcessRef) error {
	if validator.Lookup == nil || process.ID.StartTime.IsZero() {
		return nil
	}
	if process.ID.PID <= 0 {
		return ErrProcessExited
	}
	current, err := validator.Lookup.ProcessGeneration(process.ID.PID)
	if err != nil {
		return err
	}
	if !process.ID.SameGeneration(current) {
		return fmt.Errorf("%w: pid %d expected start %s observed start %s",
			ErrProcessGenerationMismatch,
			process.ID.PID,
			process.ID.StartTime.Format(time.RFC3339Nano),
			current.StartTime.Format(time.RFC3339Nano),
		)
	}
	return nil
}

type ControlFailure struct {
	Process core.ProcessRef
	Action  core.ControlActionType
	Err     error
}

type ControlResult struct {
	Runtime  core.RuntimeState
	Failures []ControlFailure
}

func (failure ControlFailure) Error() string {
	if failure.Err == nil {
		return ""
	}
	return fmt.Sprintf("%s pid %d: %v", failure.Action, failure.Process.ID.PID, failure.Err)
}

type AppGroupController struct {
	Signals                  SignalController
	Priority                 PriorityController
	Lifecycle                LifecycleController
	ProcessValidator         ProcessGenerationValidator
	AllowPriorityImprovement bool
}

func (controller *AppGroupController) PauseGroup(group core.AppGroup, state core.RuntimeState, reason core.ControlReason, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}

	for _, process := range group.Processes {
		if state.IsPausedByOpenTamer(process.ID) {
			continue
		}
		if err := controller.validateProcessGeneration(process); err != nil {
			if shouldReportControlFailure(err) {
				result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionPause, Err: err})
			}
			continue
		}
		if err := controller.Signals.Stop(process); err != nil {
			if !errors.Is(err, ErrProcessExited) {
				result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionPause, Err: err})
			}
			continue
		}
		result.Runtime.RecordPause(process, group.ID, reason, at)
	}

	return result
}

func (controller *AppGroupController) ResumeGroup(group core.AppGroup, state core.RuntimeState, reason core.ControlReason, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}

	for _, process := range group.Processes {
		if !state.IsPausedByOpenTamer(process.ID) {
			continue
		}
		if err := controller.validateProcessGeneration(process); err != nil {
			if shouldClearStaleProcessState(err) {
				result.Runtime.ClearPause(process.ID)
			}
			if shouldReportControlFailure(err) {
				result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionResume, Err: err})
			}
			continue
		}
		if err := controller.Signals.Continue(process); err != nil && !errors.Is(err, ErrProcessExited) {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionResume, Err: err})
			continue
		}
		result.Runtime.ClearPause(process.ID)
	}

	return result
}

func (controller *AppGroupController) WakeGroupBriefly(group core.AppGroup, state core.RuntimeState, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}

	for _, process := range group.Processes {
		if !state.IsPausedByOpenTamer(process.ID) {
			continue
		}
		if err := controller.validateProcessGeneration(process); err != nil {
			if shouldReportControlFailure(err) {
				result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionWakeBriefly, Err: err})
			}
			continue
		}
		if err := controller.Signals.Continue(process); err != nil && !errors.Is(err, ErrProcessExited) {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionWakeBriefly, Err: err})
			continue
		}
	}

	return result
}

func (controller *AppGroupController) SetPriorityGroup(group core.AppGroup, state core.RuntimeState, targetNice int, reason core.ControlReason, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}

	for _, process := range group.Processes {
		if controller.Priority == nil {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionSetPriority, Err: ErrUnsupportedController})
			continue
		}
		if err := controller.validateProcessGeneration(process); err != nil {
			if shouldClearStaleProcessState(err) {
				result.Runtime.ClearPriorityChange(process.ID)
			}
			if shouldReportControlFailure(err) {
				result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionSetPriority, Err: err})
			}
			continue
		}
		originalNice, err := controller.Priority.GetPriority(process)
		if err != nil {
			if errors.Is(err, ErrProcessExited) {
				result.Runtime.ClearPriorityChange(process.ID)
				continue
			}
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionSetPriority, Err: err})
			continue
		}
		if targetNice < originalNice && !controller.AllowPriorityImprovement {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionSetPriority, Err: ErrPriorityImprovementRequiresPrivilege})
			continue
		}
		if err := controller.validateProcessGeneration(process); err != nil {
			if shouldClearStaleProcessState(err) {
				result.Runtime.ClearPriorityChange(process.ID)
			}
			if shouldReportControlFailure(err) {
				result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionSetPriority, Err: err})
			}
			continue
		}
		if err := controller.Priority.SetPriority(process, targetNice); err != nil {
			if errors.Is(err, ErrProcessExited) {
				result.Runtime.ClearPriorityChange(process.ID)
				continue
			}
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionSetPriority, Err: err})
			continue
		}
		result.Runtime.RecordPriorityChange(process, group.ID, originalNice, targetNice, reason, at)
	}

	return result
}

func (controller *AppGroupController) RestorePriorityGroup(group core.AppGroup, state core.RuntimeState, reason core.ControlReason, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}

	for _, process := range group.Processes {
		change, ok := state.PriorityChanges[process.ID.Key()]
		if !ok {
			continue
		}
		if controller.Priority == nil {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionRestorePriority, Err: ErrUnsupportedController})
			continue
		}
		if change.OriginalNice < change.TargetNice && !controller.AllowPriorityImprovement {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionRestorePriority, Err: ErrPriorityImprovementRequiresPrivilege})
			continue
		}
		if err := controller.validateProcessGeneration(process); err != nil {
			if shouldClearStaleProcessState(err) {
				result.Runtime.ClearPriorityChange(process.ID)
			}
			if shouldReportControlFailure(err) {
				result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionRestorePriority, Err: err})
			}
			continue
		}
		if err := controller.Priority.SetPriority(process, change.OriginalNice); err != nil {
			if errors.Is(err, ErrProcessExited) {
				result.Runtime.ClearPriorityChange(process.ID)
				continue
			}
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionRestorePriority, Err: err})
			continue
		}
		result.Runtime.ClearPriorityChange(process.ID)
	}

	return result
}

func (controller *AppGroupController) HideGroup(group core.AppGroup, state core.RuntimeState, reason core.ControlReason, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}
	if controller.Lifecycle == nil {
		result.Failures = append(result.Failures, ControlFailure{Action: core.ControlActionHide, Err: ErrUnsupportedController})
		return result
	}
	if len(group.Processes) == 0 {
		return result
	}
	process := group.Processes[0]
	if err := controller.validateProcessGeneration(process); err != nil {
		if shouldReportControlFailure(err) {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionHide, Err: err})
		}
		return result
	}
	if err := controller.Lifecycle.Hide(process); err != nil {
		if errors.Is(err, ErrProcessExited) {
			result.Runtime.ClearHidden(group.ID)
			return result
		}
		result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionHide, Err: err})
		return result
	}
	result.Runtime.RecordHidden(process, group.ID, reason, at)
	return result
}

func (controller *AppGroupController) ActivateGroup(group core.AppGroup, state core.RuntimeState, reason core.ControlReason, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}
	if controller.Lifecycle == nil {
		result.Failures = append(result.Failures, ControlFailure{Action: core.ControlActionActivate, Err: ErrUnsupportedController})
		return result
	}
	if hidden, ok := state.HiddenApps[group.ID.Key()]; ok {
		if hidden.Process.ID.IsEmpty() {
			result.Runtime.ClearHidden(group.ID)
			return result
		}
		group.Processes = []core.ProcessRef{hidden.Process}
	}
	if len(group.Processes) == 0 {
		result.Runtime.ClearHidden(group.ID)
		return result
	}
	process := group.Processes[0]
	if err := controller.validateProcessGeneration(process); err != nil {
		if shouldClearStaleProcessState(err) {
			result.Runtime.ClearHidden(group.ID)
		}
		if shouldReportControlFailure(err) {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionActivate, Err: err})
		}
		return result
	}
	if err := controller.Lifecycle.Activate(process); err != nil && !errors.Is(err, ErrProcessExited) {
		result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionActivate, Err: err})
		return result
	}
	result.Runtime.ClearHidden(group.ID)
	return result
}

func (controller *AppGroupController) QuitGroup(group core.AppGroup, state core.RuntimeState, reason core.ControlReason, at time.Time) ControlResult {
	state = state.Clone()
	state.EnsureMaps()
	result := ControlResult{Runtime: state}
	if controller.Lifecycle == nil {
		result.Failures = append(result.Failures, ControlFailure{Action: core.ControlActionQuit, Err: ErrUnsupportedController})
		return result
	}
	if len(group.Processes) == 0 {
		return result
	}
	process := group.Processes[0]
	if err := controller.validateProcessGeneration(process); err != nil {
		if shouldReportControlFailure(err) {
			result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionQuit, Err: err})
		}
		return result
	}
	if err := controller.Lifecycle.Terminate(process); err != nil && !errors.Is(err, ErrProcessExited) {
		result.Failures = append(result.Failures, ControlFailure{Process: process, Action: core.ControlActionQuit, Err: err})
	}
	return result
}

func (controller *Controller) executeActions(groups []core.AppGroup, actions []core.ControlAction, state core.RuntimeState) (core.RuntimeState, []ControlFailure) {
	groupsByKey := make(map[string]core.AppGroup, len(groups))
	for _, group := range groups {
		groupsByKey[group.ID.Key()] = group
	}

	failures := make([]ControlFailure, 0)
	for _, action := range actions {
		group := groupForAction(groupsByKey, action)
		var result ControlResult
		switch action.Type {
		case core.ControlActionPause:
			result = controller.groupCtl.PauseGroup(group, state, action.Reason, action.At)
		case core.ControlActionResume:
			result = controller.groupCtl.ResumeGroup(group, state, action.Reason, action.At)
			if result.Runtime.IsHiddenByOpenTamer(group.ID) {
				activateResult := controller.groupCtl.ActivateGroup(group, result.Runtime, action.Reason, action.At)
				result.Runtime = activateResult.Runtime
				result.Failures = append(result.Failures, activateResult.Failures...)
			}
		case core.ControlActionSetPriority:
			if action.NiceValue == nil {
				continue
			}
			result = controller.groupCtl.SetPriorityGroup(group, state, *action.NiceValue, action.Reason, action.At)
		case core.ControlActionRestorePriority:
			result = controller.groupCtl.RestorePriorityGroup(group, state, action.Reason, action.At)
		case core.ControlActionWakeBriefly:
			result = controller.groupCtl.WakeGroupBriefly(group, state, action.At)
		case core.ControlActionHide:
			if action.Message == "hide_when_stopped" && !groupPaused(group, state) {
				continue
			}
			result = controller.groupCtl.HideGroup(group, state, action.Reason, action.At)
		case core.ControlActionActivate:
			result = controller.groupCtl.ActivateGroup(group, state, action.Reason, action.At)
		case core.ControlActionQuit:
			result = controller.groupCtl.QuitGroup(group, state, action.Reason, action.At)
		default:
			continue
		}
		state = result.Runtime
		failures = append(failures, result.Failures...)
	}
	return state, failures
}

func groupForAction(groupsByKey map[string]core.AppGroup, action core.ControlAction) core.AppGroup {
	if group, ok := groupsByKey[action.AppID.Key()]; ok {
		if len(action.Processes) > 0 {
			group.Processes = processesForAction(group, action)
		}
		return group
	}
	return groupFromAction(action)
}

func processesForAction(group core.AppGroup, action core.ControlAction) []core.ProcessRef {
	processes := make([]core.ProcessRef, 0, len(action.Processes))
	for _, id := range action.Processes {
		if process, ok := processInGroup(group, id); ok {
			processes = append(processes, process)
			continue
		}
		processes = append(processes, core.ProcessRef{ID: id, Name: action.AppID.DisplayName()})
	}
	return processes
}

func processInGroup(group core.AppGroup, id core.ProcessID) (core.ProcessRef, bool) {
	for _, process := range group.Processes {
		if process.ID.SameGeneration(id) {
			return process, true
		}
	}
	return core.ProcessRef{}, false
}

func groupFromAction(action core.ControlAction) core.AppGroup {
	processes := make([]core.ProcessRef, 0, len(action.Processes))
	for _, id := range action.Processes {
		processes = append(processes, core.ProcessRef{ID: id, Name: action.AppID.DisplayName()})
	}
	return core.AppGroup{ID: action.AppID, Processes: processes}
}

func (controller *AppGroupController) validateProcessGeneration(process core.ProcessRef) error {
	if controller == nil || controller.ProcessValidator == nil {
		return nil
	}
	return controller.ProcessValidator.ValidateProcessGeneration(process)
}

func shouldClearStaleProcessState(err error) bool {
	return errors.Is(err, ErrProcessExited) || errors.Is(err, ErrProcessGenerationMismatch)
}

func shouldReportControlFailure(err error) bool {
	return err != nil && !errors.Is(err, ErrProcessExited)
}

func cpuLimitRequests(groups []core.AppGroup, actions []core.ControlAction) []CPULimitRequest {
	groupsByKey := make(map[string]core.AppGroup, len(groups))
	for _, group := range groups {
		groupsByKey[group.ID.Key()] = group
	}

	requests := make([]CPULimitRequest, 0)
	for _, action := range actions {
		if action.Type != core.ControlActionLimitCPU || action.RunFor <= 0 || action.StopFor <= 0 {
			continue
		}
		group, ok := groupsByKey[action.AppID.Key()]
		if !ok || len(group.Processes) == 0 {
			continue
		}
		requests = append(requests, CPULimitRequest{
			AppID:     action.AppID,
			Processes: group.Processes,
			RunFor:    action.RunFor,
			StopFor:   action.StopFor,
		})
	}
	return requests
}

func groupPaused(group core.AppGroup, state core.RuntimeState) bool {
	if len(group.Processes) == 0 {
		return false
	}
	for _, process := range group.Processes {
		if !state.IsPausedByOpenTamer(process.ID) {
			return false
		}
	}
	return true
}
