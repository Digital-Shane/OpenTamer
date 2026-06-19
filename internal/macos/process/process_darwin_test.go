//go:build darwin && cgo

package process

import (
	"os"
	"testing"
)

func TestProcessEnumeratorFindsCurrentProcess(t *testing.T) {
	processes, err := NewProcessEnumerator().Processes()
	if err != nil {
		t.Fatalf("processes: %v", err)
	}

	currentPID := os.Getpid()
	for _, process := range processes {
		if process.ID.PID == currentPID {
			if process.Name == "" && process.ExecutablePath == "" {
				t.Fatal("current process should have a name or executable path")
			}
			return
		}
	}

	t.Fatalf("current pid %d was not found in %d processes", currentPID, len(processes))
}
