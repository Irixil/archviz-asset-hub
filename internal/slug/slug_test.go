package slug_test

import (
	"testing"

	"damask/server/internal/slug"
)

func TestToSlug(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		input string
		want  string
	}{
		// TODO: Add test cases.
		{name: "simple case", input: "Campaign Photos", want: "campaign-photos"},
		{name: "with special chars", input: "Q2 / Summer 2026", want: "q2-summer-2026"},
		{name: "with accents", input: "Café au lait", want: "cafe-au-lait"},
		{name: "with multiple spaces", input: "  Hello   World  ", want: "hello-world"},
		{name: "with underscores", input: "My_Project_Name", want: "my-project-name"},
		{name: "with multiple hyphens", input: "Hello--World", want: "hello-world"},
		{name: "with leading/trailing hyphens", input: "  -Hello World-  ", want: "hello-world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slug.ToSlug(tt.input)
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.want {
				t.Errorf("ToSlug() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSubaddress(t *testing.T) {
	tests := []struct {
		name      string
		localPart string
		token     string
		tag       string
	}{
		{
			"happy path",
			"ws_abc123+brand-assets",
			"ws_abc123",
			"brand-assets",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got2 := slug.ParseSubaddress(tt.localPart)
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.token {
				t.Errorf("ParseSubaddress() = %v, want %v", got, tt.token)
			}
			if got2 != tt.tag {
				t.Errorf("ParseSubaddress() = %v, want %v", got2, tt.tag)
			}
		})
	}
}
