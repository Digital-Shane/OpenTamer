package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// AppID identifies the user-facing app when bundle metadata is available.
type AppID struct {
	BundleID string `json:"bundleID,omitempty"`
	Path     string `json:"path,omitempty"`
	Name     string `json:"name,omitempty"`
}

func (id AppID) IsEmpty() bool {
	return id.BundleID == "" && id.Path == "" && id.Name == ""
}

func (id AppID) DisplayName() string {
	if id.Name != "" {
		return id.Name
	}
	if id.BundleID != "" {
		return id.BundleID
	}
	if id.Path != "" {
		base := filepath.Base(id.Path)
		if base != "." && base != "/" {
			return base
		}
	}
	return "Unknown App"
}

func (id AppID) Key() string {
	if id.BundleID != "" {
		return "bundle:" + strings.ToLower(id.BundleID)
	}
	if id.Path != "" {
		return "path:" + filepath.Clean(id.Path)
	}
	if id.Name != "" {
		return "name:" + strings.ToLower(id.Name)
	}
	return "unknown"
}

func (id AppID) Matches(other AppID) bool {
	if id.BundleID != "" && other.BundleID != "" {
		return strings.EqualFold(id.BundleID, other.BundleID)
	}
	if id.Path != "" && other.Path != "" {
		return filepath.Clean(id.Path) == filepath.Clean(other.Path)
	}
	if id.BundleID == "" && other.BundleID == "" && id.Path == "" && other.Path == "" {
		return id.Name != "" && other.Name != "" && strings.EqualFold(id.Name, other.Name)
	}
	return false
}

// ProcessID includes a process generation marker so PID reuse can be detected.
type ProcessID struct {
	PID       int       `json:"pid"`
	StartTime time.Time `json:"startTime"`
}

func (id ProcessID) IsEmpty() bool {
	return id.PID == 0 && id.StartTime.IsZero()
}

func (id ProcessID) Key() string {
	if id.StartTime.IsZero() {
		return fmt.Sprintf("pid:%d", id.PID)
	}
	return fmt.Sprintf("pid:%d:%d", id.PID, id.StartTime.UnixNano())
}

func (id ProcessID) SameGeneration(other ProcessID) bool {
	if id.PID != other.PID {
		return false
	}
	if id.StartTime.IsZero() || other.StartTime.IsZero() {
		return id.StartTime.IsZero() && other.StartTime.IsZero()
	}
	return id.StartTime.Equal(other.StartTime)
}

type ProcessRef struct {
	ID             ProcessID `json:"id"`
	ParentPID      int       `json:"parentPID,omitempty"`
	UID            int       `json:"uid,omitempty"`
	GID            int       `json:"gid,omitempty"`
	Nice           int       `json:"nice,omitempty"`
	Name           string    `json:"name,omitempty"`
	ExecutablePath string    `json:"executablePath,omitempty"`
	BundleID       string    `json:"bundleID,omitempty"`
}

func (ref ProcessRef) DisplayName() string {
	if ref.Name != "" {
		return ref.Name
	}
	if ref.ExecutablePath != "" {
		return filepath.Base(ref.ExecutablePath)
	}
	if ref.ID.PID != 0 {
		return fmt.Sprintf("PID %d", ref.ID.PID)
	}
	return "Unknown Process"
}
