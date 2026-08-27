package audit

import "testing"

func TestIsBrowserPrefetch(t *testing.T) {
	skip := []string{"image", "video", "document", "iframe"}
	for _, dest := range skip {
		if !IsBrowserPrefetch(dest) {
			t.Errorf("IsBrowserPrefetch(%q) = false, want true", dest)
		}
	}

	allow := []string{"", "empty", "audio", "object", "fetch", "worker"}
	for _, dest := range allow {
		if IsBrowserPrefetch(dest) {
			t.Errorf("IsBrowserPrefetch(%q) = true, want false", dest)
		}
	}
}
