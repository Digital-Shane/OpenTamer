package core

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRuntimeStateTracksOpenTamerOwnedPause(t *testing.T) {
	state := NewRuntimeState()
	process := ProcessRef{ID: ProcessID{PID: 100, StartTime: time.Unix(10, 0)}, Name: "sleep"}
	app := AppID{BundleID: "com.example.sleep", Name: "Sleep"}

	state.RecordPause(process, app, ControlReasonUserRule, time.Unix(20, 0))

	if !state.IsPausedByOpenTamer(process.ID) {
		t.Fatal("expected pause ownership to be recorded")
	}

	state.ClearPause(process.ID)

	if state.IsPausedByOpenTamer(process.ID) {
		t.Fatal("expected pause ownership to be cleared")
	}
}

func TestRuleModesAndStatusesAreJSONSerializable(t *testing.T) {
	rule := AppRule{
		AppID:          AppID{BundleID: "com.example.app", Name: "Example"},
		Mode:           RuleModePauseInBackground,
		BackgroundOnly: true,
	}
	group := AppGroup{
		ID:              rule.AppID,
		Kind:            AppKindUserApp,
		Controllability: ControllabilityNormal,
		Status:          AppStatusWaiting,
	}

	payload, err := json.Marshal(struct {
		Rule  AppRule  `json:"rule"`
		Group AppGroup `json:"group"`
	}{Rule: rule, Group: group})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Rule  AppRule  `json:"rule"`
		Group AppGroup `json:"group"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Rule.Mode != RuleModePauseInBackground {
		t.Fatalf("decoded mode = %q", decoded.Rule.Mode)
	}
	if decoded.Group.Status != AppStatusWaiting {
		t.Fatalf("decoded status = %q", decoded.Group.Status)
	}
}

func TestAppendActionKeepsLimit(t *testing.T) {
	state := NewRuntimeState()
	for i := range 5 {
		state.AppendAction(ControlAction{Type: ControlActionNoop, At: time.Unix(int64(i), 0)}, 3)
	}

	if len(state.LastActions) != 3 {
		t.Fatalf("last actions length = %d, want 3", len(state.LastActions))
	}
	if got := state.LastActions[0].At.Unix(); got != 2 {
		t.Fatalf("first retained action timestamp = %d, want 2", got)
	}
}
