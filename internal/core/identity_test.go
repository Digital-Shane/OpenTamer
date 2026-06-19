package core

import (
	"testing"
	"time"
)

func TestAppIDMatchesBundleIDFirst(t *testing.T) {
	left := AppID{BundleID: "com.example.Editor", Path: "/Applications/Editor.app", Name: "Editor"}
	right := AppID{BundleID: "COM.EXAMPLE.EDITOR", Path: "/Different/Editor.app", Name: "Other"}

	if !left.Matches(right) {
		t.Fatal("expected bundle IDs to match case-insensitively")
	}
}

func TestAppIDDoesNotFallbackWhenBundleIDsConflict(t *testing.T) {
	left := AppID{BundleID: "com.example.one", Path: "/Applications/Same.app"}
	right := AppID{BundleID: "com.example.two", Path: "/Applications/Same.app"}

	if left.Matches(right) {
		t.Fatal("conflicting bundle IDs should not match via path fallback")
	}
}

func TestAppIDMatchesPathWhenBundleIDsMissing(t *testing.T) {
	left := AppID{Path: "/Applications/Test.app/Contents/MacOS/Test"}
	right := AppID{Path: "/Applications/Test.app/Contents/MacOS/../MacOS/Test"}

	if !left.Matches(right) {
		t.Fatal("expected clean paths to match")
	}
}

func TestAppIDMatchesNameOnlyWhenNoStrongerIdentityExists(t *testing.T) {
	left := AppID{Name: "Preview"}
	right := AppID{Name: "preview"}

	if !left.Matches(right) {
		t.Fatal("expected name-only identities to match case-insensitively")
	}

	if left.Matches(AppID{Name: "Preview", Path: "/Applications/Preview.app"}) {
		t.Fatal("name should not match when only one side has a stronger identity")
	}
}

func TestProcessIDSameGenerationUsesStartTime(t *testing.T) {
	started := time.Unix(100, 10)
	same := ProcessID{PID: 42, StartTime: started}
	reused := ProcessID{PID: 42, StartTime: started.Add(time.Second)}

	if !same.SameGeneration(ProcessID{PID: 42, StartTime: started}) {
		t.Fatal("expected same pid and start time to match")
	}
	if same.SameGeneration(reused) {
		t.Fatal("expected reused pid with different start time not to match")
	}
}

func TestProcessIDWithoutStartTimeMatchesOnlyAnotherUnknownGeneration(t *testing.T) {
	unknown := ProcessID{PID: 42}

	if !unknown.SameGeneration(ProcessID{PID: 42}) {
		t.Fatal("expected unknown generations with same pid to match")
	}
	if unknown.SameGeneration(ProcessID{PID: 42, StartTime: time.Unix(100, 0)}) {
		t.Fatal("expected unknown generation not to match known generation")
	}
}
