package core

import "testing"

func TestStatsDocumentRecordsAutomaticPausesPerApp(t *testing.T) {
	app := AppID{Name: "App"}
	stats := NewStatsDocument()

	stats.RecordAutomaticPause(app)
	stats.RecordAutomaticPause(app)

	if stats.AutomaticPauses != 2 {
		t.Fatalf("automatic pauses = %d, want 2", stats.AutomaticPauses)
	}
	if stats.Apps[app.Key()].AutomaticPauses != 2 {
		t.Fatalf("app pauses = %d, want 2", stats.Apps[app.Key()].AutomaticPauses)
	}
}
