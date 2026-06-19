package core

const StatsSchemaVersion = 1

type AppStats struct {
	AppID           AppID `json:"appID"`
	AutomaticPauses int   `json:"automaticPauses"`
}

type StatsDocument struct {
	SchemaVersion   int                 `json:"schemaVersion"`
	AutomaticPauses int                 `json:"automaticPauses"`
	Apps            map[string]AppStats `json:"apps"`
}

func NewStatsDocument() StatsDocument {
	return StatsDocument{
		SchemaVersion: StatsSchemaVersion,
		Apps:          make(map[string]AppStats),
	}
}

func (stats *StatsDocument) RecordAutomaticPause(app AppID) {
	if stats.Apps == nil {
		stats.Apps = make(map[string]AppStats)
	}
	key := app.Key()
	appStats := stats.Apps[key]
	appStats.AppID = app
	appStats.AutomaticPauses++
	stats.Apps[key] = appStats
	stats.AutomaticPauses++
}
