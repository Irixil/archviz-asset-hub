// Package systemtags defines reserved system tag names.
package systemtags

import "slices"

const (
	GroupName = "system"
	Watermark = "_watermark"
	Template  = "_template"
)

// All returns all known system tag names.
func All() []string {
	return []string{Watermark, Template}
}

// IsSystem reports whether name is a known system tag.
func IsSystem(name string) bool {
	return slices.Contains(All(), name)
}
