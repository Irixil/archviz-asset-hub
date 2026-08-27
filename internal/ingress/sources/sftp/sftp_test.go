package sftp

import "testing"

func TestShouldSkipFile(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"render.max":        false,
		"layout.PSD":        false,
		".DS_Store":         true,
		"upload.tmp":        true,
		"upload.PART":       true,
		"upload.crdownload": true,
	}

	for name, want := range tests {
		if got := shouldSkipFile(name); got != want {
			t.Errorf("shouldSkipFile(%q) = %v, want %v", name, got, want)
		}
	}
}
