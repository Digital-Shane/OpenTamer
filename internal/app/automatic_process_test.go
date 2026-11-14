package app

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

type identityTestLookup struct {
	process core.ProcessRef
	err     error
}

func (lookup identityTestLookup) ProcessIdentity(int) (core.ProcessRef, error) {
	return lookup.process, lookup.err
}

func TestAutomaticProcessValidatorRejectsChangedIdentity(t *testing.T) {
	process := core.ProcessRef{
		ID: core.ProcessID{PID: 87654, StartTime: time.Unix(1, 0)}, UID: os.Getuid(),
		Name: "Worker", ExecutablePath: "/Applications/Worker.app/Contents/MacOS/Worker",
	}
	for _, tc := range []struct {
		name   string
		change func(*identityTestLookup)
		valid  bool
	}{
		{"unchanged", func(*identityTestLookup) {}, true},
		{"reused PID", func(l *identityTestLookup) { l.process.ID.StartTime = time.Unix(2, 0) }, false},
		{"same generation exec", func(l *identityTestLookup) { l.process.ExecutablePath = "/usr/libexec/session-service" }, false},
		{"changed owner", func(l *identityTestLookup) { l.process.UID++ }, false},
		{"changed name", func(l *identityTestLookup) { l.process.Name = "Dock" }, false},
		{"unknown generation", func(l *identityTestLookup) { l.process.ID.StartTime = time.Time{} }, false},
		{"lookup error", func(l *identityTestLookup) { l.err = errors.New("identity unavailable") }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lookup := identityTestLookup{process: process}
			tc.change(&lookup)
			validator := automaticCPUValidator{lookup: lookup}
			if err := validator.ValidateAutomaticProcess(process); (err == nil) != tc.valid {
				t.Fatalf("validation error = %v, want valid %t", err, tc.valid)
			}
		})
	}
}
