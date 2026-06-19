package observe

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type AppID = core.AppID
type ProcessID = core.ProcessID
type ProcessRef = core.ProcessRef
type AppGroup = core.AppGroup
type AppProcessHint = core.AppProcessHint
type SafetyDecision = core.SafetyDecision
type SafetyPolicy = core.SafetyPolicy

const (
	AppKindUserApp          = core.AppKindUserApp
	AppKindEssential        = core.AppKindEssential
	AppStatusObserved       = core.AppStatusObserved
	ControllabilityNormal   = core.ControllabilityNormal
	ControllabilitySlowOnly = core.ControllabilitySlowOnly
	ControllabilityBlocked  = core.ControllabilityBlocked
)

type ProcessGrouper struct {
	Safety          SafetyPolicy
	AggregateByName bool
}

func NewProcessGrouper() ProcessGrouper {
	return ProcessGrouper{
		Safety: core.NewSafetyPolicy(0),
	}
}

func (grouper ProcessGrouper) Group(processes []ProcessRef, hints []AppProcessHint) []AppGroup {
	if grouper.AggregateByName {
		return grouper.groupByName(processes)
	}

	processByPID := make(map[int]ProcessRef, len(processes))
	for _, process := range processes {
		if process.ID.PID != 0 {
			processByPID[process.ID.PID] = process
		}
	}

	groups := make(map[string]*AppGroup)
	pidToGroup := make(map[int]string)

	for _, hint := range hints {
		if hint.AppID.IsEmpty() && hint.PrimaryPID.PID == 0 {
			continue
		}
		key := hint.AppID.Key()
		if key == "unknown" && hint.PrimaryPID.PID != 0 {
			key = "pid:" + strconv.Itoa(hint.PrimaryPID.PID)
		}
		group := ensureGroup(groups, key, AppGroup{
			ID:              hint.AppID,
			Kind:            AppKindUserApp,
			Controllability: ControllabilityNormal,
			Status:          AppStatusObserved,
		})
		if primary, ok := processByPID[hint.PrimaryPID.PID]; ok {
			addProcess(group, primary)
			pidToGroup[primary.ID.PID] = key
		}
	}

	for _, process := range processes {
		if process.ID.PID == 0 || pidToGroup[process.ID.PID] != "" {
			continue
		}
		if key := grouper.matchHint(process, hints); key != "" {
			group := ensureGroup(groups, key, AppGroup{
				ID:              appIDForHintKey(key, hints),
				Kind:            AppKindUserApp,
				Controllability: ControllabilityNormal,
				Status:          AppStatusObserved,
			})
			addProcess(group, process)
			pidToGroup[process.ID.PID] = key
		}
	}

	changed := true
	for changed {
		changed = false
		for _, process := range processes {
			if process.ID.PID == 0 || pidToGroup[process.ID.PID] != "" || process.ParentPID == 0 {
				continue
			}
			parentGroup := pidToGroup[process.ParentPID]
			if parentGroup == "" {
				continue
			}
			addProcess(groups[parentGroup], process)
			pidToGroup[process.ID.PID] = parentGroup
			changed = true
		}
	}

	for _, process := range processes {
		if process.ID.PID == 0 || pidToGroup[process.ID.PID] != "" {
			continue
		}
		decision := grouper.classifyStandalone(process)
		key := "pid:" + strconv.Itoa(process.ID.PID)
		group := ensureGroup(groups, key, AppGroup{
			ID: AppID{
				Path: process.ExecutablePath,
				Name: process.DisplayName(),
			},
			Kind:            decision.Kind,
			Controllability: decision.Controllability,
			SafetyReason:    decision.Reason,
			Status:          AppStatusObserved,
		})
		addProcess(group, process)
	}

	result := make([]AppGroup, 0, len(groups))
	for _, group := range groups {
		sort.Slice(group.Processes, func(i, j int) bool {
			return group.Processes[i].ID.PID < group.Processes[j].ID.PID
		})
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].DisplayName()) < strings.ToLower(result[j].DisplayName())
	})
	return result
}

func (grouper ProcessGrouper) groupByName(processes []ProcessRef) []AppGroup {
	groups := make(map[string]*AppGroup)
	for _, process := range processes {
		name := strings.TrimSpace(process.DisplayName())
		if name == "" {
			name = "Unknown Process"
		}
		key := "name:" + strings.ToLower(name)
		group := ensureGroup(groups, key, AppGroup{
			ID:     AppID{Name: name},
			Status: AppStatusObserved,
		})
		addProcess(group, process)
	}

	result := make([]AppGroup, 0, len(groups))
	for _, group := range groups {
		sort.Slice(group.Processes, func(i, j int) bool {
			return group.Processes[i].ID.PID < group.Processes[j].ID.PID
		})
		*group = core.ApplySafetyDecision(*group, grouper.Safety.ClassifyGroup(*group))
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].DisplayName()) < strings.ToLower(result[j].DisplayName())
	})
	return result
}

func (grouper ProcessGrouper) matchHint(process ProcessRef, hints []AppProcessHint) string {
	for _, hint := range hints {
		key := hint.AppID.Key()
		if key == "unknown" && hint.PrimaryPID.PID != 0 {
			key = "pid:" + strconv.Itoa(hint.PrimaryPID.PID)
		}
		if key == "unknown" {
			continue
		}
		if process.BundleID != "" && hint.AppID.BundleID != "" && strings.EqualFold(process.BundleID, hint.AppID.BundleID) {
			return key
		}
		if hint.BundlePath != "" && pathIsInside(process.ExecutablePath, hint.BundlePath) {
			return key
		}
		if hint.ExecutablePath != "" && filepath.Clean(process.ExecutablePath) == filepath.Clean(hint.ExecutablePath) {
			return key
		}
	}
	return ""
}

func (grouper ProcessGrouper) classifyStandalone(process ProcessRef) SafetyDecision {
	return grouper.Safety.ClassifyProcess(process)
}

func ensureGroup(groups map[string]*AppGroup, key string, initial AppGroup) *AppGroup {
	if group, ok := groups[key]; ok {
		return group
	}
	initial.Processes = make([]ProcessRef, 0)
	groups[key] = &initial
	return &initial
}

func addProcess(group *AppGroup, process ProcessRef) {
	for _, existing := range group.Processes {
		if existing.ID.SameGeneration(process.ID) {
			return
		}
	}
	group.Processes = append(group.Processes, process)
}

func appIDForHintKey(key string, hints []AppProcessHint) AppID {
	for _, hint := range hints {
		hintKey := hint.AppID.Key()
		if hintKey == "unknown" && hint.PrimaryPID.PID != 0 {
			hintKey = "pid:" + strconv.Itoa(hint.PrimaryPID.PID)
		}
		if hintKey == key {
			return hint.AppID
		}
	}
	return AppID{}
}

func pathIsInside(path string, dir string) bool {
	if path == "" || dir == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	cleanDir := filepath.Clean(dir)
	rel, err := filepath.Rel(cleanDir, cleanPath)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
