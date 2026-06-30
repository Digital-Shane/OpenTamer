package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

const CurrentSchemaVersion = 1

type Config struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Preferences   core.GlobalPreferences `json:"preferences"`
	Rules         []core.AppRule         `json:"rules"`
}

func DefaultConfig() Config {
	return Config{
		SchemaVersion: CurrentSchemaVersion,
		Preferences: core.GlobalPreferences{
			ManagementEnabled:       true,
			CPULimiterEnabled:       true,
			AggregateByName:         true,
			ShowMenuBarIcon:         true,
			TopProcessesSort:        core.TopProcessesSortCurrent,
			CPUDisplayMode:          core.CPUDisplayModePerCoreProcess,
			StatsInterval:           3 * time.Second,
			AveragingWindow:         30 * time.Second,
			CPUGraphWindow:          core.DefaultCPUGraphWindow,
			WakeGrace:               30 * time.Second,
			HighCPUDetectionEnabled: true,
			HighCPUThreshold:        75,
			HighCPUDuration:         30 * time.Second,
			HighCPUCooldown:         10 * time.Minute,
			Theme:                   "system",
		},
		Rules: make([]core.AppRule, 0),
	}
}

func MigrateConfig(in Config) (Config, error) {
	defaults := DefaultConfig()
	if in.SchemaVersion < 0 {
		return defaults, fmt.Errorf("invalid schema version %d", in.SchemaVersion)
	}
	if in.SchemaVersion == 0 {
		in.SchemaVersion = 1
	}
	if in.SchemaVersion > CurrentSchemaVersion {
		return defaults, fmt.Errorf("config schema %d is newer than supported schema %d", in.SchemaVersion, CurrentSchemaVersion)
	}
	if in.Preferences.StatsInterval == 0 {
		in.Preferences.StatsInterval = defaults.Preferences.StatsInterval
	}
	if in.Preferences.AveragingWindow == 0 {
		in.Preferences.AveragingWindow = defaults.Preferences.AveragingWindow
	}
	in.Preferences.TopProcessesSort = core.NormalizeTopProcessesSortMode(in.Preferences.TopProcessesSort)
	in.Preferences.CPUDisplayMode = core.NormalizeCPUDisplayMode(in.Preferences.CPUDisplayMode)
	in.Preferences.CPUGraphWindow = core.NormalizeCPUGraphWindow(in.Preferences.CPUGraphWindow)

	in.Preferences.Theme = core.NormalizeThemeMode(in.Preferences.Theme)

	if in.Preferences.HighCPUThreshold == 0 {
		in.Preferences.HighCPUThreshold = defaults.Preferences.HighCPUThreshold
	}
	if in.Preferences.HighCPUDuration == 0 {
		in.Preferences.HighCPUDuration = defaults.Preferences.HighCPUDuration
	}
	if in.Preferences.HighCPUCooldown == 0 {
		in.Preferences.HighCPUCooldown = defaults.Preferences.HighCPUCooldown
	}
	if in.Rules == nil {
		in.Rules = make([]core.AppRule, 0)
	}
	for i := range in.Rules {
		normalized := core.NormalizeTrackIn(in.Rules[i].TrackIn)
		if len(normalized) == 0 {
			normalized = in.Rules[i].EffectiveTrackIn()
		}
		in.Rules[i].TrackIn = normalized
	}
	return in, nil
}

type Store struct {
	Dir string
}

func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "OpenTamer"), nil
}

func NewStore(dir string) Store {
	return Store{Dir: dir}
}

func (store Store) LoadConfig() (Config, error) {
	path := store.configPath()
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return DefaultConfig(), err
	}

	var cfg Config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("decode config: %w", err)
	}
	legacyStartupGrace, hasLegacyStartupGrace := legacyDurationPreference(payload, "startupGrace")
	hasLegacyTrackInTray := configUsesLegacyTrayTracking(cfg)
	migrated, err := MigrateConfig(cfg)
	if err != nil {
		return migrated, err
	}
	changed := hasLegacyTrackInTray
	if !configHasPreference(payload, "managementEnabled") {
		migrated.Preferences.ManagementEnabled = DefaultConfig().Preferences.ManagementEnabled
		changed = true
	}
	if !configHasPreference(payload, "cpuLimiterEnabled") {
		migrated.Preferences.CPULimiterEnabled = DefaultConfig().Preferences.CPULimiterEnabled
		changed = true
	}
	if !configHasPreference(payload, "aggregateByName") {
		migrated.Preferences.AggregateByName = DefaultConfig().Preferences.AggregateByName
		changed = true
	}
	if !configHasPreference(payload, "showMenuBarIcon") {
		migrated.Preferences.ShowMenuBarIcon = DefaultConfig().Preferences.ShowMenuBarIcon
		changed = true
	}
	if !configHasPreference(payload, "topProcessesSort") {
		migrated.Preferences.TopProcessesSort = DefaultConfig().Preferences.TopProcessesSort
		changed = true
	}
	if !configHasPreference(payload, "cpuDisplayMode") {
		migrated.Preferences.CPUDisplayMode = DefaultConfig().Preferences.CPUDisplayMode
		changed = true
	}
	if !configHasPreference(payload, "cpuGraphWindow") {
		migrated.Preferences.CPUGraphWindow = DefaultConfig().Preferences.CPUGraphWindow
		changed = true
	}

	if !configHasPreference(payload, "theme") {
		migrated.Preferences.Theme = DefaultConfig().Preferences.Theme
		changed = true
	}

	if !configHasPreference(payload, "wakeGrace") {
		migrated.Preferences.WakeGrace = DefaultConfig().Preferences.WakeGrace
		if hasLegacyStartupGrace {
			migrated.Preferences.WakeGrace = legacyStartupGrace
		}
		changed = true
	}
	if hasLegacyStartupGrace {
		changed = true
	}
	if !configHasPreference(payload, "highCPUDetectionEnabled") {
		migrated.Preferences.HighCPUDetectionEnabled = DefaultConfig().Preferences.HighCPUDetectionEnabled
		changed = true
	}
	if changed {
		if err := store.SaveConfig(migrated); err != nil {
			return migrated, fmt.Errorf("backfill config defaults: %w", err)
		}
	}
	return migrated, nil
}

func (store Store) SaveConfig(cfg Config) error {
	migrated, err := MigrateConfig(cfg)
	if err != nil {
		return err
	}
	return store.writeJSON(store.configPath(), migrated)
}

func configHasPreference(payload []byte, name string) bool {
	var raw struct {
		Preferences map[string]json.RawMessage `json:"preferences"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return true
	}
	if raw.Preferences == nil {
		return false
	}
	_, ok := raw.Preferences[name]
	return ok
}

func configUsesLegacyTrayTracking(cfg Config) bool {
	for _, rule := range cfg.Rules {
		for _, location := range rule.TrackIn {
			if location == core.RuleTrackingLocation("tray") {
				return true
			}
		}
	}
	return false
}

func legacyDurationPreference(payload []byte, name string) (time.Duration, bool) {
	var raw struct {
		Preferences map[string]json.RawMessage `json:"preferences"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil || raw.Preferences == nil {
		return 0, false
	}
	value, ok := raw.Preferences[name]
	if !ok {
		return 0, false
	}
	var nanos int64
	if err := json.Unmarshal(value, &nanos); err != nil {
		return 0, false
	}
	return time.Duration(nanos), true
}

func (store Store) LoadRuntimeState() (core.RuntimeState, error) {
	payload, err := os.ReadFile(store.runtimePath())
	if errors.Is(err, os.ErrNotExist) {
		return core.NewRuntimeState(), nil
	}
	if err != nil {
		return core.NewRuntimeState(), err
	}
	var state core.RuntimeState
	if err := json.Unmarshal(payload, &state); err != nil {
		return core.NewRuntimeState(), fmt.Errorf("decode runtime state: %w", err)
	}
	state = state.Clone()
	return state, nil
}

func (store Store) SaveRuntimeState(state core.RuntimeState) error {
	return store.writeJSON(store.runtimePath(), state)
}

func (store Store) LoadStats() (core.StatsDocument, error) {
	payload, err := os.ReadFile(store.statsPath())
	if errors.Is(err, os.ErrNotExist) {
		return core.NewStatsDocument(), nil
	}
	if err != nil {
		return core.NewStatsDocument(), err
	}
	var document core.StatsDocument
	if err := json.Unmarshal(payload, &document); err != nil {
		return core.NewStatsDocument(), fmt.Errorf("decode stats: %w", err)
	}
	if document.SchemaVersion == 0 {
		document.SchemaVersion = core.StatsSchemaVersion
	}
	if document.Apps == nil {
		document.Apps = make(map[string]core.AppStats)
	}
	return document, nil
}

func (store Store) SaveStats(document core.StatsDocument) error {
	if document.SchemaVersion == 0 {
		document.SchemaVersion = core.StatsSchemaVersion
	}
	return store.writeJSON(store.statsPath(), document)
}

func (store Store) configPath() string {
	return filepath.Join(store.Dir, "config.json")
}

func (store Store) runtimePath() string {
	return filepath.Join(store.Dir, "runtime-state.json")
}

func (store Store) statsPath() string {
	return filepath.Join(store.Dir, "stats.json")
}

func (store Store) writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
