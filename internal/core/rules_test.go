package core

import "testing"

func TestRuleTrackingDefaults(t *testing.T) {
	observe := AppRule{Mode: RuleModeObserveOnly}
	if !observe.TracksIn(RuleTrackInMenuBar) || !observe.TracksIn(RuleTrackInManagedApps) {
		t.Fatalf("observe defaults = %#v, want menu bar and managed apps", observe.EffectiveTrackIn())
	}

	pause := AppRule{Mode: RuleModePauseInBackground}
	if pause.TracksIn(RuleTrackInMenuBar) || !pause.TracksIn(RuleTrackInManagedApps) {
		t.Fatalf("pause defaults = %#v, want managed apps only", pause.EffectiveTrackIn())
	}
}

func TestRuleTrackingCanBeCombined(t *testing.T) {
	rule := AppRule{
		Mode:    RuleModeObserveOnly,
		TrackIn: []RuleTrackingLocation{RuleTrackInMenuBar},
	}

	rule = rule.WithTrackIn(RuleTrackInManagedApps)
	if !rule.TracksIn(RuleTrackInMenuBar) || !rule.TracksIn(RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want both locations", rule.TrackIn)
	}
}

func TestRuleTrackingCanRemoveLocation(t *testing.T) {
	rule := AppRule{
		Mode:    RuleModeObserveOnly,
		TrackIn: []RuleTrackingLocation{RuleTrackInMenuBar, RuleTrackInManagedApps},
	}

	rule = rule.WithoutTrackIn(RuleTrackInMenuBar)
	if rule.TracksIn(RuleTrackInMenuBar) || !rule.TracksIn(RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want managed apps only", rule.TrackIn)
	}
}

func TestRuleTrackingCanRepresentNoLocations(t *testing.T) {
	rule := AppRule{
		Mode:    RuleModePauseInBackground,
		TrackIn: []RuleTrackingLocation{RuleTrackInManagedApps},
	}

	rule = rule.WithoutTrackIn(RuleTrackInManagedApps)
	if rule.TracksIn(RuleTrackInMenuBar) || rule.TracksIn(RuleTrackInManagedApps) {
		t.Fatalf("track in = %#v, want no locations", rule.TrackIn)
	}
	if len(rule.TrackIn) != 1 || rule.TrackIn[0] != RuleTrackInNone {
		t.Fatalf("track in = %#v, want explicit none sentinel", rule.TrackIn)
	}
}

func TestNormalizeTrackInAcceptsLegacyTrayValue(t *testing.T) {
	locations := NormalizeTrackIn([]RuleTrackingLocation{"tray", RuleTrackInManagedApps})
	if len(locations) != 2 || locations[0] != RuleTrackInMenuBar || locations[1] != RuleTrackInManagedApps {
		t.Fatalf("locations = %#v, want menu bar and managed apps", locations)
	}
}
