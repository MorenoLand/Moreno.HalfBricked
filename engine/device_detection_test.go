package engine

import "testing"

func TestBrowserDeviceClassification(t *testing.T) {
	for _, item := range []struct {
		agent, platform string
		touches         int
		hint, mobile    bool
	}{{"Windows NT 10.0", "Win32", 0, false, false}, {"Windows NT 10.0", "Win32", 10, false, false}, {"Macintosh", "MacIntel", 0, false, false}, {"Macintosh", "MacIntel", 5, false, true}, {"Android", "Linux armv8l", 5, false, true}, {"iPhone", "iPhone", 5, false, true}, {"unknown", "", 0, true, true}} {
		if got := mobileBrowser(item.agent, item.platform, item.touches, item.hint); got != item.mobile {
			t.Fatalf("%+v classified %v", item, got)
		}
	}
}
