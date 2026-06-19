package appkit

import "testing"

func TestDecodeRunningAppsJSON(t *testing.T) {
	apps, err := decodeRunningAppsJSON(`[{"pid":123,"bundleID":"com.example.App","localizedName":"Example","executablePath":"/Applications/Example.app/Contents/MacOS/Example","bundlePath":"/Applications/Example.app","activationPolicy":0,"active":true}]`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("len(apps) = %d, want 1", len(apps))
	}
	if apps[0].AppID().BundleID != "com.example.App" {
		t.Fatalf("bundle id = %q", apps[0].AppID().BundleID)
	}
	if apps[0].ProcessRef().ID.PID != 123 {
		t.Fatalf("pid = %d, want 123", apps[0].ProcessRef().ID.PID)
	}
}

func TestDecodeAppEventJSONDefaultsTime(t *testing.T) {
	event, err := decodeAppEventJSON(`{"kind":"activated","app":{"pid":7,"localizedName":"Finder"}}`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if event.Kind != AppEventActivated {
		t.Fatalf("kind = %q", event.Kind)
	}
	if event.At.IsZero() {
		t.Fatal("expected default event time")
	}
}
