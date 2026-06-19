package appkit

import "testing"

func TestRunningAppConvertsToProcessHint(t *testing.T) {
	app := RunningApp{
		PID:            99,
		BundleID:       "com.example.App",
		LocalizedName:  "Example",
		ExecutablePath: "/Applications/Example.app/Contents/MacOS/Example",
		BundlePath:     "/Applications/Example.app",
	}

	hint := app.AppProcessHint()
	if hint.PrimaryPID.PID != 99 {
		t.Fatalf("pid = %d, want 99", hint.PrimaryPID.PID)
	}
	if hint.AppID.BundleID != "com.example.App" {
		t.Fatalf("bundle id = %q", hint.AppID.BundleID)
	}
	if hint.BundlePath != "/Applications/Example.app" {
		t.Fatalf("bundle path = %q", hint.BundlePath)
	}
}
